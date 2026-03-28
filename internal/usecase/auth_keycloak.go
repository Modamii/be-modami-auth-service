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
	"be-modami-auth-service/pkg/kafka"
	"be-modami-auth-service/pkg/kafka/events"

	"github.com/Nerzal/gocloak/v13"
	"github.com/google/uuid"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type AuthKeycloakUseCase struct {
	client   *gocloak.GoCloak
	cfg      KeycloakConfig
	logger   logging.Logger
	admin    *KeycloakUseCase
	producer kafka.Producer
	cache    CacheService
}

// CacheService is an optional dependency for state management (social login CSRF).
type CacheService interface {
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, key string) error
}

func NewAuthKeycloakUseCase(cfg KeycloakConfig, admin *KeycloakUseCase, logger logging.Logger, producer kafka.Producer, cache CacheService) *AuthKeycloakUseCase {
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
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid credentials").WithError(err)
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
		return nil, apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
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
		return nil, apperror.New(apperror.CodeConflict, "user already exists or invalid data").WithError(err)
	}

	// Set password
	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, req.Password, false); err != nil {
		uc.logger.Error("failed to set password", err, logging.String("user_id", userID))
		return nil, apperror.New(apperror.CodeInternal, "failed to set password").WithError(err)
	}

	if uc.producer != nil {
		topics := kafka.GetKafkaTopics()
		payload := events.NewUserCreatedPayload(userID, req.Email, req.Username, req.FirstName, req.LastName)
		uc.producer.EmitAsync(ctx, topics.User.Created, &kafka.ProducerMessage{
			Key:   userID,
			Value: payload,
		})
	}

	return &entity.RegisterResponse{UserID: userID}, nil
}

func (uc *AuthKeycloakUseCase) Logout(ctx context.Context, req entity.LogoutRequest) error {
	err := uc.client.Logout(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, req.RefreshToken)
	if err != nil {
		uc.logger.Debug("logout failed", logging.Any("error", err.Error()))
		return apperror.New(apperror.CodeBadRequest, "logout failed").WithError(err)
	}
	return nil
}

func (uc *AuthKeycloakUseCase) RefreshToken(ctx context.Context, req entity.RefreshRequest) (*entity.LoginResponse, error) {
	token, err := uc.client.RefreshToken(ctx, req.RefreshToken, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm)
	if err != nil {
		uc.logger.Debug("refresh token failed", logging.Any("error", err.Error()))
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid refresh token").WithError(err)
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
		return apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
	}

	// Find user by email
	exact := true
	users, err := uc.client.GetUsers(ctx, adminToken, uc.cfg.Realm, gocloak.GetUsersParams{
		Email: &req.Email,
		Exact: &exact,
	})
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "failed to find user").WithError(err)
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
		return nil, apperror.New(apperror.CodeBadRequest, "unsupported provider: "+provider)
	}

	// Generate CSRF state and store in Redis
	state := uuid.NewString()
	stateData := map[string]string{
		"provider": provider,
	}
	if uc.cache != nil {
		if err := uc.cache.SetJSON(ctx, "social:state:"+state, stateData, 10*time.Minute); err != nil {
			uc.logger.Error("failed to store social login state", err)
			return nil, apperror.New(apperror.CodeInternal, "failed to initiate social login").WithError(err)
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
		return nil, apperror.New(apperror.CodeBadRequest, "missing state parameter")
	}
	var stateData map[string]string
	if uc.cache != nil {
		if err := uc.cache.GetJSON(ctx, "social:state:"+state, &stateData); err != nil {
			uc.logger.Debug("invalid or expired social login state", logging.String("state", state))
			return nil, apperror.New(apperror.CodeBadRequest, "invalid or expired state parameter")
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
		return nil, apperror.New(apperror.CodeBadGateway, "failed to contact keycloak").WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "failed to read keycloak response").WithError(err)
	}

	if resp.StatusCode != http.StatusOK {
		uc.logger.Debug("code exchange failed", logging.Int("status", resp.StatusCode), logging.String("body", string(body)))
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid or expired authorization code")
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "failed to parse keycloak response").WithError(err)
	}

	// Emit Kafka social login event
	if uc.producer != nil && stateData != nil {
		provider := stateData["provider"]
		topics := kafka.GetKafkaTopics()
		payload := events.NewSocialLoginPayload(provider, "")
		uc.producer.EmitAsync(ctx, topics.Auth.SocialLogin, &kafka.ProducerMessage{
			Key:   state,
			Value: payload,
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
		return "", apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
	}
	exact := true
	users, err := uc.client.GetUsers(ctx, adminToken, uc.cfg.Realm, gocloak.GetUsersParams{
		Email: &email,
		Exact: &exact,
	})
	if err != nil {
		return "", apperror.New(apperror.CodeBadGateway, "failed to find user").WithError(err)
	}
	if len(users) == 0 {
		return "", apperror.New(apperror.CodeNotFound, "user not found")
	}
	return derefStr(users[0].ID), nil
}

// RegisterWithVerifiedEmail creates a user in Keycloak with emailVerified=true,
// sets the password, emits a Kafka event, and returns OAuth2 tokens.
func (uc *AuthKeycloakUseCase) RegisterWithVerifiedEmail(ctx context.Context, req entity.RegisterRequest) (*entity.LoginResponse, error) {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return nil, apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
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
		return nil, apperror.New(apperror.CodeConflict, "user already exists or invalid data").WithError(err)
	}

	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, req.Password, false); err != nil {
		uc.logger.Error("failed to set password", err, logging.String("user_id", userID))
		return nil, apperror.New(apperror.CodeInternal, "failed to set password").WithError(err)
	}

	if uc.producer != nil {
		topics := kafka.GetKafkaTopics()
		payload := events.NewUserCreatedPayload(userID, req.Email, req.Username, req.FirstName, req.LastName)
		uc.producer.EmitAsync(ctx, topics.User.Created, &kafka.ProducerMessage{
			Key:   userID,
			Value: payload,
		})
	}

	// Issue token via login
	token, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, req.Username, req.Password)
	if err != nil {
		uc.logger.Error("failed to login after registration", err)
		return nil, apperror.New(apperror.CodeInternal, "registration succeeded but login failed").WithError(err)
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
		return apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
	}
	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, newPassword, false); err != nil {
		uc.logger.Error("failed to reset password", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "failed to reset password").WithError(err)
	}
	return nil
}

