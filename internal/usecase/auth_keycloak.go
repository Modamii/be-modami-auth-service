package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/pkg/events"

	"github.com/Nerzal/gocloak/v13"
	"github.com/google/uuid"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
	pkgkafka "gitlab.com/lifegoeson-libs/pkg-gokit/kafka"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type AuthKeycloakUseCase struct {
	client   *gocloak.GoCloak
	cfg      KeycloakConfig
	logger   logging.Logger
	admin    *KeycloakUseCase
	producer pkgkafka.Producer
	cache    CacheService
}

// CacheService is an optional dependency for state management (social login CSRF).
type CacheService interface {
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, keys ...string) error
}

func NewAuthKeycloakUseCase(cfg KeycloakConfig, admin *KeycloakUseCase, logger logging.Logger, producer pkgkafka.Producer, cache CacheService) *AuthKeycloakUseCase {
	return &AuthKeycloakUseCase{
		client:   gocloak.NewClient(cfg.BaseURL),
		cfg:      cfg,
		logger:   logger,
		admin:    admin,
		producer: producer,
		cache:    cache,
	}
}

func (uc *AuthKeycloakUseCase) Login(ctx context.Context, req entity.LoginRequest) (*entity.LoginResponse, error) {
	token, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, req.Username, req.Password)
	if err != nil {
		uc.logger.Debug("login failed", logging.String("username", req.Username), logging.Any("error", err.Error()))
		return nil, apperror.New(apperror.CodeUnauthorized, "thông tin đăng nhập không hợp lệ").WithError(err)
	}

	return &entity.LoginResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    token.ExpiresIn,
		TokenType:    token.TokenType,
	}, nil
}

func (uc *AuthKeycloakUseCase) Register(ctx context.Context, req entity.RegisterRequest) (*entity.RegisterResponse, error) {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}

	enabled := true
	user := gocloak.User{
		Username:  &req.Username,
		Email:     &req.Email,
		FirstName: &req.FirstName,
		LastName:  &req.LastName,
		Enabled:   &enabled,
	}

	userID, err := uc.client.CreateUser(ctx, adminToken, uc.cfg.Realm, user)
	if err != nil {
		uc.logger.Error("failed to create user", err)
		return nil, apperror.New(apperror.CodeConflict, "người dùng đã tồn tại hoặc dữ liệu không hợp lệ").WithError(err)
	}

	// Set password
	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, req.Password, false); err != nil {
		uc.logger.Error("không thể đặt mật khẩu", err, logging.String("user_id", userID))
		return nil, apperror.New(apperror.CodeInternal, "không thể đặt mật khẩu").WithError(err)
	}

	if uc.producer != nil {
		uc.producer.EmitAsync(ctx, events.TopicUserCreated, &pkgkafka.ProducerMessage{
			Key:   userID,
			Value: events.NewUserCreatedPayload(userID, req.Email, req.Username, req.FirstName, req.LastName),
		})
	}

	return &entity.RegisterResponse{UserID: userID}, nil
}

func (uc *AuthKeycloakUseCase) Logout(ctx context.Context, req entity.LogoutRequest) error {
	err := uc.client.Logout(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, req.RefreshToken)
	if err != nil {
		uc.logger.Debug("logout failed", logging.Any("error", err.Error()))
		return apperror.New(apperror.CodeBadRequest, "đăng xuất thất bại").WithError(err)
	}
	return nil
}

func (uc *AuthKeycloakUseCase) RefreshToken(ctx context.Context, req entity.RefreshRequest) (*entity.LoginResponse, error) {
	token, err := uc.client.RefreshToken(ctx, req.RefreshToken, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm)
	if err != nil {
		uc.logger.Debug("refresh token failed", logging.Any("error", err.Error()))
		return nil, apperror.New(apperror.CodeUnauthorized, "refresh token không hợp lệ").WithError(err)
	}

	return &entity.LoginResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    token.ExpiresIn,
		TokenType:    token.TokenType,
	}, nil
}

func (uc *AuthKeycloakUseCase) ForgotPassword(ctx context.Context, req entity.ForgotPasswordRequest) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}

	// Find user by email
	exact := true
	users, err := uc.client.GetUsers(ctx, adminToken, uc.cfg.Realm, gocloak.GetUsersParams{
		Email: &req.Email,
		Exact: &exact,
	})
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "không thể tìm kiếm người dùng").WithError(err)
	}
	if len(users) == 0 {
		// Don't reveal whether email exists
		return nil
	}

	userID := derefStr(users[0].ID)
	actions := []string{"UPDATE_PASSWORD"}

	err = uc.client.ExecuteActionsEmail(ctx, adminToken, uc.cfg.Realm, gocloak.ExecuteActionsEmail{
		UserID:  &userID,
		Actions: &actions,
	})
	if err != nil {
		uc.logger.Error("failed to send reset password email", err)
		return fmt.Errorf("send reset email: %w", err)
	}

	return nil
}

