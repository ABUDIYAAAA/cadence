package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTGenerateAndValidate(t *testing.T) {
	secret := "test-jwt-secret-key-1234567890"

	t.Run("valid token roundtrip", func(t *testing.T) {
		tok, err := GenerateAccessToken("user-123", "alice@cadence.local", "judge", 3, secret, 15*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error generating token: %v", err)
		}

		claims, err := ValidateAccessToken(tok, secret)
		if err != nil {
			t.Fatalf("unexpected error validating token: %v", err)
		}
		if claims.UserID != "user-123" || claims.Email != "alice@cadence.local" || claims.Role != "judge" || claims.TokenVersion != 3 {
			t.Fatalf("unexpected claims: %+v", claims)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		tok, err := GenerateAccessToken("user-123", "alice@cadence.local", "judge", 1, secret, -1*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error generating expired token: %v", err)
		}
		if _, err := ValidateAccessToken(tok, secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
		}
	})

	t.Run("wrong secret rejected", func(t *testing.T) {
		tok, _ := GenerateAccessToken("user-123", "alice@cadence.local", "judge", 1, secret, 15*time.Minute)
		if _, err := ValidateAccessToken(tok, "wrong-secret"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken for wrong secret, got %v", err)
		}
	})

	t.Run("alg none attack rejected", func(t *testing.T) {
		claims := AccessTokenClaims{
			UserID: "admin-1",
			Role:   "admin",
		}
		unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		tokStr, _ := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)

		if _, err := ValidateAccessToken(tokStr, secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken for alg:none token, got %v", err)
		}
	})
}

func TestCookieManagerSignUnsignAndHTTP(t *testing.T) {
	cm := NewCookieManager("super-secret-cookie-key", ".devclubxnst.com", true, "lax")

	t.Run("sign and unsign valid value", func(t *testing.T) {
		signed := cm.Sign("session.payload.value")
		unsigned, err := cm.Unsign(signed)
		if err != nil {
			t.Fatalf("unexpected unsign error: %v", err)
		}
		if unsigned != "session.payload.value" {
			t.Fatalf("expected 'session.payload.value', got %q", unsigned)
		}
	})

	t.Run("tampered value or signature rejected", func(t *testing.T) {
		signed := cm.Sign("user-id-100")
		tamperedValue := "user-id-999" + signed[len("user-id-100"):]
		if _, err := cm.Unsign(tamperedValue); !errors.Is(err, ErrInvalidCookieSignature) {
			t.Fatalf("expected ErrInvalidCookieSignature for tampered value, got %v", err)
		}

		if _, err := cm.Unsign("no-dot-separator"); !errors.Is(err, ErrInvalidCookieSignature) {
			t.Fatalf("expected ErrInvalidCookieSignature for missing dot, got %v", err)
		}
	})

	t.Run("HTTP SetSignedCookie, GetSignedCookie, and ClearCookie", func(t *testing.T) {
		rec := httptest.NewRecorder()
		cm.SetSignedCookie(rec, "access_token", "jwt-value-xyz", 15*time.Minute)

		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie set, got %d", len(cookies))
		}
		c := cookies[0]
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Domain != "devclubxnst.com" && c.Domain != ".devclubxnst.com" {
			t.Fatalf("unexpected cookie security flags: %+v", c)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(c)

		val, err := cm.GetSignedCookie(req, "access_token")
		if err != nil || val != "jwt-value-xyz" {
			t.Fatalf("expected 'jwt-value-xyz', got %q (err: %v)", val, err)
		}

		// Replay under different cookie name/purpose rejected (e.g. cookie named "refresh_token" containing signed access_token payload)
		mismatchedReq := httptest.NewRequest(http.MethodGet, "/", nil)
		mismatchedReq.AddCookie(&http.Cookie{
			Name:  "refresh_token",
			Value: c.Value,
		})
		if _, err := cm.GetSignedCookie(mismatchedReq, "refresh_token"); !errors.Is(err, ErrInvalidCookiePurpose) {
			t.Fatalf("expected ErrInvalidCookiePurpose for purpose mismatch, got %v", err)
		}

		if _, err := cm.GetSignedCookie(req, "missing_cookie"); !errors.Is(err, ErrCookieNotFound) {
			t.Fatalf("expected ErrCookieNotFound, got %v", err)
		}

		// Expired cookie rejected
		expRec := httptest.NewRecorder()
		cm.SetSignedCookie(expRec, "expired_token", "value", -1*time.Minute)
		expReq := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range expRec.Result().Cookies() {
			expReq.AddCookie(cookie)
		}
		if _, err := cm.GetSignedCookie(expReq, "expired_token"); !errors.Is(err, ErrCookieExpired) {
			t.Fatalf("expected ErrCookieExpired, got %v", err)
		}

		clearRec := httptest.NewRecorder()
		cm.ClearCookie(clearRec, "access_token")
		cleared := clearRec.Result().Cookies()[0]
		if cleared.MaxAge != -1 || cleared.Value != "" {
			t.Fatalf("expected cleared cookie MaxAge=-1 and empty value, got %+v", cleared)
		}
	})
}

func TestGenerateRandomTokenAndHash(t *testing.T) {
	tok1, err := GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tok2, err := GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok1) != 64 || tok1 == tok2 {
		t.Fatalf("expected unique 64-char hex tokens, got %q and %q", tok1, tok2)
	}

	h1 := HashToken(tok1)
	h2 := HashToken(tok1)
	if len(h1) != 64 || h1 != h2 {
		t.Fatalf("expected deterministic 64-char SHA-256 hex digest, got %q", h1)
	}
}

func TestValidateStructAndResponses(t *testing.T) {
	type sampleInput struct {
		Email           string `validate:"required,email"`
		Name            string `validate:"required,min=3,max=10"`
		Password        string `validate:"required,min=8"`
		ConfirmPassword string `validate:"required,eqfield=Password"`
		Role            string `validate:"oneof=admin judge"`
	}

	errs := ValidateStruct(sampleInput{
		Email:           "not-an-email",
		Name:            "Al",
		Password:        "Secret123!",
		ConfirmPassword: "DifferentPassword!",
		Role:            "invalid_role",
	})
	if len(errs) != 4 {
		t.Fatalf("expected 4 validation errors, got %v", errs)
	}
	if !strings.Contains(errs["Email"], "valid email") ||
		!strings.Contains(errs["Name"], "at least 3") ||
		!strings.Contains(errs["ConfirmPassword"], "must match") ||
		!strings.Contains(errs["Role"], "oneof") {
		t.Fatalf("unexpected validation messages: %v", errs)
	}

	if okErrs := ValidateStruct(sampleInput{
		Email:           "valid@cadence.local",
		Name:            "Alice",
		Password:        "Secret123!",
		ConfirmPassword: "Secret123!",
		Role:            "admin",
	}); okErrs != nil {
		t.Fatalf("expected nil validation errors for valid struct, got %v", okErrs)
	}

	rec := httptest.NewRecorder()
	ValidationErrorJSON(rec, errs)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
	}
	var errResp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal ErrorResponse: %v", err)
	}
	if errResp.Error != "validation failed" || len(errResp.Details) != 4 {
		t.Fatalf("unexpected ErrorResponse payload: %+v", errResp)
	}
}
