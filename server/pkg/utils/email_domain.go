package utils

import "strings"

// IsEmailExcluded reports whether email belongs to one of the excluded domains.
// A domain matches the exact mail domain (user@domain) or any subdomain of it.
// Entries may carry a leading "@" and are matched case-insensitively.
func IsEmailExcluded(email string, excludeDomains []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, d := range excludeDomains {
		d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
		if d == "" {
			continue
		}
		if strings.HasSuffix(email, "@"+d) || strings.HasSuffix(email, "."+d) {
			return true
		}
	}
	return false
}
