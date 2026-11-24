package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/ratelimit"
	"devclubxnst.com/cadence/pkg/logger"
	"devclubxnst.com/cadence/pkg/utils"
)

type Handler struct {
	service       *Service
	cookieManager *utils.CookieManager
	cfg           *config.Config
	limiter       *ratelimit.Limiter
}

func NewHandler(
	service *Service,
	cookieManager *utils.CookieManager,
	cfg *config.Config,
	loginGuard *ratelimit.LoginGuard,
	limiter *ratelimit.Limiter,
) *Handler {
	return &Handler{
		service:       service,
		cookieManager: cookieManager,
		cfg:           cfg,
		limiter:       limiter,
	}
}

type oauthStatePayload struct {
	Nonce    string `json:"nonce"`
	Redirect string `json:"redirect"`
}

// HandleGoogleLogin initiates Google OAuth flow by generating a secure state and redirecting.
func (h *Handler) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	returnPath := r.URL.Query().Get("redirect")
	if returnPath == "" || !strings.HasPrefix(returnPath, "/") {
		returnPath = "/"
	}

	rawNonce, err := utils.GenerateRandomToken(24)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to generate authentication state")
		return
	}

	statePayload := oauthStatePayload{
		Nonce:    rawNonce,
		Redirect: returnPath,
	}
	stateJSON, _ := json.Marshal(statePayload)
	encodedState := base64.RawURLEncoding.EncodeToString(stateJSON)

	// Store signed state cookie (10 minute expiry)
	h.cookieManager.SetSignedCookie(w, "oauth_state", encodedState, 10*time.Minute)

	authURL := h.service.GetGoogleAuthURL(encodedState)

	if r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		utils.JSON(w, http.StatusOK, GoogleAuthURLResponse{URL: authURL})
		return
	}

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleGoogleCallback processes the OAuth redirect from Google, authenticates the user, and sets cookies.
func (h *Handler) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Check for Google error response (e.g. user denied consent)
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		logger.FromContext(r.Context()).Warn("google oauth denied or returned error", slog.String("error", errParam))
		h.clearOAuthCookie(w)
		utils.ErrorJSON(w, http.StatusBadRequest, fmt.Sprintf("google authentication failed: %s", errParam))
		return
	}

	stateParam := r.URL.Query().Get("state")
	codeParam := r.URL.Query().Get("code")
	if stateParam == "" || codeParam == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "missing state or code parameter")
		return
	}

	// Validate signed state cookie against parameter
	cookieState, err := h.cookieManager.GetSignedCookie(r, "oauth_state")
	if err != nil || cookieState != stateParam {
		h.clearOAuthCookie(w)
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid or expired oauth state parameter; please try logging in again")
		return
	}
	h.clearOAuthCookie(w)

	var statePayload oauthStatePayload
	if stateBytes, err := base64.RawURLEncoding.DecodeString(stateParam); err == nil {
		_ = json.Unmarshal(stateBytes, &statePayload)
	}
	redirectPath := statePayload.Redirect
	if redirectPath == "" || !strings.HasPrefix(redirectPath, "/") {
		redirectPath = "/"
	}

	deviceID := ratelimit.GetDeviceID(r.Context())
	userAgent := r.UserAgent()
	ip := getClientIP(r)

	user, accessToken, refreshToken, err := h.service.HandleGoogleCallback(r.Context(), codeParam, deviceID, userAgent, ip)
	if err != nil {
		switch {
		case errors.Is(err, ErrInactiveUser):
			utils.ErrorJSON(w, http.StatusForbidden, "user account is deactivated")
		case errors.Is(err, ErrOAuthExchangeFailed):
			utils.ErrorJSON(w, http.StatusBadGateway, "failed to exchange authorization token with Google")
		case errors.Is(err, ErrOAuthProfileFailed):
			utils.ErrorJSON(w, http.StatusBadGateway, "failed to retrieve profile from Google")
		default:
			logger.FromContext(r.Context()).Error("google callback internal error", slog.Any("error", err))
			utils.ErrorJSON(w, http.StatusInternalServerError, "authentication failed")
		}
		return
	}

	h.setAuthCookies(w, accessToken, refreshToken)

	// If client expects JSON, return user payload directly
	if r.URL.Query().Get("format") == "json" || (strings.Contains(r.Header.Get("Accept"), "application/json") && !strings.Contains(r.Header.Get("Accept"), "text/html")) {
		utils.JSON(w, http.StatusOK, AuthResponse{
			User:    *user,
			Message: "authenticated successfully with Google",
		})
		return
	}

	// Otherwise redirect back to the frontend application
	frontendTarget := strings.TrimRight(h.cfg.FrontendURL, "/") + redirectPath
	http.Redirect(w, r, frontendTarget, http.StatusTemporaryRedirect)
}

// HandleRefreshToken rotates the access and refresh token cookies bound to this verified device.
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken, err := h.cookieManager.GetSignedCookie(r, "refresh_token")
	if err != nil {
		utils.ErrorJSON(w, http.StatusUnauthorized, "missing or invalid refresh token")
		return
	}

	deviceID := ratelimit.GetDeviceID(r.Context())
	userAgent := r.UserAgent()
	ip := getClientIP(r)

	newAccess, newRefresh, err := h.service.RefreshToken(r.Context(), rawRefreshToken, deviceID, userAgent, ip)
	if err != nil {
		h.clearAuthCookies(w)
		switch {
		case errors.Is(err, ErrTokenDeviceMismatch):
			utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized device: session does not belong to this machine")
		case errors.Is(err, ErrTokenReplayDetect):
			utils.ErrorJSON(w, http.StatusUnauthorized, "session compromised; logged out")
		case errors.Is(err, ErrTokenExpired):
			utils.ErrorJSON(w, http.StatusUnauthorized, "refresh token expired; please log in again")
		default:
			utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		}
		return
	}

	h.setAuthCookies(w, newAccess, newRefresh)

	utils.JSON(w, http.StatusOK, MessageResponse{
		Message: "token refreshed successfully",
	})
}

// HandleLogout revokes the current session and clears signed cookies.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if rawRefreshToken, err := h.cookieManager.GetSignedCookie(r, "refresh_token"); err == nil {
		_ = h.service.Logout(r.Context(), rawRefreshToken)
	}

	h.clearAuthCookies(w)

	utils.JSON(w, http.StatusOK, MessageResponse{
		Message: "logged out successfully",
	})
}

// HandleMe returns the currently authenticated user's profile.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.service.GetMe(r.Context(), claims.UserID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusNotFound, "user profile not found")
		return
	}

	utils.JSON(w, http.StatusOK, user)
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	h.cookieManager.SetSignedCookie(w, "access_token", accessToken, h.cfg.AccessCookieExpiry)
	h.cookieManager.SetSignedCookie(w, "refresh_token", refreshToken, h.cfg.RefreshCookieExpiry)
}

func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	h.cookieManager.ClearCookie(w, "access_token")
	h.cookieManager.ClearCookie(w, "refresh_token")
}

func (h *Handler) clearOAuthCookie(w http.ResponseWriter) {
	h.cookieManager.ClearCookie(w, "oauth_state")
}

func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