var allowedProviders = map[string]bool{
	"google":   true,
	"facebook": true,
	"github":   true,
}

func (uc *AuthKeycloakUseCase) SocialLoginURL(ctx context.Context, provider string) (*entity.SocialLoginResponse, error) {
	if !allowedProviders[provider] {
		return nil, apperror.New(apperror.CodeBadRequest, "nhà cung cấp không được hỗ trợ: "+provider)
	}

	// Generate CSRF state and store in Redis
	state := uuid.NewString()
	stateData := map[string]string{
		"provider": provider,
	}
	if uc.cache != nil {
		if err := uc.cache.SetJSON(ctx, "social:state:"+state, stateData, 10*time.Minute); err != nil {
			uc.logger.Error("failed to store social login state", err)
			return nil, apperror.New(apperror.CodeInternal, "không thể khởi tạo đăng nhập mạng xã hội").WithError(err)
		}
	}

	authURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid&kc_idp_hint=%s&state=%s",
		uc.cfg.BaseURL,
		uc.cfg.Realm,
		url.QueryEscape(uc.cfg.ClientID),
		url.QueryEscape(uc.cfg.RedirectURL),
		url.QueryEscape(provider),
		url.QueryEscape(state),
	)

	return &entity.SocialLoginResponse{AuthURL: authURL}, nil
}

func (uc *AuthKeycloakUseCase) ExchangeCode(ctx context.Context, code, state string) (*entity.LoginResponse, error) {
	// Validate CSRF state
	if state == "" {
		return nil, apperror.New(apperror.CodeBadRequest, "thiếu tham số state")
	}
	var stateData map[string]string
	if uc.cache != nil {
		if err := uc.cache.GetJSON(ctx, "social:state:"+state, &stateData); err != nil {
			uc.logger.Debug("invalid or expired social login state", logging.String("state", state))
			return nil, apperror.New(apperror.CodeBadRequest, "tham số state không hợp lệ hoặc đã hết hạn")
		}
		// Delete state immediately (one-time use)
		_ = uc.cache.Delete(ctx, "social:state:"+state)
	}

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", uc.cfg.BaseURL, uc.cfg.Realm)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {uc.cfg.ClientID},
		"client_secret": {uc.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {uc.cfg.RedirectURL},
	}

	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		uc.logger.Error("failed to exchange code", err)
		return nil, apperror.New(apperror.CodeBadGateway, "không thể kết nối đến dịch vụ xác thực").WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "không thể đọc phản hồi từ dịch vụ xác thực").WithError(err)
	}

	if resp.StatusCode != http.StatusOK {
		uc.logger.Debug("code exchange failed", logging.Int("status", resp.StatusCode), logging.String("body", string(body)))
		return nil, apperror.New(apperror.CodeUnauthorized, "mã xác thực không hợp lệ hoặc đã hết hạn")
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "không thể phân tích phản hồi từ dịch vụ xác thực").WithError(err)
	}

	if uc.producer != nil && stateData != nil {
		uc.producer.EmitAsync(ctx, events.TopicSocialLogin, &pkgkafka.ProducerMessage{
			Key:   state,
			Value: events.NewSocialLoginPayload(stateData["provider"], ""),
		})
	}

	return &entity.LoginResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		TokenType:    tokenResp.TokenType,
	}, nil
}

// GetFrontendCallbackURL returns the configured frontend callback URL.
func (uc *AuthKeycloakUseCase) GetFrontendCallbackURL() string {
	return uc.cfg.FrontendCallbackURL
}

// FindUserByEmail returns the Keycloak user ID for the given email, or error if not found.
func (uc *AuthKeycloakUseCase) FindUserByEmail(ctx context.Context, email string) (string, error) {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return "", apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}
	exact := true
	users, err := uc.client.GetUsers(ctx, adminToken, uc.cfg.Realm, gocloak.GetUsersParams{
		Email: &email,
		Exact: &exact,
	})
	if err != nil {
		return "", apperror.New(apperror.CodeBadGateway, "không thể tìm kiếm người dùng").WithError(err)
	}
	if len(users) == 0 {
		return "", apperror.New(apperror.CodeNotFound, "không tìm thấy người dùng")
	}
	return derefStr(users[0].ID), nil
}

