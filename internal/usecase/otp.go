package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"be-modami-auth-service/internal/delivery/http/dto"
	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/pkg/auth"
	"be-modami-auth-service/pkg/email"

	pkgredis "gitlab.com/lifegoeson-libs/pkg-gokit/redis"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

const (
	registerLockPrefix = "register:lock:"
	registerLockTTL    = 30 * time.Second
)

// OTPUseCase defines the interface for OTP operations.
type OTPUseCase interface {
	SendOTP(ctx context.Context, req dto.SendOTPRequest, userID string) error
	VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, userID string) (any, error)
	ResetPassword(ctx context.Context, resetToken, newPassword string) error
}

type otpUseCase struct {
	otpService        *auth.OTPService
	resetTokenService *auth.ResetTokenService
	emailService      *email.EmailService
	authKC            *AuthKeycloakUseCase
	cache             pkgredis.CachePort
}

func NewOTPUseCase(
	otpService *auth.OTPService,
	resetTokenService *auth.ResetTokenService,
	emailService *email.EmailService,
	authKC *AuthKeycloakUseCase,
	cache pkgredis.CachePort,
) OTPUseCase {
	return &otpUseCase{
		otpService:        otpService,
		resetTokenService: resetTokenService,
		emailService:      emailService,
		authKC:            authKC,
		cache:             cache,
	}
}

// ---------------------------------------------------------------------------
// SendOTP — unified for all purposes
// ---------------------------------------------------------------------------

func (uc *otpUseCase) SendOTP(ctx context.Context, req dto.SendOTPRequest, userID string) error {
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	purpose := purposeFromString(req.Purpose)

	// Pre-check per purpose
	switch req.Purpose {
	case dto.PurposeRegister:
		// Email must NOT already be registered
		if _, err := uc.authKC.FindUserByEmail(ctx, emailAddr); err == nil {
			return fmt.Errorf("email đã được đăng ký")
		}
	case dto.PurposeForgotPassword:
		// User must exist
		if _, err := uc.authKC.FindUserByEmail(ctx, emailAddr); err != nil {
			return fmt.Errorf("không tìm thấy email")
		}
	case dto.PurposeChangeEmail:
		// New email must NOT already be taken
		if _, err := uc.authKC.FindUserByEmail(ctx, emailAddr); err == nil {
			return fmt.Errorf("email đã được sử dụng")
		}
	}

	otp, err := uc.otpService.GenerateOTP()
	if err != nil {
		logger.FromContext(ctx).Error("Failed to generate OTP", err)
		return fmt.Errorf("không thể tạo mã OTP")
	}
	if err := uc.otpService.StoreOTP(ctx, purpose, emailAddr, otp); err != nil {
		logger.FromContext(ctx).Error("Failed to store OTP", err)
		return fmt.Errorf("không thể lưu mã OTP")
	}
	if err := uc.emailService.SendOTPEmail(ctx, emailAddr, emailAddr, otp); err != nil {
		logger.FromContext(ctx).Error("Failed to send OTP email", err, logging.String("email", emailAddr))
		return fmt.Errorf("không thể gửi email OTP")
	}

	logger.FromContext(ctx).Info("OTP sent",
		logging.String("purpose", req.Purpose),
		logging.String("email", emailAddr),
	)
	return nil
}

// ---------------------------------------------------------------------------
// VerifyOTP — unified, dispatches to purpose-specific logic after OTP check
// ---------------------------------------------------------------------------

func (uc *otpUseCase) VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, userID string) (any, error) {
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	otp := strings.TrimSpace(req.OTP)
	purpose := purposeFromString(req.Purpose)

	// Verify & delete OTP (common)
	if err := uc.verifyAndDeleteOTP(ctx, purpose, emailAddr, otp); err != nil {
		return nil, err
	}

	// Dispatch to purpose-specific post-verification logic
	switch req.Purpose {
	case dto.PurposeRegister:
		return uc.afterVerifyRegister(ctx, emailAddr, req)
	case dto.PurposeForgotPassword:
		return uc.afterVerifyForgot(ctx, emailAddr)
	case dto.PurposeChangeEmail:
		return uc.afterVerifyChangeEmail(ctx, userID, emailAddr)
	default:
		return nil, fmt.Errorf("mục đích không được hỗ trợ")
	}
}

// ---------------------------------------------------------------------------
// ResetPassword — separate, uses reset token (not OTP)
// ---------------------------------------------------------------------------

