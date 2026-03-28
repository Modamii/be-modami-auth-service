package dto

const (
	PurposeRegister       = "register"
	PurposeForgotPassword = "forgot-password"
	PurposeChangeEmail    = "change-email"
)

// SendOTPRequest is the unified request for POST /auth/otp/send.
type SendOTPRequest struct {
	Email   string `json:"email"   validate:"required,email"`
	Purpose string `json:"purpose" validate:"required,oneof=register forgot-password change-email"`
}

// VerifyOTPRequest is the unified request for POST /auth/otp/verify.
// Fields Username/Password/FirstName/LastName are required only when Purpose = "register".
type VerifyOTPRequest struct {
	Email   string `json:"email"   validate:"required,email"`
	OTP     string `json:"otp"     validate:"required,len=6"`
	Purpose string `json:"purpose" validate:"required,oneof=register forgot-password change-email"`

	// Register-only
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// ForgotVerifyOTPResponse is returned when purpose = "forgot-password".
type ForgotVerifyOTPResponse struct {
	ResetToken string `json:"reset_token"`
}

// ResetPasswordRequest for POST /auth/reset-password (separate endpoint).
type ResetPasswordRequest struct {
	ResetToken  string `json:"reset_token"  validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}
