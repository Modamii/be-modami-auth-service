package auth

import (
	"context"
	"fmt"
	"time"

	pkgredis "be-modami-auth-service/pkg/storage/redis"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
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

// ResetTokenService handles generation and one-time validation of
// password-reset tokens backed by Redis.
type ResetTokenService struct {
	cache *pkgredis.CacheService
}

func NewResetTokenService(cache *pkgredis.CacheService) *ResetTokenService {
	return &ResetTokenService{cache: cache}
}

// Generate creates a new reset token, stores it in Redis with a short TTL,
// and returns the token string.
func (s *ResetTokenService) Generate(ctx context.Context, email, userID string) (string, error) {
	token := uuid.New().String()
	key := resetTokenKeyPrefix + token
	data := ResetTokenData{
		Email:  email,
		UserID: userID,
	}
	if err := s.cache.SetJSON(ctx, key, data, resetTokenTTL); err != nil {
		return "", fmt.Errorf("store reset token: %w", err)
	}
	return token, nil
}

// Validate reads and deletes the reset token (one-time use).
// Returns the associated data or an error if the token is invalid/expired.
func (s *ResetTokenService) Validate(ctx context.Context, token string) (*ResetTokenData, error) {
	key := resetTokenKeyPrefix + token
	var data ResetTokenData
	if err := s.cache.GetJSON(ctx, key, &data); err != nil {
		if err == goredis.Nil {
			return nil, fmt.Errorf("reset token invalid or expired")
		}
		return nil, fmt.Errorf("read reset token: %w", err)
	}
	// One-time: delete after reading
	_ = s.cache.Delete(ctx, key)
	return &data, nil
}
