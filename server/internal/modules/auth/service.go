package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInactiveUser        = errors.New("user account is inactive")
	ErrPasswordNotSet      = errors.New("password has not been configured yet")
	ErrTokenExpired        = errors.New("token has expired")
	ErrTokenAlreadyUsed    = errors.New("token has already been used")
	ErrSessionRevoked      = errors.New("session has been revoked")
	ErrTokenReplayDetect   = errors.New("token replay detected: session compromised")
	ErrTokenDeviceMismatch = errors.New("device mismatch: session does not belong to this device")
)

type EmailSender interface {
	SendSetupPasswordEmail(ctx context.Context, to, name, setupURL string, expiryHours int) error
	SendResetPasswordEmail(ctx context.Context, to, name, resetURL string, expiryHours int) error
}

type Service struct {
	repo        *Repository
	cfg         *config.Config
	emailSender EmailSender
	auditor     audit.Recorder
}

func NewService(repo *Repository, cfg *config.Config, emailSender EmailSender, auditor audit.Recorder) *Service {
	return &Service{
		repo:        repo,
		cfg:         cfg,
		emailSender: emailSender,
		auditor:     auditor,
	}
}

func (s *Service) Login(ctx context.Context, req LoginRequest, deviceID, userAgent, ip string) (*UserResponse, string, string, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			s.recordAudit(ctx, audit.ActionAuthLoginFailure, audit.TargetAuth, nil, fmt.Sprintf("Failed login: unknown email %s", req.Email), &req.Email, nil, nil, &deviceID, &userAgent, &ip)
			return nil, "", "", ErrInvalidCredentials
		}
		return nil, "", "", err
	}

	if !user.IsActive {
		s.recordAudit(ctx, audit.ActionAuthLoginFailure, audit.TargetAuth, &user.ID, fmt.Sprintf("Failed login: account %s is deactivated", req.Email), &user.Email, &user.Role, nil, &deviceID, &userAgent, &ip)
		return nil, "", "", ErrInactiveUser
	}

	if !user.IsPasswordConfigured || user.PasswordHash == nil {
		return nil, "", "", ErrPasswordNotSet
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		metrics.InstrumentAuthEvent("login", false)
		s.recordAudit(ctx, audit.ActionAuthLoginFailure, audit.TargetAuth, &user.ID, fmt.Sprintf("Failed login: invalid password for %s", req.Email), &user.Email, &user.Role, nil, &deviceID, &userAgent, &ip)
		return nil, "", "", ErrInvalidCredentials
	}

	// Deduplicate active sessions on this specific device
	_ = s.repo.RevokeDeviceSessions(ctx, user.ID, deviceID)

	accessToken, refreshToken, err := s.createSession(ctx, user, deviceID, nil, nil, userAgent, ip)
	if err != nil {
		metrics.InstrumentAuthEvent("login", false)
		return nil, "", "", err
	}

	metrics.InstrumentAuthEvent("login", true)
	s.recordAudit(ctx, audit.ActionAuthLoginSuccess, audit.TargetAuth, &user.ID, fmt.Sprintf("User %s logged in successfully", user.Email), &user.Email, &user.Role, nil, &deviceID, &userAgent, &ip)

	return toUserResponse(user), accessToken, refreshToken, nil
}

func (s *Service) SetupPassword(ctx context.Context, req SetupPasswordRequest, deviceID, userAgent, ip string) (*UserResponse, string, string, error) {
	tokenHash := utils.HashToken(req.Token)
	resetToken, err := s.repo.GetResetTokenByHash(ctx, tokenHash, "initial_setup")
	if err != nil {
		return nil, "", "", ErrTokenNotFound
	}

	if resetToken.UsedAt != nil {
		return nil, "", "", ErrTokenAlreadyUsed
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return nil, "", "", ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, resetToken.UserID)
	if err != nil {
		return nil, "", "", err
	}

	if !user.IsActive {
		return nil, "", "", ErrInactiveUser
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return nil, "", "", err
	}

	if err := s.repo.MarkResetTokenUsed(ctx, resetToken.ID); err != nil {
		return nil, "", "", err
	}

	user.IsPasswordConfigured = true
	user.TokenVersion++

	accessToken, refreshToken, err := s.createSession(ctx, user, deviceID, nil, nil, userAgent, ip)
	if err != nil {
		return nil, "", "", err
	}

	s.recordAudit(ctx, audit.ActionAuthPasswordSetup, audit.TargetUser, &user.ID, fmt.Sprintf("User %s configured their initial password", user.Email), &user.Email, &user.Role, nil, &deviceID, &userAgent, &ip)

	return toUserResponse(user), accessToken, refreshToken, nil
}

