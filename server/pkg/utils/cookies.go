package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrInvalidCookieSignature = errors.New("invalid or tampered cookie signature")
	ErrCookieNotFound         = errors.New("cookie not found")
)

// CookieManager manages signed HTTP cookies.
type CookieManager struct {
	secret   []byte
	domain   string
	secure   bool
	sameSite http.SameSite
}

// NewCookieManager creates a new CookieManager instance.
func NewCookieManager(secret, domain string, secure bool, sameSiteStr string) *CookieManager {
	var sameSite http.SameSite
	switch strings.ToLower(sameSiteStr) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
	default:
		sameSite = http.SameSiteLaxMode
	}

	return &CookieManager{
		secret:   []byte(secret),
		domain:   domain,
		secure:   secure,
		sameSite: sameSite,
	}
}

// Sign signs a raw string value using HMAC-SHA256.
func (cm *CookieManager) Sign(value string) string {
	mac := hmac.New(sha256.New, cm.secret)
	mac.Write([]byte(value))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", value, signature)
}

// Unsign validates and extracts the raw value from a signed string.
func (cm *CookieManager) Unsign(signedValue string) (string, error) {
	lastDot := strings.LastIndex(signedValue, ".")
	if lastDot == -1 {
		return "", ErrInvalidCookieSignature
	}

	value := signedValue[:lastDot]
	signature := signedValue[lastDot+1:]

	mac := hmac.New(sha256.New, cm.secret)
	mac.Write([]byte(value))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return "", ErrInvalidCookieSignature
	}

	return value, nil
}

// SetSignedCookie writes an HTTP-only signed cookie to the response.
func (cm *CookieManager) SetSignedCookie(w http.ResponseWriter, name, value string, duration time.Duration) {
	signed := cm.Sign(value)
	cookie := &http.Cookie{
		Name:     name,
		Value:    signed,
		Path:     "/",
		Domain:   cm.domain,
		Expires:  time.Now().Add(duration),
		MaxAge:   int(duration.Seconds()),
		HttpOnly: true,
		Secure:   cm.secure,
		SameSite: cm.sameSite,
	}
	http.SetCookie(w, cookie)
}

// GetSignedCookie reads and validates a signed cookie from the request.
func (cm *CookieManager) GetSignedCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", ErrCookieNotFound
		}
		return "", err
	}

	return cm.Unsign(cookie.Value)
}

// ClearCookie invalidates an HTTP cookie by setting MaxAge to -1.
func (cm *CookieManager) ClearCookie(w http.ResponseWriter, name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   cm.domain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cm.secure,
		SameSite: cm.sameSite,
	}
	http.SetCookie(w, cookie)
}
