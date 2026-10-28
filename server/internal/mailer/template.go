package mailer

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/*.html
var templatesFS embed.FS

var (
	ErrHeaderInjection = errors.New("crlf characters detected: potential header injection")
	ErrTemplateNotFound = errors.New("email template not found")
)

// TemplateEngine manages and renders HTML email templates with context-aware escaping.
type TemplateEngine struct {
	templates *template.Template
}

// NewTemplateEngine parses and prepares embedded templates.
func NewTemplateEngine() (*TemplateEngine, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse embedded templates: %w", err)
	}
	return &TemplateEngine{templates: tmpl}, nil
}

// Render executes the given template with contextual HTML auto-escaping.
func (te *TemplateEngine) Render(templateName string, data any) (string, error) {
	var buf bytes.Buffer
	nameWithExt := templateName
	if !strings.HasSuffix(nameWithExt, ".html") {
		nameWithExt += ".html"
	}

	tmpl := te.templates.Lookup(nameWithExt)
	if tmpl == nil {
		return "", fmt.Errorf("%w: %s", ErrTemplateNotFound, templateName)
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// ValidateHeaderField verifies that a header value does not contain CRLF characters (Header Injection protection).
func ValidateHeaderField(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return ErrHeaderInjection
	}
	return nil
}
