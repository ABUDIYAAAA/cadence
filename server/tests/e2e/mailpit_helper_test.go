package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

type MailpitAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

type MailpitAttachment struct {
	PartID      string `json:"PartID"`
	FileName    string `json:"FileName"`
	ContentType string `json:"ContentType"`
	Size        int    `json:"Size"`
}

type MailpitMessageSummary struct {
	ID          string           `json:"ID"`
	Subject     string           `json:"Subject"`
	From        MailpitAddress   `json:"From"`
	To          []MailpitAddress `json:"To"`
	Attachments int              `json:"Attachments"`
	Snippet     string           `json:"Snippet"`
}

type MailpitMessagesListResponse struct {
	Total    int                     `json:"total"`
	Count    int                     `json:"count"`
	Messages []MailpitMessageSummary `json:"messages"`
}

type MailpitMessageDetail struct {
	ID          string              `json:"ID"`
	Subject     string              `json:"Subject"`
	From        MailpitAddress      `json:"From"`
	To          []MailpitAddress    `json:"To"`
	HTML        string              `json:"HTML"`
	Text        string              `json:"Text"`
	Attachments []MailpitAttachment `json:"Attachments"`
}

// WaitForEmail polls Mailpit until an email matching recipientEmail and subjectSubstring arrives,
// then fetches and returns the full MailpitMessageDetail.
func WaitForEmail(t *testing.T, recipientEmail, subjectSubstring string) *MailpitMessageDetail {
	t.Helper()

	var matchedID string
	WaitForCondition(
		t,
		12*time.Second,
		150*time.Millisecond,
		fmt.Sprintf("email to %q with subject containing %q", recipientEmail, subjectSubstring),
		func() (bool, error) {
			list, err := fetchMailpitMessages()
			if err != nil {
				return false, err
			}
			for _, msg := range list.Messages {
				if !strings.Contains(strings.ToLower(msg.Subject), strings.ToLower(subjectSubstring)) {
					continue
				}
				for _, to := range msg.To {
					if strings.EqualFold(to.Address, recipientEmail) {
						matchedID = msg.ID
						return true, nil
					}
				}
			}
			return false, nil
		},
		func() string {
			list, err := fetchMailpitMessages()
			if err != nil {
				return fmt.Sprintf("Failed fetching Mailpit messages for diagnostics: %v", err)
			}
			b, _ := json.MarshalIndent(list.Messages, "  ", "  ")
			return fmt.Sprintf("Current Mailpit Inbox (%d messages):\n  %s", list.Count, string(b))
		},
	)

	detail, err := fetchMailpitMessageDetail(matchedID)
	if err != nil {
		t.Fatalf("failed fetching Mailpit message detail for ID %s: %v", matchedID, err)
	}
	return detail
}

func fetchMailpitMessages() (*MailpitMessagesListResponse, error) {
	resp, err := http.Get(mailpitAPIURL + "/api/v1/messages")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mailpit list status %d", resp.StatusCode)
	}

	var out MailpitMessagesListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fetchMailpitMessageDetail(id string) (*MailpitMessageDetail, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/message/%s", mailpitAPIURL, id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mailpit detail status %d", resp.StatusCode)
	}

	var out MailpitMessageDetail
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExtractTokenFromEmailHTML extracts the raw ?token=<hex> parameter from a setup-password or reset-password email HTML body.
func ExtractTokenFromEmailHTML(t *testing.T, htmlBody, routeSlug string) string {
	t.Helper()
	pattern := fmt.Sprintf(`https?://[^\s"<>]+/auth/%s\?token=([a-fA-F0-9]+)`, regexp.QuoteMeta(routeSlug))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(htmlBody)
	if len(matches) < 2 || matches[1] == "" {
		t.Fatalf("failed to extract %s token from email HTML body.\nRegex: %s\nHTML Body:\n%s", routeSlug, pattern, htmlBody)
	}
	return matches[1]
}

// VerifyAndDownloadTicketEmail waits for the entry ticket email in Mailpit, extracts the HMAC-signed
// ticket code from the HTML body, downloads the attached ticket_qr.png, and verifies it is a valid 300x300 PNG image.
func VerifyAndDownloadTicketEmail(t *testing.T, recipientEmail string) (ticketCode string, qrPNGBytes []byte) {
	t.Helper()

	msg := WaitForEmail(t, recipientEmail, "Your Entry Ticket for Cadence Hackathon")

	// 1. Extract ticket code from <div class="ticket-code">cadence:tkt:...</div>
	re := regexp.MustCompile(`cadence:tkt:[a-fA-F0-9]+:[a-fA-F0-9]+`)
	ticketCode = re.FindString(msg.HTML)
	if ticketCode == "" {
		t.Fatalf("failed to extract cadence:tkt:... code from ticket email HTML for %s:\n%s", recipientEmail, msg.HTML)
	}

	// 2. Verify ticket_qr.png attachment exists
	if len(msg.Attachments) == 0 {
		t.Fatalf("expected ticket_qr.png attachment in ticket email for %s, got 0 attachments", recipientEmail)
	}

	var qrAtt *MailpitAttachment
	for i := range msg.Attachments {
		if msg.Attachments[i].FileName == "ticket_qr.png" {
			qrAtt = &msg.Attachments[i]
			break
		}
	}
	if qrAtt == nil {
		t.Fatalf("expected attachment named 'ticket_qr.png', found: %+v", msg.Attachments)
	}
	if !strings.HasPrefix(strings.ToLower(qrAtt.ContentType), "image/png") {
		t.Fatalf("expected ticket_qr.png content-type 'image/png', got %q", qrAtt.ContentType)
	}
	if qrAtt.Size <= 0 {
		t.Fatalf("expected positive attachment size for ticket_qr.png, got %d", qrAtt.Size)
	}

	// 3. Download attachment bytes from Mailpit REST API
	partURL := fmt.Sprintf("%s/api/v1/message/%s/part/%s", mailpitAPIURL, msg.ID, qrAtt.PartID)
	resp, err := http.Get(partURL)
	if err != nil {
		t.Fatalf("failed downloading attachment from %s: %v", partURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK downloading attachment %s, got %d", partURL, resp.StatusCode)
	}

	qrPNGBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading attachment bytes: %v", err)
	}

	// 4. Decode PNG and assert 300x300 QR dimensions
	img, err := png.Decode(bytes.NewReader(qrPNGBytes))
	if err != nil {
		t.Fatalf("downloaded ticket_qr.png is not a valid PNG image: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 300 || bounds.Dy() != 300 {
		t.Fatalf("expected 300x300 QR code PNG dimensions, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	return ticketCode, qrPNGBytes
}
