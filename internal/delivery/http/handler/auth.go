package handler

import (
	"fmt"
	"net/http"
	"net/url"

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
// @Router       /auth/login [post]
func (h *Auth) Login(c *gin.Context) {
	var req entity.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
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
// @Router       /auth/register [post]
func (h *Auth) Register(c *gin.Context) {
	var req entity.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
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
// @Router       /auth/logout [post]
func (h *Auth) Logout(c *gin.Context) {
	var req entity.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
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
// @Router       /auth/refresh [post]
func (h *Auth) RefreshToken(c *gin.Context) {
	var req entity.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
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
// @Router       /auth/forgot-password [post]
func (h *Auth) ForgotPassword(c *gin.Context) {
	var req entity.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
		return
	}

	if err := h.authUC.ForgotPassword(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"message": "nếu email tồn tại, liên kết đặt lại mật khẩu đã được gửi"})
}

// SocialLogin godoc
// @Summary      Social login
// @Description  Returns the Keycloak authorization URL for the given social provider
// @Tags         auth
// @Produce      json
// @Param        provider query string true "Social provider (google, facebook, github)"
// @Success      200 {object} response.Response{data=entity.SocialLoginResponse}
// @Failure      400 {object} response.Response
// @Router       /auth/social/login [get]
func (h *Auth) SocialLogin(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "thiếu tham số provider"))
		return
	}

	resp, err := h.authUC.SocialLoginURL(c.Request.Context(), provider)
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
// @Router       /auth/social/callback [get]
func (h *Auth) SocialCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "code query parameter is required"))
		return
	}
	state := c.Query("state")

	resp, err := h.authUC.ExchangeCode(c.Request.Context(), code, state)
	if err != nil {
		response.Error(c, err)
		return
	}

	// Redirect to frontend with tokens in URL fragment (not query params for security)
	frontendURL := h.authUC.GetFrontendCallbackURL()
	if frontendURL != "" {
		fragment := fmt.Sprintf("access_token=%s&refresh_token=%s&expires_in=%d&token_type=%s",
			url.QueryEscape(resp.AccessToken),
			url.QueryEscape(resp.RefreshToken),
			resp.ExpiresIn,
			url.QueryEscape(resp.TokenType),
		)
		redirectURL := frontendURL + "#" + fragment
		c.Redirect(http.StatusFound, redirectURL)
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
// @Router       /auth/password [put]
func (h *Auth) ChangePassword(c *gin.Context) {
	claims, ok := ctxutil.GetClaims(c)
	if !ok {
		response.Error(c, apperror.ErrUnauthorized)
		return
	}

	var req entity.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
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
		response.Error(c, apperror.New(apperror.CodeBadRequest, "dữ liệu yêu cầu không hợp lệ").WithError(err))
		return
	}

	if err := h.authUC.UpdateProfile(c.Request.Context(), claims.Sub, req); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}
