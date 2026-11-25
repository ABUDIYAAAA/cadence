package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/logger"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
	"log/slog"
)

var (
	ErrInactiveUser        = errors.New("user account is inactive")
	ErrTokenExpired        = errors.New("token has expired")
	ErrSessionRevoked      = errors.New("session has been revoked")
	ErrTokenReplayDetect   = errors.New("token replay detected: session compromised")
	ErrTokenDeviceMismatch = errors.New("device mismatch: session does not belong to this device")
	ErrOAuthExchangeFailed = errors.New("failed to exchange authorization code with Google")
	ErrOAuthProfileFailed  = errors.New("failed to retrieve profile from Google")
)

type Service struct {
	repo       *Repository
	cfg        *config.Config
	auditor    audit.Recorder
	httpClient *http.Client
}

func NewService(repo *Repository, cfg *config.Config, auditor audit.Recorder) *Service {
	return &Service{
		repo:       repo,
		cfg:        cfg,
		auditor:    auditor,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetGoogleAuthURL generates the Google OAuth authorization URL with state nonce.
func (s *Service) GetGoogleAuthURL(state string) string {
	redirectURI := s.cfg.GetGoogleRedirectURL()
	params := url.Values{}
	params.Set("client_id", s.cfg.GoogleClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid profile email")
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "select_account")

	return fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?%s", params.Encode())
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	EmailVerified bool   `json:"email_verified"`
}

// HandleGoogleCallback exchanges code for user profile, upserts user, and establishes session.
func (s *Service) HandleGoogleCallback(ctx context.Context, code, deviceID, userAgent, ip string) (*UserResponse, string, string, error) {
	sanitizedUA := truncateUserAgent(userAgent)

	// 1. Exchange authorization code for Google access token
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.cfg.GoogleClientID)
	form.Set("client_secret", s.cfg.GoogleClientSecret)
	form.Set("redirect_uri", s.cfg.GetGoogleRedirectURL())
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		metrics.InstrumentAuthEvent("google_login", false)
		logger.FromContext(ctx).Error("google oauth token exchange failed", slog.Any("error", err))
		if s.auditor != nil {
			s.auditor.RecordAction(ctx, audit.ActionAuthLoginFailure, audit.TargetAuth, nil, "Failed Google OAuth login: token exchange HTTP error", map[string]any{"error": err.Error()})
		}
		return nil, "", "", fmt.Errorf("%w: %v", ErrOAuthExchangeFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		metrics.InstrumentAuthEvent("google_login", false)
		body, _ := io.ReadAll(resp.Body)
		logger.FromContext(ctx).Warn("google oauth token exchange non-200 status", slog.Int("status", resp.StatusCode))
		if s.auditor != nil {
			s.auditor.RecordAction(ctx, audit.ActionAuthLoginFailure, audit.TargetAuth, nil, fmt.Sprintf("Failed Google OAuth login: token endpoint returned status %d", resp.StatusCode), map[string]any{"status": resp.StatusCode})
		}
		return nil, "", "", fmt.Errorf("%w: google returned status %d (%s)", ErrOAuthExchangeFailed, resp.StatusCode, string(body))
	}

	var tokenResp googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, "", "", fmt.Errorf("decode token response: %w", err)
	}

	// 2. Fetch user profile with the access token
	profileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("create profile request: %w", err)
	}
	profileReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	profileResp, err := s.httpClient.Do(profileReq)
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: %v", ErrOAuthProfileFailed, err)
	}
	defer profileResp.Body.Close()

	if profileResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(profileResp.Body)
		return nil, "", "", fmt.Errorf("%w: status %d (%s)", ErrOAuthProfileFailed, profileResp.StatusCode, string(body))
	}

	var gUser googleUserInfo
	if err := json.NewDecoder(profileResp.Body).Decode(&gUser); err != nil {
		return nil, "", "", fmt.Errorf("decode profile response: %w", err)
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(gUser.Email))
	if cleanEmail == "" {
		return nil, "", "", errors.New("google account has no email address")
	}

	// Determine default role: seed admin email receives dev superuser role
	defaultRole := RoleUser
	if s.cfg.SeedAdminEmail != "" && strings.EqualFold(cleanEmail, s.cfg.SeedAdminEmail) {
		defaultRole = RoleDev
	}

	var pic *string
	if gUser.Picture != "" {
		pic = &gUser.Picture
	}

	// 3. Upsert user in database
	user, err := s.repo.UpsertGoogleUser(ctx, gUser.Sub, cleanEmail, gUser.Name, pic, defaultRole)
	if err != nil {
		return nil, "", "", fmt.Errorf("upsert user: %w", err)
	}

	if !user.IsActive {
		if s.auditor != nil {
			s.auditor.Record(ctx, audit.Event{
				ActorID:     &user.ID,
				ActorEmail:  &user.Email,
				Action:      audit.ActionAuthLoginFailure,
				TargetType:  audit.TargetAuth,
				TargetID:    &user.ID,
				Description: fmt.Sprintf("Failed login: account %s is deactivated", user.Email),
			})
		}
		return nil, "", "", ErrInactiveUser
	}

	// Deduplicate active sessions on this specific device
	_ = s.repo.RevokeDeviceSessions(ctx, user.ID, deviceID)

	// 4. Create session & JWT
	accessToken, refreshToken, err := s.createSession(ctx, user, deviceID, nil, nil, sanitizedUA, ip)
	if err != nil {
		metrics.InstrumentAuthEvent("google_login", false)
		return nil, "", "", err
	}

	evs, _ := s.repo.GetUserEvents(ctx, user.ID)

	metrics.InstrumentAuthEvent("google_login", true)
	if s.auditor != nil {
		s.auditor.Record(ctx, audit.Event{
			ActorID:     &user.ID,
			ActorEmail:  &user.Email,
			ActorRole:   &user.Role,
			Action:      audit.ActionAuthLoginSuccess,
			TargetType:  audit.TargetAuth,
			TargetID:    &user.ID,
			Description: fmt.Sprintf("User %s authenticated via Google OAuth", user.Email),
		})
	}

	return toUserResponse(user, evs), accessToken, refreshToken, nil
}