func (s *Service) ForgotPassword(ctx context.Context, email string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Silently return to prevent user enumeration attacks
			return "", nil
		}
		return "", err
	}

	if !user.IsActive {
		return "", nil
	}

	rawToken, err := utils.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}

	token := &ResetToken{
		UserID:    user.ID,
		TokenHash: utils.HashToken(rawToken),
		Purpose:   "forgot_password",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if err := s.repo.CreateResetToken(ctx, token); err != nil {
		return "", err
	}

	if s.emailSender != nil {
		resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", s.cfg.FrontendURL, rawToken)
		_ = s.emailSender.SendResetPasswordEmail(ctx, user.Email, user.Name, resetURL, 1)
	}

	return rawToken, nil
}

func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	tokenHash := utils.HashToken(req.Token)
	resetToken, err := s.repo.GetResetTokenByHash(ctx, tokenHash, "forgot_password")
	if err != nil {
		return ErrTokenNotFound
	}

	if resetToken.UsedAt != nil {
		return ErrTokenAlreadyUsed
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, resetToken.UserID)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return err
	}

	if err := s.repo.MarkResetTokenUsed(ctx, resetToken.ID); err != nil {
		return err
	}

	// Invalidate all existing refresh sessions for instant logout on password change
	_ = s.repo.RevokeAllUserTokens(ctx, user.ID)

	s.recordAudit(ctx, audit.ActionAuthPasswordReset, audit.TargetUser, &user.ID, fmt.Sprintf("User %s reset their password", user.Email), &user.Email, &user.Role, nil, nil, nil, nil)

	return nil
}

func (s *Service) RefreshToken(ctx context.Context, rawRefreshToken, deviceID, userAgent, ip string) (string, string, error) {
	tokenHash := utils.HashToken(rawRefreshToken)
	session, err := s.repo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return "", "", ErrTokenNotFound
	}

	// Anti-Hijacking check: verify token is being refreshed from the same signed device that received it
	if session.DeviceID != deviceID {
		metrics.InstrumentAuthEvent("refresh", false)
		metrics.InstrumentSecurityThreat("device_mismatch")
		_ = s.repo.RevokeAllUserTokens(ctx, session.UserID)
		s.recordAudit(ctx, audit.ActionAuthLoginFailure, audit.TargetSession, &session.UserID, "Session hijack detected: refresh token replayed from different device", nil, nil, map[string]any{"session_id": session.ID, "expected_device": session.DeviceID, "actual_device": deviceID}, &deviceID, &userAgent, &ip)
		return "", "", ErrTokenDeviceMismatch
	}

	// Replay attack detection: if token is already revoked, compromise detected
	if session.IsRevoked {
		metrics.InstrumentAuthEvent("refresh", false)
		metrics.InstrumentSecurityThreat("token_replay")
		_ = s.repo.RevokeAllUserTokens(ctx, session.UserID)
		s.recordAudit(ctx, audit.ActionAuthLoginFailure, audit.TargetSession, &session.UserID, "Token replay detected: revoked refresh token used again", nil, nil, map[string]any{"session_id": session.ID}, &deviceID, &userAgent, &ip)
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
	if session != nil {
		metrics.InstrumentAuthEvent("logout", true)
		s.recordAudit(ctx, audit.ActionAuthLogout, audit.TargetSession, &session.UserID, "User logged out session", nil, nil, map[string]any{"session_id": session.ID}, &session.DeviceID, session.UserAgent, session.IPAddress)
	}
	return s.repo.RevokeRefreshToken(ctx, tokenHash)
}

func (s *Service) GetMe(ctx context.Context, userID string) (*UserResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserResponse(user), nil
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

func (s *Service) recordAudit(
	ctx context.Context,
	action, targetType string,
	targetID *string,
	description string,
	actorEmail, actorRole *string,
	metadata map[string]any,
	deviceID, userAgent, ip *string,
) {
	if s.auditor == nil {
		return
	}
	s.auditor.Record(ctx, audit.Event{
		ActorID:     targetID,
		ActorEmail:  actorEmail,
		ActorRole:   actorRole,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Description: description,
		Metadata:    metadata,
		DeviceID:    deviceID,
		UserAgent:   userAgent,
		IPAddress:   ip,
	})
}

func toUserResponse(u *User) *UserResponse {
	return &UserResponse{
		ID:                   u.ID,
		Email:                u.Email,
		Name:                 u.Name,
		PhoneNumber:          u.PhoneNumber,
		Role:                 u.Role,
		TeamID:               u.TeamID,
		IsPasswordConfigured: u.IsPasswordConfigured,
		CreatedAt:            u.CreatedAt,
	}
}
