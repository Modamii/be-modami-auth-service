package handler

import (
	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/ctxutil"
	"be-modami-auth-service/pkg/response"

	"github.com/gin-gonic/gin"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
)

type Auth struct {
	authUC *usecase.AuthKeycloakUseCase
}

func NewAuth(authUC *usecase.AuthKeycloakUseCase) *Auth {
	return &Auth{authUC: authUC}
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user with username and password, returns JWT tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body entity.LoginRequest true "Login credentials"
// @Success      200 {object} response.Response{data=entity.LoginResponse}
// @Failure      401 {object} response.Response
// @Router       /v1/auth-services/auth/login [post]
func (h *Auth) Login(c *gin.Context) {
	var req entity.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	resp, err := h.authUC.Login(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resp)
}

// Register godoc
// @Summary      Register new user
// @Description  Create a new user account in Keycloak
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body entity.RegisterRequest true "Registration data"
// @Success      201 {object} response.Response{data=entity.RegisterResponse}
// @Failure      400 {object} response.Response
// @Failure      409 {object} response.Response
// @Router       /v1/auth-services/auth/register [post]
func (h *Auth) Register(c *gin.Context) {
	var req entity.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	resp, err := h.authUC.Register(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resp)
}

// Logout godoc
// @Summary      Logout user
// @Description  Invalidate refresh token to end the session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body entity.LogoutRequest true "Refresh token"
// @Success      204
// @Failure      400 {object} response.Response
// @Router       /v1/auth-services/auth/logout [post]
func (h *Auth) Logout(c *gin.Context) {
	var req entity.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	if err := h.authUC.Logout(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// RefreshToken godoc
// @Summary      Refresh access token
// @Description  Exchange refresh token for a new access token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body entity.RefreshRequest true "Refresh token"
// @Success      200 {object} response.Response{data=entity.LoginResponse}
// @Failure      401 {object} response.Response
// @Router       /v1/auth-services/auth/refresh [post]
func (h *Auth) RefreshToken(c *gin.Context) {
	var req entity.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	resp, err := h.authUC.RefreshToken(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resp)
}

// ForgotPassword godoc
// @Summary      Forgot password
// @Description  Send a password reset email to the user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body entity.ForgotPasswordRequest true "User email"
// @Success      200 {object} response.Response
// @Failure      400 {object} response.Response
// @Router       /v1/auth-services/auth/forgot-password [post]
func (h *Auth) ForgotPassword(c *gin.Context) {
	var req entity.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	if err := h.authUC.ForgotPassword(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"message": "if the email exists, a reset link has been sent"})
}

// SocialLogin godoc
// @Summary      Social login
// @Description  Returns the Keycloak authorization URL for the given social provider
// @Tags         auth
// @Produce      json
// @Param        provider query string true "Social provider (google, facebook, github)"
// @Success      200 {object} response.Response{data=entity.SocialLoginResponse}
// @Failure      400 {object} response.Response
// @Router       /v1/auth-services/auth/social/login [get]
func (h *Auth) SocialLogin(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "provider query parameter is required"))
		return
	}

	resp, err := h.authUC.SocialLoginURL(provider)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resp)
}

// SocialCallback godoc
// @Summary      Social login callback
// @Description  Exchanges the authorization code from Keycloak for JWT tokens
// @Tags         auth
// @Produce      json
// @Param        code query string true "Authorization code"
// @Success      200 {object} response.Response{data=entity.LoginResponse}
// @Failure      401 {object} response.Response
// @Router       /v1/auth-services/auth/social/callback [get]
func (h *Auth) SocialCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "code query parameter is required"))
		return
	}

	resp, err := h.authUC.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resp)
}

// ChangePassword godoc
// @Summary      Change password
// @Description  Change the authenticated user's password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body entity.ChangePasswordRequest true "Password change data"
// @Success      204
// @Failure      401 {object} response.Response
// @Router       /v1/auth-services/auth/password [put]
func (h *Auth) ChangePassword(c *gin.Context) {
	claims, ok := ctxutil.GetClaims(c)
	if !ok {
		response.Error(c, apperror.ErrUnauthorized)
		return
	}

	var req entity.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	if err := h.authUC.ChangePassword(c.Request.Context(), claims.PreferredUsername, req.OldPassword, req.NewPassword); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}


func (h *Auth) UpdateProfile(c *gin.Context) {
	claims, ok := ctxutil.GetClaims(c)
	if !ok {
		response.Error(c, apperror.ErrUnauthorized)
		return
	}

	var req entity.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	if err := h.authUC.UpdateProfile(c.Request.Context(), claims.Sub, req); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}
