package tickets

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/mailer"
	"devclubxnst.com/cadence/internal/modules/events"
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
	cache      settingsCache
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

// resolveEmailTemplateSettings merges optional request overrides with saved event ticket email settings.
func (s *Service) resolveEmailTemplateSettings(ctx context.Context, reqSubject, reqHTML *string) (string, string, error) {
	settings, err := s.cachedSettings(ctx)
	if err != nil {
		return "", "", err
	}

	subject := settings.EmailSubject
	if reqSubject != nil && strings.TrimSpace(*reqSubject) != "" {
		subject = strings.TrimSpace(*reqSubject)
		if err := mailer.ValidateHeaderField(subject); err != nil {
			return "", "", ErrInvalidEmailSubject
		}
	}

	customHTML := settings.EmailHTML
	if reqHTML != nil && strings.TrimSpace(*reqHTML) != "" {
		customHTML = utils.SanitizeEmailHTML(*reqHTML)
	}

	return subject, customHTML, nil
}

// SendSingleTicket generates or fetches a user's ticket, releases it on their dashboard, and enqueues their entry pass email with QR attachment.
func (s *Service) SendSingleTicket(
	ctx context.Context,
	targetUserID string,
	req SendSingleTicketRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (*SingleSendTicketResponse, error) {
	name, email, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	subject, customHTML, err := s.resolveEmailTemplateSettings(ctx, req.EmailSubject, req.EmailHTML)
	if err != nil {
		return nil, err
	}

	ticketCode, err := s.generateTicketCode()
	if err != nil {
		return nil, fmt.Errorf("generate ticket code: %w", err)
	}

	ticket, err := s.repo.GetOrCreateReleasedTicket(ctx, targetUserID, ticketCode)
	if err != nil {
		return nil, err
	}

	qrPNG, err := qrcode.Encode(ticket.TicketCode, qrcode.Medium, 300)
	if err != nil {
		return nil, fmt.Errorf("encode qr png: %w", err)
	}

	eventName := "Cadence Hackathon"
	if evCtx, ok := events.FromContext(ctx); ok && evCtx.Name != "" {
		eventName = evCtx.Name
	}

	_, _ = s.repo.ClaimTicketForEmail(ctx, ticket.ID, true)
	emailStatus := EmailStatusQueued

	renderedHTML := ""
	if customHTML != "" {
		renderedHTML = utils.RenderCustomTicketEmailHTML(customHTML, name, eventName)
	}

	if s.mailQueue != nil {
		if err := s.mailQueue.SendCustomTicketEmail(ctx, ticket.ID, email, name, eventName, subject, renderedHTML, qrPNG); err != nil {
			_ = s.repo.MarkTicketEmailFailed(ctx, ticket.ID, err.Error())
			return nil, fmt.Errorf("enqueue ticket email: %w", err)
		}
	} else {
		_ = s.repo.MarkTicketEmailDelivered(ctx, ticket.ID)
		emailStatus = EmailStatusSent
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
			"recipient_id":   targetUserID,
			"recipient_mail": email,
			"custom_html":    renderedHTML != "",
		},
		&ip, &userAgent,
	)

	return &SingleSendTicketResponse{
		Message:     "Ticket generated and enqueued for delivery",
		UserID:      targetUserID,
		TicketCode:  ticket.TicketCode,
		EmailStatus: emailStatus,
	}, nil
}

