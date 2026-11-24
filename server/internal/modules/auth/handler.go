package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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

func isSafeRelativePath(p string) bool {
	return p != "" && strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//")
}

// HandleGoogleLogin initiates Google OAuth flow by generating a secure state and redirecting.
func (h *Handler) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	returnPath := r.URL.Query().Get("redirect")
	if !isSafeRelativePath(returnPath) {
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

// HandleGoogleCallback processes the OAuth redirect from Google, authenticates the user, sets cookies,
// and redirects back to the frontend with query parameters indicating success/failure and the user's events.
func (h *Handler) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateParam := r.URL.Query().Get("state")
	redirectPath := "/"
	if stateParam != "" {
		var statePayload oauthStatePayload
		if stateBytes, err := base64.RawURLEncoding.DecodeString(stateParam); err == nil {
			_ = json.Unmarshal(stateBytes, &statePayload)
		}
		if isSafeRelativePath(statePayload.Redirect) {
			redirectPath = statePayload.Redirect
		}
	}

	// Check for Google error response (e.g. user denied consent)
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		logger.FromContext(r.Context()).Warn("google oauth denied or returned error", slog.String("error", errParam))
		h.clearOAuthCookie(w)
		h.respondOrRedirectOAuth(w, r, redirectPath, false, errParam, http.StatusBadRequest, "google authentication failed: "+errParam, nil)
		return
	}

	codeParam := r.URL.Query().Get("code")
	if stateParam == "" || codeParam == "" {
		h.clearOAuthCookie(w)
		h.respondOrRedirectOAuth(w, r, redirectPath, false, "missing_state_or_code", http.StatusBadRequest, "missing state or code parameter", nil)
		return
	}

	// Validate signed state cookie against parameter
	cookieState, err := h.cookieManager.GetSignedCookie(r, "oauth_state")
	if err != nil || cookieState != stateParam {
		h.clearOAuthCookie(w)
		h.respondOrRedirectOAuth(w, r, redirectPath, false, "invalid_oauth_state", http.StatusBadRequest, "invalid or expired oauth state parameter; please try logging in again", nil)
		return
	}
	h.clearOAuthCookie(w)

	deviceID := ratelimit.GetDeviceID(r.Context())
	userAgent := r.UserAgent()
	ip := getClientIP(r)

	user, accessToken, refreshToken, err := h.service.HandleGoogleCallback(r.Context(), codeParam, deviceID, userAgent, ip)
	if err != nil {
		switch {
		case errors.Is(err, ErrInactiveUser):
			h.respondOrRedirectOAuth(w, r, redirectPath, false, "account_deactivated", http.StatusForbidden, "user account is deactivated", nil)
		case errors.Is(err, ErrOAuthExchangeFailed):
			h.respondOrRedirectOAuth(w, r, redirectPath, false, "oauth_exchange_failed", http.StatusBadGateway, "failed to exchange authorization token with Google", nil)
		case errors.Is(err, ErrOAuthProfileFailed):
			h.respondOrRedirectOAuth(w, r, redirectPath, false, "oauth_profile_failed", http.StatusBadGateway, "failed to retrieve profile from Google", nil)
		default:
			logger.FromContext(r.Context()).Error("google callback internal error", slog.Any("error", err))
			h.respondOrRedirectOAuth(w, r, redirectPath, false, "authentication_failed", http.StatusInternalServerError, "authentication failed", nil)
		}
		return
	}

	h.setAuthCookies(w, accessToken, refreshToken)
	h.respondOrRedirectOAuth(w, r, redirectPath, true, "", http.StatusOK, "authenticated successfully with Google", user)
}

func (h *Handler) respondOrRedirectOAuth(
	w http.ResponseWriter,
	r *http.Request,
	redirectPath string,
	success bool,
	errCode string,
	httpStatus int,
	msg string,
	user *UserResponse,
) {
	wantsJSON := r.URL.Query().Get("format") == "json" ||
		(strings.Contains(r.Header.Get("Accept"), "application/json") && !strings.Contains(r.Header.Get("Accept"), "text/html"))

	if wantsJSON {
		if !success {
			utils.ErrorJSON(w, httpStatus, msg)
			return
		}
		utils.JSON(w, http.StatusOK, AuthResponse{
			User:    *user,
			Message: msg,
		})
		return
	}

	var targetURL *url.URL
	var err error
	if h.cfg.FrontendOAuthRedirectURL != "" {
		targetURL, err = url.Parse(h.cfg.FrontendOAuthRedirectURL)
	} else {
		if !isSafeRelativePath(redirectPath) {
			redirectPath = "/"
		}
		base := strings.TrimRight(h.cfg.FrontendURL, "/") + redirectPath
		targetURL, err = url.Parse(base)
	}
	if err != nil || targetURL == nil {
		targetURL, _ = url.Parse(strings.TrimRight(h.cfg.FrontendURL, "/") + "/")
	}

	q := targetURL.Query()
	if h.cfg.FrontendOAuthRedirectURL != "" && isSafeRelativePath(redirectPath) && redirectPath != "/" {
		q.Set("redirect", redirectPath)
	}
	if success {
		q.Set("success", "true")
		evs := make([]UserEventRole, 0)
		if user != nil && user.Events != nil {
			evs = user.Events
		}
		if eventsJSON, err := json.Marshal(evs); err == nil {
			q.Set("events", string(eventsJSON))
		}
	} else {
		q.Set("success", "false")
		if errCode != "" {
			q.Set("error", errCode)
		}
		q.Set("events", "[]")
	}
	targetURL.RawQuery = q.Encode()

	http.Redirect(w, r, targetURL.String(), http.StatusTemporaryRedirect)
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