// UpdateUserEmail updates a user's email in Keycloak and marks it as verified.
func (uc *AuthKeycloakUseCase) UpdateUserEmail(ctx context.Context, userID, newEmail string) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
	}
	user, err := uc.client.GetUserByID(ctx, adminToken, uc.cfg.Realm, userID)
	if err != nil {
		return apperror.New(apperror.CodeNotFound, "user not found").WithError(err)
	}
	emailVerified := true
	user.Email = &newEmail
	user.Username = &newEmail
	user.EmailVerified = &emailVerified
	if err := uc.client.UpdateUser(ctx, adminToken, uc.cfg.Realm, *user); err != nil {
		uc.logger.Error("failed to update user email", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "failed to update email").WithError(err)
	}

	if uc.producer != nil {
		topics := kafka.GetKafkaTopics()
		payload := events.NewUserUpdatedPayload(userID,
			newEmail,
			derefStr(user.FirstName),
			derefStr(user.LastName),
		)
		uc.producer.EmitAsync(ctx, topics.User.Updated, &kafka.ProducerMessage{
			Key:   userID,
			Value: payload,
		})
	}

	return nil
}

func (uc *AuthKeycloakUseCase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	_, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, userID, oldPassword)
	if err != nil {
		return apperror.New(apperror.CodeUnauthorized, "old password is incorrect").WithError(err)
	}

	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
	}

	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, newPassword, false); err != nil {
		uc.logger.Error("failed to set new password", err, logging.String("user_id", userID))
		return apperror.New(apperror.CodeInternal, "failed to change password").WithError(err)
	}

	return nil
}

func (uc *AuthKeycloakUseCase) UpdateProfile(ctx context.Context, userID string, req entity.UpdateProfileRequest) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return apperror.New(apperror.CodeBadGateway, "keycloak unavailable").WithError(err)
	}

	user, err := uc.client.GetUserByID(ctx, adminToken, uc.cfg.Realm, userID)
	if err != nil {
		return apperror.New(apperror.CodeNotFound, "user not found").WithError(err)
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
		return apperror.New(apperror.CodeInternal, "failed to update profile").WithError(err)
	}

	if uc.producer != nil {
		topics := kafka.GetKafkaTopics()
		payload := events.NewUserUpdatedPayload(userID,
			derefStr(user.Email),
			derefStr(user.FirstName),
			derefStr(user.LastName),
		)
		uc.producer.EmitAsync(ctx, topics.User.Updated, &kafka.ProducerMessage{
			Key:   userID,
			Value: payload,
		})
	}

	return nil
}
