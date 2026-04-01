package handler

import (
	"net/http"

	"be-modami-auth-service/internal/delivery/http/dto"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/ctxutil"
	pkgerrors "be-modami-auth-service/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type OTPHandler struct {
	otpUsecase usecase.OTPUseCase
	validator  *validator.Validate
}

func NewOTPHandler(otpUsecase usecase.OTPUseCase, validator *validator.Validate) *OTPHandler {
	return &OTPHandler{
		otpUsecase: otpUsecase,
		validator:  validator,
	}
}

// SendOTP godoc
// @Summary      Send OTP
// @Description  Send OTP to the given email. Purpose: register, forgot-password, change-email.
// @Tags         otp
// @Accept       json
// @Produce      json
// @Param        body body dto.SendOTPRequest true "Email and purpose"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]interface{}
// @Router       /auth/otp/send [post]
func (h *OTPHandler) SendOTP(c *gin.Context) {
	var req dto.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng yêu cầu không hợp lệ"})
		return
	}
	if err := h.validator.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Xác thực dữ liệu thất bại", "details": pkgerrors.FormatValidationErrors(err)})
		return
	}

	// change-email requires authenticated user
	var userID string
	if req.Purpose == dto.PurposeChangeEmail {
		claims, ok := ctxutil.GetClaims(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "không được phép"})
			return
		}
		userID = claims.Sub
	}

	if err := h.otpUsecase.SendOTP(c.Request.Context(), req, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Mã OTP đã được gửi thành công"})
}

// VerifyOTP godoc
// @Summary      Verify OTP
// @Description  Verify OTP. Response depends on purpose: register returns tokens, forgot-password returns reset_token, change-email returns success.
// @Tags         otp
// @Accept       json
// @Produce      json
// @Param        body body dto.VerifyOTPRequest true "Email, OTP, purpose, and optional fields"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]interface{}
// @Router       /auth/otp/verify [post]
func (h *OTPHandler) VerifyOTP(c *gin.Context) {
	var req dto.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng yêu cầu không hợp lệ"})
		return
	}
	if err := h.validator.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Xác thực dữ liệu thất bại", "details": pkgerrors.FormatValidationErrors(err)})
		return
	}

	// change-email requires authenticated user
	var userID string
	if req.Purpose == dto.PurposeChangeEmail {
		claims, ok := ctxutil.GetClaims(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "không được phép"})
			return
		}
		userID = claims.Sub
	}

	result, err := h.otpUsecase.VerifyOTP(c.Request.Context(), req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Purpose-specific response
	switch req.Purpose {
	case dto.PurposeRegister:
		c.JSON(http.StatusOK, result)
	case dto.PurposeForgotPassword:
		c.JSON(http.StatusOK, result)
	case dto.PurposeChangeEmail:
		c.JSON(http.StatusOK, gin.H{"message": "Email đã được cập nhật thành công"})
	}
}

// ResetPassword godoc
// @Summary      Reset password with reset token
// @Description  Use the one-time reset token (from forgot-password verify) to set a new password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.ResetPasswordRequest true "Reset token and new password"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]interface{}
// @Router       /auth/reset-password [post]
func (h *OTPHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng yêu cầu không hợp lệ"})
		return
	}
	if err := h.validator.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Xác thực dữ liệu thất bại", "details": pkgerrors.FormatValidationErrors(err)})
		return
	}
	if err := h.otpUsecase.ResetPassword(c.Request.Context(), req.ResetToken, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đặt lại mật khẩu thành công"})
}