// BulkSendTickets releases event tickets globally, mints passes for all matching event members,
// skips email dispatch for excluded @domains and already-mailed users (unless ForceResend is true),
// and enqueues sanitized HTML + QR attachment emails.
func (s *Service) BulkSendTickets(
	ctx context.Context,
	req BulkSendTicketsRequest,
	actorID, actorEmail, actorRole string,
	ip, userAgent string,
) (*BulkSendTicketsResponse, error) {
	subject, customHTML, err := s.resolveEmailTemplateSettings(ctx, req.EmailSubject, req.EmailHTML)
	if err != nil {
		return nil, err
	}

	savedDomains, err := s.InternalDomains(ctx)
	if err != nil {
		return nil, err
	}
	reqDomains, err := normalizeDomains(req.ExcludeDomains)
	if err != nil {
		return nil, err
	}
	effectiveExcludeDomains := make([]string, 0, len(savedDomains)+len(reqDomains))
	effectiveExcludeDomains = append(effectiveExcludeDomains, savedDomains...)
	effectiveExcludeDomains = append(effectiveExcludeDomains, reqDomains...)

	// Mark tickets released for the event so late-joining participants also get their dashboard ticket automatically.
	if err := s.setTicketsReleased(ctx, true, actorID); err != nil {
		return nil, err
	}

	eligible, err := s.repo.FindEligibleUsers(ctx, req)
	if err != nil {
		return nil, err
	}

	eventName := "Cadence Hackathon"
	if evCtx, ok := events.FromContext(ctx); ok && evCtx.Name != "" {
		eventName = evCtx.Name
	}

	resp := &BulkSendTicketsResponse{
		TotalProcessed: len(eligible),
	}

	for _, u := range eligible {
		ticketCode := u.TicketCode
		ticketID := u.TicketID

		if ticketCode == "" {
			generatedCode, err := s.generateTicketCode()
			if err != nil {
				resp.Skipped++
				continue
			}
			tkt, err := s.repo.GetOrCreateReleasedTicket(ctx, u.UserID, generatedCode)
			if err != nil {
				resp.Skipped++
				continue
			}
			ticketID = tkt.ID
			ticketCode = tkt.TicketCode
		} else if u.ReleasedAt == nil {
			_ = s.repo.ReleaseTicket(ctx, ticketID)
		}

		// 1. Check if user's email belongs to an excluded @domain
		if utils.IsEmailExcluded(u.Email, effectiveExcludeDomains) {
			_ = s.repo.MarkTicketDomainExcluded(ctx, ticketID)
			resp.ExcludedDomain++
			continue
		}

		// 2. Check if user has already been mailed or is currently queued (unless ForceResend is set)
		if !req.ForceResend && (u.EmailStatus == EmailStatusSent || u.EmailStatus == EmailStatusQueued || u.EmailSentAt != nil) {
			resp.AlreadyMailed++
			continue
		}

		// 3. Atomically claim ticket in PostgreSQL to prevent concurrent duplicate sends
		claimed, err := s.repo.ClaimTicketForEmail(ctx, ticketID, req.ForceResend)
		if err != nil {
			resp.Skipped++
			continue
		}
		if !claimed {
			resp.AlreadyMailed++
			continue
		}

		// 4. Generate QR code attachment and render custom HTML (if configured)
		qrPNG, err := qrcode.Encode(ticketCode, qrcode.Medium, 300)
		if err != nil {
			_ = s.repo.MarkTicketEmailFailed(ctx, ticketID, err.Error())
			resp.Skipped++
			continue
		}

		renderedHTML := ""
		if customHTML != "" {
			renderedHTML = utils.RenderCustomTicketEmailHTML(customHTML, u.Name, eventName)
		}

		if s.mailQueue != nil {
			if err := s.mailQueue.SendCustomTicketEmail(ctx, ticketID, u.Email, u.Name, eventName, subject, renderedHTML, qrPNG); err != nil {
				_ = s.repo.MarkTicketEmailFailed(ctx, ticketID, err.Error())
				resp.Skipped++
				continue
			}
		} else {
			_ = s.repo.MarkTicketEmailDelivered(ctx, ticketID)
		}

		resp.Enqueued++
	}

	s.recordAudit(
		ctx,
		audit.ActionTicketIssued,
		audit.TargetTicket,
		nil,
		fmt.Sprintf("Bulk ticket dispatch executed by %s: %d enqueued, %d already mailed, %d excluded by domain, %d skipped",
			actorEmail, resp.Enqueued, resp.AlreadyMailed, resp.ExcludedDomain, resp.Skipped),
		&actorID,
		&actorEmail,
		&actorRole,
		map[string]any{
			"total_processed":  resp.TotalProcessed,
			"enqueued":         resp.Enqueued,
			"already_mailed":   resp.AlreadyMailed,
			"excluded_domain":  resp.ExcludedDomain,
			"skipped":          resp.Skipped,
			"excluded_domains": effectiveExcludeDomains,
			"force_resend":     req.ForceResend,
			"custom_html":      customHTML != "",
		},
		&ip, &userAgent,
	)

	return resp, nil
}

// GetMyTicket returns the authenticated user's issued ticket with a base64 QR data URI.
// If tickets have been released for the active event and a participant created or joined the event later,
// their ticket is automatically minted and released on-the-fly.
func (s *Service) GetMyTicket(ctx context.Context, userID string) (*MyTicketResponse, error) {
	ticket, err := s.repo.GetTicketByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ErrTicketNotFound) {
		return nil, err
	}

	if ticket == nil {
		released, relErr := s.AreTicketsReleased(ctx)
		if relErr != nil {
			return nil, relErr
		}
		if !released {
			return nil, ErrTicketNotFound
		}

		isMember, memErr := s.repo.IsEventMember(ctx, userID)
		if memErr != nil {
			return nil, memErr
		}
		if !isMember {
			return nil, ErrTicketNotFound
		}

		code, codeErr := s.generateTicketCode()
		if codeErr != nil {
			return nil, fmt.Errorf("generate ticket code: %w", codeErr)
		}
		ticket, err = s.repo.GetOrCreateReleasedTicket(ctx, userID, code)
		if err != nil {
			return nil, err
		}
	} else if ticket.ReleasedAt == nil && ticket.EmailSentAt == nil && !ticket.IsCheckedIn {
		released, relErr := s.AreTicketsReleased(ctx)
		if relErr != nil {
			return nil, relErr
		}
		if !released {
			return nil, ErrTicketNotFound
		}
		_ = s.repo.ReleaseTicket(ctx, ticket.ID)
		now := time.Now()
		ticket.ReleasedAt = &now
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
		EmailStatus:  ticket.EmailStatus,
		EmailSentAt:  ticket.EmailSentAt,
		ReleasedAt:   ticket.ReleasedAt,
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
	eventID := resolveEventID(ctx)
	s.auditQueue.Record(ctx, audit.Event{
		EventID:     &eventID,
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
