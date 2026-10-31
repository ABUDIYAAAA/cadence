package mailer

import (
	"errors"
	"strings"
	"testing"
)

func TestTemplateEngineRenderAllTemplates(t *testing.T) {
	engine, err := NewTemplateEngine()
	if err != nil {
		t.Fatalf("failed to initialize template engine: %v", err)
	}

	t.Run("reset_password template renders with .html extension", func(t *testing.T) {
		html, err := engine.Render("reset_password.html", map[string]any{
			"Name":        "Alan Turing",
			"ResetURL":    "https://cadence.devclubxnst.com/auth/reset-password?token=xyz987",
			"ExpiryHours": 1,
		})
		if err != nil {
			t.Fatalf("unexpected render error: %v", err)
		}
		if !strings.Contains(html, "Alan Turing") || !strings.Contains(html, "token=xyz987") {
			t.Fatalf("rendered reset_password HTML missing expected fields:\n%s", html)
		}
	})

	t.Run("ticket template shows the QR and never the ticket code", func(t *testing.T) {
		html, err := engine.Render("ticket", map[string]any{
			"Name":       "Grace Hopper",
			"EventName":  "Cadence Hackathon",
			"TicketCode": "cadence:tkt:deadbeef:12345678",
		})
		if err != nil {
			t.Fatalf("unexpected render error: %v", err)
		}
		if !strings.Contains(html, "Grace Hopper") || !strings.Contains(html, `src="cid:ticket_qr.png"`) {
			t.Fatalf("rendered ticket HTML missing expected fields:\n%s", html)
		}
		if strings.Contains(html, "cadence:tkt:") {
			t.Fatalf("ticket email must not contain the ticket code string:\n%s", html)
		}
	})

	t.Run("contextual HTML auto-escaping blocks XSS payloads", func(t *testing.T) {
		html, err := engine.Render("reset_password", map[string]any{
			"Name":        `<script>alert("xss")</script>`,
			"ResetURL":    "https://cadence.devclubxnst.com/auth/reset-password?token=123",
			"ExpiryHours": 24,
		})
		if err != nil {
			t.Fatalf("unexpected render error: %v", err)
		}
		if strings.Contains(html, `<script>alert("xss")</script>`) {
			t.Fatalf("expected XSS payload to be HTML-escaped, got raw script tag:\n%s", html)
		}
		if !strings.Contains(html, `&lt;script&gt;`) {
			t.Fatalf("expected escaped script entity in output:\n%s", html)
		}
	})

	t.Run("unknown template returns ErrTemplateNotFound", func(t *testing.T) {
		if _, err := engine.Render("non_existent_template", nil); !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("expected ErrTemplateNotFound, got %v", err)
		}
	})
}

func TestValidateHeaderFieldCRLFInjection(t *testing.T) {
	if err := ValidateHeaderField("valid@cadence.devclubxnst.com"); err != nil {
		t.Fatalf("expected valid header field to pass, got %v", err)
	}

	payloads := []string{
		"victim@example.com\r\nBcc: attacker@evil.com",
		"Subject Line\nCc: attacker@evil.com",
		"Header\rInjection",
	}
	for _, p := range payloads {
		if err := ValidateHeaderField(p); !errors.Is(err, ErrHeaderInjection) {
			t.Fatalf("expected ErrHeaderInjection for payload %q, got %v", p, err)
		}
	}
}