// RegisterWithVerifiedEmail creates a user in Keycloak with emailVerified=true,
// sets the password, emits a Kafka event, and returns OAuth2 tokens.
func (uc *AuthKeycloakUseCase) RegisterWithVerifiedEmail(ctx context.Context, req entity.RegisterRequest) (*entity.LoginResponse, error) {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}

	enabled := true
	emailVerified := true
	user := gocloak.User{
		Username:      &req.Username,
		Email:         &req.Email,
		FirstName:     &req.FirstName,
		LastName:      &req.LastName,
		Enabled:       &enabled,
		EmailVerified: &emailVerified,
	}

	userID, err := uc.client.CreateUser(ctx, adminToken, uc.cfg.Realm, user)
	if err != nil {
		uc.logger.Error("failed to create user", err)
		return nil, apperror.New(apperror.CodeConflict, "người dùng đã tồn tại hoặc dữ liệu không hợp lệ").WithError(err)
	}

	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, req.Password, false); err != nil {
		uc.logger.Error("không thể đặt mật khẩu", err, logging.String("user_id", userID))
		return nil, apperror.New(apperror.CodeInternal, "không thể đặt mật khẩu").WithError(err)
	}

	if uc.producer != nil {
		uc.producer.EmitAsync(ctx, events.TopicUserCreated, &pkgkafka.ProducerMessage{
			Key:   userID,
			Value: events.NewUserCreatedPayload(userID, req.Email, req.Username, req.FirstName, req.LastName),
		})
	}

	// Issue token via login
	token, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, req.Username, req.Password)
	if err != nil {
		uc.logger.Error("failed to login after registration", err)
		return nil, apperror.New(apperror.CodeInternal, "đăng ký thành công nhưng đăng nhập thất bại").WithError(err)
	}

	return &entity.LoginResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    token.ExpiresIn,
		TokenType:    token.TokenType,
	}, nil
}

// SetPasswordByUserID resets a user's password via admin token.
func (uc *AuthKeycloakUseCase) SetPasswordByUserID(ctx context.Context, userID, newPassword string) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}
	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, newPassword, false); err != nil {
		uc.logger.Error("không thể đặt lại mật khẩu", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "không thể đặt lại mật khẩu").WithError(err)
	}
	return nil
}

// UpdateUserEmail updates a user's email in Keycloak and marks it as verified.
func (uc *AuthKeycloakUseCase) UpdateUserEmail(ctx context.Context, userID, newEmail string) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}
	user, err := uc.client.GetUserByID(ctx, adminToken, uc.cfg.Realm, userID)
	if err != nil {
		return apperror.New(apperror.CodeNotFound, "không tìm thấy người dùng").WithError(err)
	}
	emailVerified := true
	user.Email = &newEmail
	user.Username = &newEmail
	user.EmailVerified = &emailVerified
	if err := uc.client.UpdateUser(ctx, adminToken, uc.cfg.Realm, *user); err != nil {
		uc.logger.Error("failed to update user email", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "không thể cập nhật email").WithError(err)
	}

	if uc.producer != nil {
		uc.producer.EmitAsync(ctx, events.TopicUserUpdated, &pkgkafka.ProducerMessage{
			Key:   userID,
			Value: events.NewUserUpdatedPayload(userID, newEmail, derefStr(user.FirstName), derefStr(user.LastName)),
		})
	}

	return nil
}

func (uc *AuthKeycloakUseCase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	_, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, userID, oldPassword)
	if err != nil {
		return apperror.New(apperror.CodeUnauthorized, "mật khẩu cũ không đúng").WithError(err)
	}

	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}

	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, newPassword, false); err != nil {
		uc.logger.Error("failed to set new password", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "không thể đổi mật khẩu").WithError(err)
	}

	return nil
}

func (uc *AuthKeycloakUseCase) UpdateProfile(ctx context.Context, userID string, req entity.UpdateProfileRequest) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "dịch vụ xác thực không khả dụng").WithError(err)
	}

	user, err := uc.client.GetUserByID(ctx, adminToken, uc.cfg.Realm, userID)
	if err != nil {
		return apperror.New(apperror.CodeNotFound, "không tìm thấy người dùng").WithError(err)
	}

	if req.FirstName != "" {
		user.FirstName = &req.FirstName
	}
	if req.LastName != "" {
		user.LastName = &req.LastName
	}
	if req.Email != "" {
		user.Email = &req.Email
	}

	if err := uc.client.UpdateUser(ctx, adminToken, uc.cfg.Realm, *user); err != nil {
		uc.logger.Error("failed to update user profile", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "không thể cập nhật thông tin cá nhân").WithError(err)
	}

	if uc.producer != nil {
		uc.producer.EmitAsync(ctx, events.TopicUserUpdated, &pkgkafka.ProducerMessage{
			Key:   userID,
			Value: events.NewUserUpdatedPayload(userID, derefStr(user.Email), derefStr(user.FirstName), derefStr(user.LastName)),
		})
	}

	return nil
}
