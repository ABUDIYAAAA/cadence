package utils

import "strings"

// ExtractEmailDomain normalizes a domain or email address entry by extracting everything after the last "@"
// (or trimming a leading "@") and lower-casing it.
// For example: "@nst.rishihood.edu.in", "user@nst.rishihood.edu.in", and "nst.rishihood.edu.in"
// all normalize to "nst.rishihood.edu.in".
func ExtractEmailDomain(input string) string {
	d := strings.ToLower(strings.TrimSpace(input))
	if idx := strings.LastIndex(d, "@"); idx != -1 {
		d = strings.TrimSpace(d[idx+1:])
	}
	return d
}

// IsEmailExcluded reports whether email belongs to one of the excluded domains.
// A domain matches the exact mail domain (user@domain) or any subdomain of it.
// Entries may carry a leading "@" (or full user@domain) and are matched case-insensitively.
func IsEmailExcluded(email string, excludeDomains []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, raw := range excludeDomains {
		d := ExtractEmailDomain(raw)
		if d == "" {
			continue
		}
		if strings.HasSuffix(email, "@"+d) || strings.HasSuffix(email, "."+d) {
			return true
		}
	}
	return false
}
