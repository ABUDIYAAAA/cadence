package mailer

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"github.com/wneessen/go-mail"
)

// Attachment defines a file attachment for an email message.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
	// Inline embeds the file in the message body (referenced as cid:<Filename>) instead of listing it as an attachment.
	Inline bool `json:"inline,omitempty"`
}

type Mailer struct {
	client         *mail.Client
	templateEngine *TemplateEngine
	fromAddress    string
	fromName       string
}

func NewMailer(cfg *config.Config, engine *TemplateEngine) (*Mailer, error) {
	opts := []mail.Option{
		mail.WithPort(cfg.SMTPPort),
		mail.WithTimeout(10 * time.Second),
	}

	if cfg.SMTPSSL {
		opts = append(opts, mail.WithSSL())
	} else if cfg.SMTPPort == 1025 || cfg.SMTPHost == "localhost" {
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	} else {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSOpportunistic))
	}

	if cfg.SMTPUsername != "" && cfg.SMTPPassword != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.SMTPUsername),
			mail.WithPassword(cfg.SMTPPassword),
		)
	}

	client, err := mail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("initialize smtp client: %w", err)
	}

	return &Mailer{
		client:         client,
		templateEngine: engine,
		fromAddress:    cfg.SMTPFromEmail,
		fromName:       cfg.SMTPFromName,
	}, nil
}

// SendWithAttachments renders the template with injection protection and dispatches the email with attachments via SMTP.
func (m *Mailer) SendWithAttachments(ctx context.Context, to, subject, templateName string, data any, attachments []Attachment) error {
	if err := ValidateHeaderField(to); err != nil {
		return fmt.Errorf("invalid recipient header: %w", err)
	}
	if err := ValidateHeaderField(subject); err != nil {
		return fmt.Errorf("invalid subject header: %w", err)
	}

	htmlBody, err := m.templateEngine.Render(templateName, data)
	if err != nil {
		return fmt.Errorf("render email template: %w", err)
	}

	msg := mail.NewMsg()
	if m.fromName != "" {
		if err := msg.FromFormat(m.fromName, m.fromAddress); err != nil {
			return fmt.Errorf("set from header: %w", err)
		}
	} else {
		if err := msg.From(m.fromAddress); err != nil {
			return fmt.Errorf("set from header: %w", err)
		}
	}

	if err := msg.To(to); err != nil {
		return fmt.Errorf("set to header: %w", err)
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, htmlBody)

	for _, att := range attachments {
		var opts []mail.FileOption
		if att.ContentType != "" {
			opts = append(opts, mail.WithFileContentType(mail.ContentType(att.ContentType)))
		}
		if att.Inline {
			if err := msg.EmbedReader(att.Filename, bytes.NewReader(att.Data), opts...); err != nil {
				return fmt.Errorf("embed file %s: %w", att.Filename, err)
			}
			continue
		}
		if err := msg.AttachReader(att.Filename, bytes.NewReader(att.Data), opts...); err != nil {
			return fmt.Errorf("attach file %s: %w", att.Filename, err)
		}
	}

	if err := m.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("dispatch email via smtp: %w", err)
	}

	return nil
}

// Send renders the template with injection protection and dispatches the email via SMTP without attachments.
func (m *Mailer) Send(ctx context.Context, to, subject, templateName string, data any) error {
	return m.SendWithAttachments(ctx, to, subject, templateName, data, nil)
}

// Close gracefully closes the mail client connection pool.
func (m *Mailer) Close() error {
	return m.client.Close()
}