func (s *Service) RefreshToken(ctx context.Context, rawRefreshToken, deviceID, userAgent, ip string) (string, string, error) {
	tokenHash := utils.HashToken(rawRefreshToken)

	session, err := s.repo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		metrics.InstrumentAuthEvent("refresh", false)
		return "", "", ErrTokenNotFound
	}

	if session.DeviceID != "" && session.DeviceID != deviceID {
		metrics.InstrumentAuthEvent("refresh", false)
		metrics.InstrumentSecurityThreat("device_hijack")
		_ = s.repo.RevokeAllUserTokens(ctx, session.UserID)
		if s.auditor != nil {
			s.auditor.Record(ctx, audit.Event{
				ActorID:     &session.UserID,
				Action:      audit.ActionAuthLoginFailure,
				TargetType:  audit.TargetSession,
				TargetID:    &session.UserID,
				Description: "Device mismatch: refresh token used from unrecognized device",
				Metadata:    map[string]any{"expected_device": session.DeviceID, "actual_device": deviceID},
			})
		}
		return "", "", ErrTokenDeviceMismatch
	}

	if session.IsRevoked {
		metrics.InstrumentAuthEvent("refresh", false)
		metrics.InstrumentSecurityThreat("token_replay")
		_ = s.repo.RevokeAllUserTokens(ctx, session.UserID)
		if s.auditor != nil {
			s.auditor.Record(ctx, audit.Event{
				ActorID:     &session.UserID,
				Action:      audit.ActionAuthLoginFailure,
				TargetType:  audit.TargetSession,
				TargetID:    &session.UserID,
				Description: "Token replay detected: revoked refresh token used again",
				Metadata:    map[string]any{"session_id": session.ID},
			})
		}
		return "", "", ErrTokenReplayDetect
	}

	if time.Now().After(session.ExpiresAt) {
		metrics.InstrumentAuthEvent("refresh", false)
		return "", "", ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		metrics.InstrumentAuthEvent("refresh", false)
		return "", "", err
	}

	if !user.IsActive {
		metrics.InstrumentAuthEvent("refresh", false)
		return "", "", ErrInactiveUser
	}

	// Revoke the old token
	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		metrics.InstrumentAuthEvent("refresh", false)
		return "", "", err
	}

	// Issue rotated token bound to the same device & family
	familyID := session.FamilyID
	parentID := &session.ID
	newAccess, newRefresh, err := s.createSession(ctx, user, deviceID, &familyID, parentID, userAgent, ip)
	if err != nil {
		metrics.InstrumentAuthEvent("refresh", false)
		return "", "", err
	}

	metrics.InstrumentAuthEvent("refresh", true)
	return newAccess, newRefresh, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	tokenHash := utils.HashToken(rawRefreshToken)
	session, _ := s.repo.GetRefreshTokenByHash(ctx, tokenHash)
	if session != nil && s.auditor != nil {
		s.auditor.Record(ctx, audit.Event{
			ActorID:     &session.UserID,
			Action:      audit.ActionAuthLogout,
			TargetType:  audit.TargetSession,
			TargetID:    &session.UserID,
			Description: "User logged out",
		})
	}
	return s.repo.RevokeRefreshToken(ctx, tokenHash)
}

func (s *Service) GetMe(ctx context.Context, userID string) (*UserResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	evs, _ := s.repo.GetUserEvents(ctx, userID)
	return toUserResponse(user, evs), nil
}

func (s *Service) createSession(ctx context.Context, user *User, deviceID string, familyID *string, parentID *string, userAgent, ip string) (string, string, error) {
	accessToken, err := utils.GenerateAccessToken(
		user.ID,
		user.Email,
		user.Role,
		user.TokenVersion,
		s.cfg.JWTAccessSecret,
		s.cfg.JWTAccessExpiry,
	)
	if err != nil {
		return "", "", err
	}

	rawRefreshToken, err := utils.GenerateRandomToken(32)
	if err != nil {
		return "", "", err
	}

	session := &RefreshToken{
		UserID:    user.ID,
		DeviceID:  deviceID,
		TokenHash: utils.HashToken(rawRefreshToken),
		ParentID:  parentID,
		UserAgent: &userAgent,
		IPAddress: &ip,
		ExpiresAt: time.Now().Add(s.cfg.JWTRefreshExpiry),
	}
	if familyID != nil {
		session.FamilyID = *familyID
	}

	if err := s.repo.CreateRefreshToken(ctx, session); err != nil {
		return "", "", err
	}

	return accessToken, rawRefreshToken, nil
}

func toUserResponse(u *User, evs []UserEventRole) *UserResponse {
	role := u.Role
	if role != RoleDev && len(evs) > 0 {
		role = evs[0].Role
	}
	if evs == nil {
		evs = make([]UserEventRole, 0)
	}

	return &UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		PhoneNumber: u.PhoneNumber,
		GoogleID:    u.GoogleID,
		AvatarURL:   u.AvatarURL,
		Role:        role,
		Events:      evs,
		CreatedAt:   u.CreatedAt,
	}
}

func truncateUserAgent(ua string) string {
	const maxUALen = 512
	cleaned := strings.ReplaceAll(ua, "\x00", "")
	if len(cleaned) > maxUALen {
		return cleaned[:maxUALen]
	}
	return cleaned
}