func (uc *otpUseCase) ResetPassword(ctx context.Context, resetToken, newPassword string) error {
	data, err := uc.resetTokenService.Validate(ctx, resetToken)
	if err != nil {
		return fmt.Errorf("token đặt lại mật khẩu không hợp lệ hoặc đã hết hạn")
	}
	if err := uc.authKC.SetPasswordByUserID(ctx, data.UserID, newPassword); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("Password reset via OTP", logging.String("email", data.Email))
	return nil
}

// ---------------------------------------------------------------------------
// Post-verification: Register
// ---------------------------------------------------------------------------

func (uc *otpUseCase) afterVerifyRegister(ctx context.Context, emailAddr string, req dto.VerifyOTPRequest) (*entity.LoginResponse, error) {
	// Validate register-specific fields
	if strings.TrimSpace(req.Username) == "" {
		return nil, fmt.Errorf("tên đăng nhập là bắt buộc")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("mật khẩu phải có ít nhất 8 ký tự")
	}

	// Idempotency lock via pipeline SetNX
	lockKey := registerLockPrefix + emailAddr
	pipe := uc.cache.Pipeline()
	setNXCmd := pipe.SetNX(ctx, lockKey, "1", registerLockTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("không thể khởi tạo phiên đăng ký")
	}
	if !setNXCmd.Val() {
		return nil, fmt.Errorf("đang xử lý đăng ký, vui lòng thử lại sau")
	}

	resp, err := uc.authKC.RegisterWithVerifiedEmail(ctx, entity.RegisterRequest{
		Username:  req.Username,
		Email:     emailAddr,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		_ = uc.cache.Delete(ctx, lockKey)
		return nil, err
	}

	logger.FromContext(ctx).Info("User registered via OTP", logging.String("email", emailAddr))
	return resp, nil
}

// ---------------------------------------------------------------------------
// Post-verification: Forgot Password → issue reset token
// ---------------------------------------------------------------------------

func (uc *otpUseCase) afterVerifyForgot(ctx context.Context, emailAddr string) (*dto.ForgotVerifyOTPResponse, error) {
	userID, err := uc.authKC.FindUserByEmail(ctx, emailAddr)
	if err != nil {
		return nil, fmt.Errorf("email not found")
	}

	token, err := uc.resetTokenService.Generate(ctx, emailAddr, userID)
	if err != nil {
		logger.FromContext(ctx).Error("Failed to generate reset token", err)
		return nil, fmt.Errorf("không thể tạo token đặt lại mật khẩu")
	}

	logger.FromContext(ctx).Info("Reset token issued", logging.String("email", emailAddr))
	return &dto.ForgotVerifyOTPResponse{ResetToken: token}, nil
}

// ---------------------------------------------------------------------------
// Post-verification: Change Email → update Keycloak
// ---------------------------------------------------------------------------

func (uc *otpUseCase) afterVerifyChangeEmail(ctx context.Context, userID, newEmail string) (any, error) {
	if userID == "" {
		return nil, fmt.Errorf("không được phép")
	}
	if err := uc.authKC.UpdateUserEmail(ctx, userID, newEmail); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("Email changed via OTP",
		logging.String("new_email", newEmail),
		logging.String("user_id", userID),
	)
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (uc *otpUseCase) verifyAndDeleteOTP(ctx context.Context, purpose auth.OTPPurpose, identifier, code string) error {
	valid, err := uc.otpService.ValidateOTP(ctx, purpose, identifier, code)
	if err != nil {
		if strings.Contains(err.Error(), "max retry exceeded") {
			return fmt.Errorf("đã vượt quá số lần thử tối đa")
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "expired") {
			return fmt.Errorf("mã OTP đã hết hạn")
		}
		if strings.Contains(err.Error(), "invalid") {
			return fmt.Errorf("mã OTP không hợp lệ")
		}
		return err
	}
	if !valid {
		return fmt.Errorf("mã OTP không hợp lệ")
	}
	_ = uc.otpService.DeleteOTP(ctx, purpose, identifier)
	return nil
}

func purposeFromString(s string) auth.OTPPurpose {
	switch s {
	case dto.PurposeRegister:
		return auth.PurposeRegister
	case dto.PurposeForgotPassword:
		return auth.PurposeForgot
	case dto.PurposeChangeEmail:
		return auth.PurposeChangeEmail
	default:
		return auth.OTPPurpose(s)
	}
}
