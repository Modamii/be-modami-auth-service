package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	pkgredis "gitlab.com/lifegoeson-libs/pkg-gokit/redis"
)

const (
	resetTokenTTL       = 10 * time.Minute
	resetTokenKeyPrefix = "reset:"
)

// ResetTokenData is the JSON payload stored in Redis for a password-reset token.
type ResetTokenData struct {
	Email  string `json:"email"`
	UserID string `json:"user_id"`
}

// ResetTokenService handles generation and one-time validation of password-reset tokens.
type ResetTokenService struct {
	cache pkgredis.CachePort
}

func NewResetTokenService(cache pkgredis.CachePort) *ResetTokenService {
	return &ResetTokenService{cache: cache}
}

// Generate creates a new reset token, stores it in Redis, and returns the token string.
func (s *ResetTokenService) Generate(ctx context.Context, email, userID string) (string, error) {
	token := uuid.New().String()
	key := resetTokenKeyPrefix + token
	if err := s.cache.SetJSON(ctx, key, ResetTokenData{Email: email, UserID: userID}, resetTokenTTL); err != nil {
		return "", fmt.Errorf("store reset token: %w", err)
	}
	return token, nil
}

// Validate reads and deletes the reset token (one-time use).
func (s *ResetTokenService) Validate(ctx context.Context, token string) (*ResetTokenData, error) {
	key := resetTokenKeyPrefix + token
	var data ResetTokenData
	if err := s.cache.GetJSON(ctx, key, &data); err != nil {
		if err == pkgredis.ErrCacheMiss {
			return nil, fmt.Errorf("reset token invalid or expired")
		}
		return nil, fmt.Errorf("read reset token: %w", err)
	}
	_ = s.cache.Delete(ctx, key)
	return &data, nil
}
