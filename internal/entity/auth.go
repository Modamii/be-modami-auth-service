package entity

type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"john"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type RegisterRequest struct {
	Username  string `json:"username" binding:"required" example:"john"`
	Email     string `json:"email" binding:"required,email" example:"john@example.com"`
	Password  string `json:"password" binding:"required,min=8" example:"secret123"`
	FirstName string `json:"first_name" example:"John"`
	LastName  string `json:"last_name" example:"Doe"`
}

type RegisterResponse struct {
	UserID string `json:"user_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email" example:"john@example.com"`
}

type SocialLoginResponse struct {
	AuthURL string `json:"auth_url"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"oldSecret123"`
	NewPassword string `json:"new_password" binding:"required,min=8" example:"newSecret123"`
}

type UpdateProfileRequest struct {
	FirstName string `json:"first_name" example:"John"`
	LastName  string `json:"last_name" example:"Doe"`
	Email     string `json:"email" binding:"omitempty,email" example:"john@example.com"`
}
