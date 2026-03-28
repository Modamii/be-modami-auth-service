package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	pkgredis "be-modami-auth-service/pkg/storage/redis"

	goredis "github.com/redis/go-redis/v9"
)

const (
	otpLength  = 6
	maxRetries = 5
)

// OTPPurpose identifies the flow that requested the OTP.
// Each purpose has its own Redis key prefix and TTL.
type OTPPurpose string

const (
	PurposeRegister    OTPPurpose = "register"
	PurposeForgot      OTPPurpose = "forgot"
	PurposeChangeEmail OTPPurpose = "change-email"
)

// OTPData is the JSON document stored in Redis for each OTP.
type OTPData struct {
	Code  string `json:"code"`
	Retry int64  `json:"retry"`
}

// OTPService handles OTP generation, storage, and validation via Redis.
type OTPService struct {
	cache *pkgredis.CacheService
}

func NewOTPService(cache *pkgredis.CacheService) *OTPService {
	return &OTPService{cache: cache}
}

func buildKey(purpose OTPPurpose, identifier string) string {
	return fmt.Sprintf("otp:%s:%s", purpose, identifier)
}

func ttlForPurpose(purpose OTPPurpose) time.Duration {
	switch purpose {
	case PurposeRegister:
		return 60 * time.Second
	default:
		return 10 * time.Minute
	}
}

// GenerateOTP returns a cryptographically random numeric string of otpLength digits.
func (s *OTPService) GenerateOTP() (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(otpLength)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	return fmt.Sprintf("%0*d", otpLength, n), nil
}

// StoreOTP saves the OTP in Redis.
// Key format: otp:{purpose}:{identifier}
func (s *OTPService) StoreOTP(ctx context.Context, purpose OTPPurpose, identifier, code string) error {
	key := buildKey(purpose, identifier)
	data := OTPData{
		Code:  code,
		Retry: 0,
	}
	return s.cache.SetJSON(ctx, key, data, ttlForPurpose(purpose))
}

// ValidateOTP checks the OTP for the given purpose and identifier.
// Returns (true, nil) on success.
// On mismatch it increments the retry counter; if maxRetries is reached
// the key is deleted and an error is returned.
func (s *OTPService) ValidateOTP(ctx context.Context, purpose OTPPurpose, identifier, code string) (bool, error) {
	key := buildKey(purpose, identifier)

	var data OTPData
	if err := s.cache.GetJSON(ctx, key, &data); err != nil {
		if err == goredis.Nil {
			return false, fmt.Errorf("otp not found or expired")
		}
		return false, fmt.Errorf("read otp: %w", err)
	}

	if data.Retry >= int64(maxRetries) {
		_ = s.cache.Delete(ctx, key)
		return false, fmt.Errorf("max retry exceeded")
	}

	if data.Code != code {
		newRetry, _ := s.cache.JSONIncrementField(ctx, key, "retry", 1)
		if newRetry >= int64(maxRetries) {
			_ = s.cache.Delete(ctx, key)
			return false, fmt.Errorf("max retry exceeded")
		}
		return false, fmt.Errorf("invalid otp")
	}

	return true, nil
}

// DeleteOTP removes the OTP key.
func (s *OTPService) DeleteOTP(ctx context.Context, purpose OTPPurpose, identifier string) error {
	return s.cache.Delete(ctx, buildKey(purpose, identifier))
}
