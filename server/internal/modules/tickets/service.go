package tickets

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/mailer"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/skip2/go-qrcode"
)

var (
	ErrAlreadyCheckedIn = errors.New("ticket has already been checked in")
)

type Service struct {
	repo       *Repository
	cfg        *config.Config
	mailQueue  *mailer.Queue
	auditQueue audit.Recorder
}

func NewService(repo *Repository, cfg *config.Config, mailQueue *mailer.Queue, auditQueue audit.Recorder) *Service {
	return &Service{
		repo:       repo,
		cfg:        cfg,
		mailQueue:  mailQueue,
		auditQueue: auditQueue,
	}
}

// generateTicketCode creates a cryptographically signed ticket identifier.
func (s *Service) generateTicketCode() (string, error) {
	token, err := utils.GenerateRandomToken(16)
	if err != nil {
		return "", err
	}
	h := hmac.New(sha256.New, []byte(s.cfg.CookieSecret))
	h.Write([]byte(token))
	sig := hex.EncodeToString(h.Sum(nil))[:8]
	return fmt.Sprintf("cadence:tkt:%s:%s", token, sig), nil
}

// CheckIn processes an attendee ticket scan with instant duplicate detection.
func (s *Service) CheckIn(
	ctx context.Context,
	volunteerID, volunteerEmail, volunteerRole string,
	ticketCode string,
	ip, userAgent string,
) (*CheckInResponse, *DuplicateCheckInResponse, error) {
	info, err := s.repo.GetTicketByCode(ctx, ticketCode)
	if err != nil {
		metrics.InstrumentTicketScan("invalid")
		return nil, nil, err
	}

	attendee := AttendeeDetails{
		ID:          info.UserID,
		Name:        info.UserName,
		Email:       info.UserEmail,
		Role:        info.UserRole,
		PhoneNumber: info.UserPhone,
		TeamID:      info.TeamID,
		TeamName:    info.TeamName,
	}

	// Instant duplicate check-in detection
	if info.IsCheckedIn {
		metrics.InstrumentTicketScan("duplicate")
		checkedAtStr := ""
		if info.CheckedInAt != nil {
			checkedAtStr = info.CheckedInAt.Format(time.RFC3339)
		}

		s.recordAudit(
			ctx,
			audit.ActionTicketDuplicateScan,
			audit.TargetTicket,
			&info.TicketID,
			fmt.Sprintf("Duplicate ticket check-in attempted by %s for attendee %s", volunteerEmail, info.UserEmail),
			&volunteerID,
			&volunteerEmail,
			&volunteerRole,
			map[string]any{
				"ticket_code":         ticketCode,
				"attendee_id":         info.UserID,
				"attendee_email":      info.UserEmail,
				"prior_checked_in_at": checkedAtStr,
				"prior_volunteer_id":  info.CheckedInByID,
			},
			&ip, &userAgent,
		)

		return nil, &DuplicateCheckInResponse{
			Error:           "Ticket has already been checked in",
			CheckedInAt:     checkedAtStr,
			CheckedInByID:   info.CheckedInByID,
			CheckedInByName: info.CheckedInByName,
			Attendee:        attendee,
		}, ErrAlreadyCheckedIn
	}

	// Atomic update to mark present
	checkedInAt, marked, err := s.repo.MarkCheckedIn(ctx, info.TicketID, volunteerID)
	if err != nil {
		return nil, nil, err
	}

	if !marked {
		// Race condition: another scanner processed this ticket in the same instant
		updated, _ := s.repo.GetTicketByCode(ctx, ticketCode)
		priorCheckedAt := ""
		if updated != nil && updated.CheckedInAt != nil {
			priorCheckedAt = updated.CheckedInAt.Format(time.RFC3339)
		}
		var priorVolID, priorVolName *string
		if updated != nil {
			priorVolID = updated.CheckedInByID
			priorVolName = updated.CheckedInByName
		}

		return nil, &DuplicateCheckInResponse{
			Error:           "Ticket was just checked in by another scanner",
			CheckedInAt:     priorCheckedAt,
			CheckedInByID:   priorVolID,
			CheckedInByName: priorVolName,
			Attendee:        attendee,
		}, ErrAlreadyCheckedIn
	}

	checkedAtStr := checkedInAt.Format(time.RFC3339)
	metrics.InstrumentTicketScan("valid")

	s.recordAudit(
		ctx,
		audit.ActionTicketCheckIn,
		audit.TargetTicket,
		&info.TicketID,
		fmt.Sprintf("Attendee %s successfully checked in by %s", info.UserEmail, volunteerEmail),
		&volunteerID,
		&volunteerEmail,
		&volunteerRole,
		map[string]any{
			"ticket_code":    ticketCode,
			"attendee_id":    info.UserID,
			"attendee_email": info.UserEmail,
			"checked_in_at":  checkedAtStr,
		},
		&ip, &userAgent,
	)

	return &CheckInResponse{
		Success:     true,
		Message:     "Check-in verified successfully",
		TicketCode:  ticketCode,
		CheckedInAt: checkedAtStr,
		Attendee:    attendee,
	}, nil, nil
}

