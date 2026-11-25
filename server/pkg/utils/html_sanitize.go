package utils

import (
	"fmt"
	stdhtml "html"
	"strings"

	"golang.org/x/net/html"
)

// allowedEmailTags lists the HTML tags permitted in frontend-defined custom email bodies.
var allowedEmailTags = map[string]bool{
	"p":          true,
	"br":         true,
	"div":        true,
	"span":       true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"strong":     true,
	"b":          true,
	"em":         true,
	"i":          true,
	"u":          true,
	"s":          true,
	"ul":         true,
	"ol":         true,
	"li":         true,
	"a":          true,
	"img":        true,
	"table":      true,
	"thead":      true,
	"tbody":      true,
	"tfoot":      true,
	"tr":         true,
	"td":         true,
	"th":         true,
	"blockquote": true,
	"hr":         true,
	"code":       true,
	"pre":        true,
}

// voidEmailTags are self-closing HTML tags that must not emit a closing tag.
var voidEmailTags = map[string]bool{
	"br":  true,
	"hr":  true,
	"img": true,
}

// strippedContainerTags are dangerous elements whose inner text/content must also be discarded.
var strippedContainerTags = map[string]bool{
	"script":   true,
	"style":    true,
	"iframe":   true,
	"object":   true,
	"embed":    true,
	"svg":      true,
	"math":     true,
	"head":     true,
	"title":    true,
	"noscript": true,
	"template": true,
	"applet":   true,
}

var allowedTagAttributes = map[string]map[string]bool{
	"*": {
		"style": true,
		"align": true,
		"title": true,
	},
	"a": {
		"href":   true,
		"target": true,
	},
	"img": {
		"src":    true,
		"alt":    true,
		"width":  true,
		"height": true,
	},
	"table": {
		"width":       true,
		"cellpadding": true,
		"cellspacing": true,
		"border":      true,
	},
	"td": {
		"width":   true,
		"height":  true,
		"colspan": true,
		"rowspan": true,
		"valign":  true,
	},
	"th": {
		"width":   true,
		"height":  true,
		"colspan": true,
		"rowspan": true,
		"valign":  true,
	},
}

// SanitizeEmailHTML parses untrusted HTML from the frontend and returns a safe, escaped HTML fragment.
// All <script>, <iframe>, <style>, <svg>, <form>, event handlers (on*), and unsafe URLs (javascript:, data:, vbscript:) are stripped.
func SanitizeEmailHTML(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	var out strings.Builder
	skipDepth := 0

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return strings.TrimSpace(out.String())

		case html.TextToken:
			if skipDepth > 0 {
				continue
			}
			text := string(tokenizer.Text())
			out.WriteString(stdhtml.EscapeString(text))

		case html.StartTagToken, html.SelfClosingTagToken:
			tnBytes, hasAttr := tokenizer.TagName()
			tagName := strings.ToLower(string(tnBytes))

			if strippedContainerTags[tagName] {
				if tt == html.StartTagToken {
					skipDepth++
				}
				continue
			}
			if skipDepth > 0 {
				continue
			}
			if !allowedEmailTags[tagName] {
				continue
			}

			out.WriteByte('<')
			out.WriteString(tagName)

			if hasAttr {
				for {
					keyBytes, valBytes, more := tokenizer.TagAttr()
					attrName := strings.ToLower(string(keyBytes))
					attrVal := strings.TrimSpace(string(valBytes))

					if isAllowedEmailAttr(tagName, attrName, attrVal) {
						if attrName == "style" {
							attrVal = sanitizeInlineStyle(attrVal)
						}
						if attrVal != "" || attrName == "alt" {
							out.WriteByte(' ')
							out.WriteString(attrName)
							out.WriteString(`="`)
							out.WriteString(stdhtml.EscapeString(attrVal))
							out.WriteByte('"')
						}
					}
					if !more {
						break
					}
				}
			}

			if tagName == "a" {
				out.WriteString(` rel="noopener noreferrer"`)
			}
			out.WriteByte('>')

		case html.EndTagToken:
			tnBytes, _ := tokenizer.TagName()
			tagName := strings.ToLower(string(tnBytes))

			if strippedContainerTags[tagName] {
				if skipDepth > 0 {
					skipDepth--
				}
				continue
			}
			if skipDepth > 0 {
				continue
			}
			if !allowedEmailTags[tagName] || voidEmailTags[tagName] {
				continue
			}
			out.WriteString("</")
			out.WriteString(tagName)
			out.WriteByte('>')
		}
	}
}

func isAllowedEmailAttr(tagName, attrName, attrVal string) bool {
	if strings.HasPrefix(attrName, "on") {
		return false
	}
	allowed := allowedTagAttributes["*"][attrName] || (allowedTagAttributes[tagName] != nil && allowedTagAttributes[tagName][attrName])
	if !allowed {
		return false
	}

	if attrName == "href" || attrName == "src" {
		return isSafeEmailURI(tagName, attrVal)
	}
	return true
}

func isSafeEmailURI(tagName, uri string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(uri))
	// Strip control characters and whitespace that could obfuscate "javascript:"
	var sb strings.Builder
	for _, r := range cleaned {
		if r > 32 {
			sb.WriteRune(r)
		}
	}
	compact := sb.String()

	if strings.HasPrefix(compact, "https://") || strings.HasPrefix(compact, "http://") {
		return true
	}
	if tagName == "a" && strings.HasPrefix(compact, "mailto:") {
		return true
	}
	if tagName == "img" && strings.HasPrefix(compact, "cid:") {
		return true
	}
	return false
}

func sanitizeInlineStyle(style string) string {
	lower := strings.ToLower(style)
	if strings.Contains(lower, "expression") ||
		strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "vbscript:") ||
		strings.Contains(lower, "url(") ||
		strings.Contains(lower, "@import") ||
		strings.Contains(lower, "behavior") {
		return ""
	}
	return style
}

// RenderCustomTicketEmailHTML sanitizes customHTML, substitutes HTML-escaped {{name}} and {{event_name}}
// placeholders, and wraps the content in a responsive email layout (the QR code is sent as a file attachment, not inline).
func RenderCustomTicketEmailHTML(customHTML, recipientName, eventName string) string {
	safeBody := SanitizeEmailHTML(customHTML)
	escapedName := stdhtml.EscapeString(recipientName)
	escapedEvent := stdhtml.EscapeString(eventName)

	replacer := strings.NewReplacer(
		"{{name}}", escapedName,
		"{{Name}}", escapedName,
		"{{.Name}}", escapedName,
		"{{event_name}}", escapedEvent,
		"{{EventName}}", escapedEvent,
		"{{.EventName}}", escapedEvent,
	)
	renderedBody := replacer.Replace(safeBody)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>Your entry pass for %s</title>
</head>
<body style="margin:0;padding:0;background-color:#F8FAFC;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F8FAFC;">
<tr><td align="center" style="padding:40px 16px;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:520px;font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
    <tr><td style="background-color:#FFFFFF;border:1px solid #E2E8F0;border-radius:16px;padding:32px 28px;font-size:14px;line-height:22px;color:#0F172A;">
      %s
    </td></tr>
  </table>
</td></tr>
</table>
</body>
</html>`, escapedEvent, renderedBody)
}
