package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/pkg/kafka"
	"be-modami-auth-service/pkg/kafka/events"

	"github.com/Nerzal/gocloak/v13"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type AuthKeycloakUseCase struct {
	client   *gocloak.GoCloak
	cfg      KeycloakConfig
	logger   logging.Logger
	admin    *KeycloakUseCase
	producer kafka.Producer
}

func NewAuthKeycloakUseCase(cfg KeycloakConfig, admin *KeycloakUseCase, logger logging.Logger, producer kafka.Producer) *AuthKeycloakUseCase {
	return &AuthKeycloakUseCase{
		client:   gocloak.NewClient(cfg.BaseURL),
		cfg:      cfg,
		logger:   logger,
		admin:    admin,
		producer: producer,
	}
}

func (uc *AuthKeycloakUseCase) Login(ctx context.Context, req entity.LoginRequest) (*entity.LoginResponse, error) {
	token, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, req.Username, req.Password)
	if err != nil {
		uc.logger.Debug("login failed", logging.String("username", req.Username), logging.Any("error", err.Error()))
		return nil, entity.NewAppError(401, "invalid credentials", err)
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
		return nil, entity.NewAppError(502, "keycloak unavailable", err)
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
		return nil, entity.NewAppError(409, "user already exists or invalid data", err)
	}

	// Set password
	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, req.Password, false); err != nil {
		uc.logger.Error("failed to set password", err, logging.String("user_id", userID))
		return nil, entity.NewAppError(500, "failed to set password", err)
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
		return entity.NewAppError(400, "logout failed", err)
	}
	return nil
}

func (uc *AuthKeycloakUseCase) RefreshToken(ctx context.Context, req entity.RefreshRequest) (*entity.LoginResponse, error) {
	token, err := uc.client.RefreshToken(ctx, req.RefreshToken, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm)
	if err != nil {
		uc.logger.Debug("refresh token failed", logging.Any("error", err.Error()))
		return nil, entity.NewAppError(401, "invalid refresh token", err)
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
		return entity.NewAppError(502, "keycloak unavailable", err)
	}

	// Find user by email
	exact := true
	users, err := uc.client.GetUsers(ctx, adminToken, uc.cfg.Realm, gocloak.GetUsersParams{
		Email: &req.Email,
		Exact: &exact,
	})
	if err != nil {
		return entity.NewAppError(502, "failed to find user", err)
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

func (uc *AuthKeycloakUseCase) SocialLoginURL(provider string) (*entity.SocialLoginResponse, error) {
	if !allowedProviders[provider] {
		return nil, entity.NewAppError(400, "unsupported provider: "+provider, nil)
	}

	authURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid&kc_idp_hint=%s",
		uc.cfg.BaseURL,
		uc.cfg.Realm,
		url.QueryEscape(uc.cfg.ClientID),
		url.QueryEscape(uc.cfg.RedirectURL),
		url.QueryEscape(provider),
	)

	return &entity.SocialLoginResponse{AuthURL: authURL}, nil
}

func (uc *AuthKeycloakUseCase) ExchangeCode(ctx context.Context, code string) (*entity.LoginResponse, error) {
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
		return nil, entity.NewAppError(502, "failed to contact keycloak", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, entity.NewAppError(502, "failed to read keycloak response", err)
	}

	if resp.StatusCode != http.StatusOK {
		uc.logger.Debug("code exchange failed", logging.Int("status", resp.StatusCode), logging.String("body", string(body)))
		return nil, entity.NewAppError(401, "invalid or expired authorization code", nil)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, entity.NewAppError(502, "failed to parse keycloak response", err)
	}

	return &entity.LoginResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		TokenType:    tokenResp.TokenType,
	}, nil
}

func (uc *AuthKeycloakUseCase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	_, err := uc.client.Login(ctx, uc.cfg.ClientID, uc.cfg.ClientSecret, uc.cfg.Realm, userID, oldPassword)
	if err != nil {
		return entity.NewAppError(401, "old password is incorrect", err)
	}

	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return entity.NewAppError(502, "keycloak unavailable", err)
	}

	if err := uc.client.SetPassword(ctx, adminToken, userID, uc.cfg.Realm, newPassword, false); err != nil {
		uc.logger.Error("failed to set new password", err, logging.String("user_id", userID))
		return entity.NewAppError(500, "failed to change password", err)
	}

	return nil
}

func (uc *AuthKeycloakUseCase) UpdateProfile(ctx context.Context, userID string, req entity.UpdateProfileRequest) error {
	adminToken, err := uc.admin.getAdminToken(ctx)
	if err != nil {
		return entity.NewAppError(502, "keycloak unavailable", err)
	}

	user, err := uc.client.GetUserByID(ctx, adminToken, uc.cfg.Realm, userID)
	if err != nil {
		return entity.NewAppError(404, "user not found", err)
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
		return entity.NewAppError(500, "failed to update profile", err)
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
