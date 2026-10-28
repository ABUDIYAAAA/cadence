package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInactiveUser         = errors.New("user account is inactive")
	ErrPasswordNotSet       = errors.New("password has not been configured yet")
	ErrTokenExpired         = errors.New("token has expired")
	ErrTokenAlreadyUsed     = errors.New("token has already been used")
	ErrSessionRevoked       = errors.New("session has been revoked")
	ErrTokenReplayDetect    = errors.New("token replay detected: session compromised")
	ErrTokenDeviceMismatch  = errors.New("device mismatch: session does not belong to this device")
)

type EmailSender interface {
	SendSetupPasswordEmail(ctx context.Context, to, name, setupURL string, expiryHours int) error
	SendResetPasswordEmail(ctx context.Context, to, name, resetURL string, expiryHours int) error
}

type Service struct {
	repo        *Repository
	cfg         *config.Config
	emailSender EmailSender
}

func NewService(repo *Repository, cfg *config.Config, emailSender EmailSender) *Service {
	return &Service{
		repo:        repo,
		cfg:         cfg,
		emailSender: emailSender,
	}
}

func (s *Service) Login(ctx context.Context, req LoginRequest, deviceID, userAgent, ip string) (*UserResponse, string, string, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, "", "", ErrInvalidCredentials
		}
		return nil, "", "", err
	}

	if !user.IsActive {
		return nil, "", "", ErrInactiveUser
	}

	if !user.IsPasswordConfigured || user.PasswordHash == nil {
		return nil, "", "", ErrPasswordNotSet
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, "", "", ErrInvalidCredentials
	}

	// Deduplicate active sessions on this specific device
	_ = s.repo.RevokeDeviceSessions(ctx, user.ID, deviceID)

	accessToken, refreshToken, err := s.createSession(ctx, user, deviceID, nil, nil, userAgent, ip)
	if err != nil {
		return nil, "", "", err
	}

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
		_ = s.repo.RevokeAllUserTokens(ctx, session.UserID)
		return "", "", ErrTokenDeviceMismatch
	}

	// Replay attack detection: if token is already revoked, compromise detected
	if session.IsRevoked {
		_ = s.repo.RevokeAllUserTokens(ctx, session.UserID)
		return "", "", ErrTokenReplayDetect
	}

	if time.Now().After(session.ExpiresAt) {
		return "", "", ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return "", "", err
	}

	if !user.IsActive {
		return "", "", ErrInactiveUser
	}

	// Revoke the old token
	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return "", "", err
	}

	// Issue rotated token bound to the same device & family
	familyID := session.FamilyID
	parentID := &session.ID
	newAccess, newRefresh, err := s.createSession(ctx, user, deviceID, &familyID, parentID, userAgent, ip)
	if err != nil {
		return "", "", err
	}

	return newAccess, newRefresh, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	tokenHash := utils.HashToken(rawRefreshToken)
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