// SendSingleTicket generates or fetches a user's ticket and enqueues their entry pass email with QR attachment.
func (s *Service) SendSingleTicket(
	ctx context.Context,
	targetUserID string,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (*SingleSendTicketResponse, error) {
	name, email, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	ticket, err := s.repo.GetTicketByUserID(ctx, targetUserID)
	if err != nil && !errors.Is(err, ErrTicketNotFound) {
		return nil, err
	}

	if ticket == nil {
		ticketCode, err := s.generateTicketCode()
		if err != nil {
			return nil, fmt.Errorf("generate ticket code: %w", err)
		}

		ticket, err = s.repo.GetOrCreateTicket(ctx, targetUserID, ticketCode)
		if err != nil {
			return nil, err
		}
	}

	qrPNG, err := qrcode.Encode(ticket.TicketCode, qrcode.Medium, 300)
	if err != nil {
		return nil, fmt.Errorf("encode qr png: %w", err)
	}

	if s.mailQueue != nil {
		eventName := "Cadence Hackathon"
		if err := s.mailQueue.SendTicketEmail(ctx, email, name, eventName, qrPNG); err != nil {
			return nil, fmt.Errorf("enqueue ticket email: %w", err)
		}
		_ = s.repo.MarkEmailSent(ctx, ticket.ID)
	}

	s.recordAudit(
		ctx,
		audit.ActionTicketIssued,
		audit.TargetTicket,
		&ticket.ID,
		fmt.Sprintf("Entry ticket issued and emailed to %s", email),
		&actorID,
		&actorEmail,
		&actorRole,
		map[string]any{
			"ticket_code":    ticket.TicketCode,
			"recipient_id":   targetUserID,
			"recipient_mail": email,
		},
		&ip, &userAgent,
	)

	return &SingleSendTicketResponse{
		Message:    "Ticket generated and enqueued for delivery",
		UserID:     targetUserID,
		TicketCode: ticket.TicketCode,
	}, nil
}

// BulkSendTickets dispatches tickets asynchronously to users matching filters.
func (s *Service) BulkSendTickets(
	ctx context.Context,
	req BulkSendTicketsRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (*BulkSendTicketsResponse, error) {
	eligible, err := s.repo.FindEligibleUsers(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(req.ExcludeDomains) > 0 {
		kept := eligible[:0]
		for _, u := range eligible {
			if !utils.IsEmailExcluded(u.Email, req.ExcludeDomains) {
				kept = append(kept, u)
			}
		}
		eligible = kept
	}

	total := len(eligible)
	enqueued := 0
	skipped := 0
	eventName := "Cadence Hackathon"

	for _, u := range eligible {
		ticketCode := u.TicketCode
		ticketID := u.TicketID

		if ticketCode == "" {
			var err error
			ticketCode, err = s.generateTicketCode()
			if err != nil {
				skipped++
				continue
			}
			tkt, err := s.repo.GetOrCreateTicket(ctx, u.UserID, ticketCode)
			if err != nil {
				skipped++
				continue
			}
			ticketID = tkt.ID
			ticketCode = tkt.TicketCode
		}

		qrPNG, err := qrcode.Encode(ticketCode, qrcode.Medium, 300)
		if err != nil {
			skipped++
			continue
		}

		if s.mailQueue != nil {
			if err := s.mailQueue.SendTicketEmail(ctx, u.Email, u.Name, eventName, qrPNG); err != nil {
				skipped++
				continue
			}
			_ = s.repo.MarkEmailSent(ctx, ticketID)
		}

		enqueued++
	}

	s.recordAudit(
		ctx,
		audit.ActionTicketIssued,
		audit.TargetTicket,
		nil,
		fmt.Sprintf("Bulk ticket dispatch executed by %s: %d enqueued, %d skipped", actorEmail, enqueued, skipped),
		&actorID,
		&actorEmail,
		&actorRole,
		map[string]any{
			"total_processed": total,
			"enqueued":        enqueued,
			"skipped":         skipped,
		},
		&ip, &userAgent,
	)

	return &BulkSendTicketsResponse{
		TotalProcessed: total,
		Enqueued:       enqueued,
		Skipped:        skipped,
	}, nil
}

// GetMyTicket returns the authenticated user's issued ticket with a base64 QR data URI.
// Tickets are only created when an admin dispatches them, so a user with none gets ErrTicketNotFound
// rather than a freshly minted, scannable pass.
func (s *Service) GetMyTicket(ctx context.Context, userID string) (*MyTicketResponse, error) {
	ticket, err := s.repo.GetTicketByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	qrPNG, err := qrcode.Encode(ticket.TicketCode, qrcode.Medium, 300)
	if err != nil {
		return nil, fmt.Errorf("generate qr png: %w", err)
	}

	qrBase64 := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(qrPNG))

	return &MyTicketResponse{
		TicketCode:   ticket.TicketCode,
		IsCheckedIn:  ticket.IsCheckedIn,
		CheckedInAt:  ticket.CheckedInAt,
		QRCodeBase64: qrBase64,
	}, nil
}

// GetAnalytics computes full event attendance statistics.
func (s *Service) GetAnalytics(ctx context.Context) (*AnalyticsResponse, error) {
	return s.repo.GetAttendanceAnalytics(ctx)
}

func (s *Service) recordAudit(
	ctx context.Context,
	action, targetType string,
	targetID *string,
	desc string,
	actorID, actorEmail, actorRole *string,
	metadata map[string]any,
	ip, ua *string,
) {
	if s.auditQueue == nil {
		return
	}
	s.auditQueue.Record(ctx, audit.Event{
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		ActorRole:   actorRole,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Description: desc,
		Metadata:    metadata,
		IPAddress:   ip,
		UserAgent:   ua,
		Timestamp:   time.Now(),
	})
}
