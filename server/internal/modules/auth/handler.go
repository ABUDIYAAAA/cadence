package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
)

type Handler struct {
	service       *Service
	cookieManager *utils.CookieManager
	cfg           *config.Config
}

func NewHandler(service *Service, cookieManager *utils.CookieManager, cfg *config.Config) *Handler {
	return &Handler{
		service:       service,
		cookieManager: cookieManager,
		cfg:           cfg,
	}
}

// HandleLogin authenticates the user and sets signed access & refresh cookies.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	userAgent := r.UserAgent()
	ip := getClientIP(r)

	user, accessToken, refreshToken, err := h.service.Login(r.Context(), req, userAgent, ip)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			utils.ErrorJSON(w, http.StatusUnauthorized, "invalid email or password")
		case errors.Is(err, ErrPasswordNotSet):
			utils.ErrorJSON(w, http.StatusForbidden, "password not set; please use the setup link sent by admin")
		case errors.Is(err, ErrInactiveUser):
			utils.ErrorJSON(w, http.StatusForbidden, "user account is deactivated")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.setAuthCookies(w, accessToken, refreshToken)

	utils.JSON(w, http.StatusOK, AuthResponse{
		User:    *user,
		Message: "logged in successfully",
	})
}

// HandleSetupPassword handles first-landing password setup via setup token.
func (h *Handler) HandleSetupPassword(w http.ResponseWriter, r *http.Request) {
	var req SetupPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	userAgent := r.UserAgent()
	ip := getClientIP(r)

	user, accessToken, refreshToken, err := h.service.SetupPassword(r.Context(), req, userAgent, ip)
	if err != nil {
		switch {
		case errors.Is(err, ErrTokenNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "invalid or expired setup token")
		case errors.Is(err, ErrTokenAlreadyUsed):
			utils.ErrorJSON(w, http.StatusGone, "setup token has already been used")
		case errors.Is(err, ErrTokenExpired):
			utils.ErrorJSON(w, http.StatusGone, "setup token has expired")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to configure password")
		}
		return
	}

	h.setAuthCookies(w, accessToken, refreshToken)

	utils.JSON(w, http.StatusOK, AuthResponse{
		User:    *user,
		Message: "password configured successfully",
	})
}

// HandleForgotPassword initiates password reset.
func (h *Handler) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	_, _ = h.service.ForgotPassword(r.Context(), req.Email)

	// Always return generic success to prevent email enumeration
	utils.JSON(w, http.StatusOK, MessageResponse{
		Message: "if the account exists, a password reset link has been dispatched",
	})
}

// HandleResetPassword completes password reset.
func (h *Handler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if err := h.service.ResetPassword(r.Context(), req); err != nil {
		switch {
		case errors.Is(err, ErrTokenNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "invalid or expired reset token")
		case errors.Is(err, ErrTokenAlreadyUsed):
			utils.ErrorJSON(w, http.StatusGone, "reset token has already been used")
		case errors.Is(err, ErrTokenExpired):
			utils.ErrorJSON(w, http.StatusGone, "reset token has expired")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to reset password")
		}
		return
	}

	h.clearAuthCookies(w)

	utils.JSON(w, http.StatusOK, MessageResponse{
		Message: "password reset successfully; please log in with your new password",
	})
}

// HandleRefreshToken rotates the access and refresh token cookies.
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken, err := h.cookieManager.GetSignedCookie(r, "refresh_token")
	if err != nil {
		utils.ErrorJSON(w, http.StatusUnauthorized, "missing or invalid refresh token")
		return
	}

	userAgent := r.UserAgent()
	ip := getClientIP(r)

	newAccess, newRefresh, err := h.service.RefreshToken(r.Context(), rawRefreshToken, userAgent, ip)
	if err != nil {
		h.clearAuthCookies(w)
		switch {
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

func getClientIP(r *http.Request) string {
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		return xForwardedFor
	}
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}
	return r.RemoteAddr
}
