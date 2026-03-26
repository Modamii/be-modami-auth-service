package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"be-modami-auth-service/internal/entity"

	"github.com/Nerzal/gocloak/v13"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type KeycloakConfig struct {
	BaseURL      string
	Realm        string
	ClientID     string
	ClientSecret string
	AdminUser    string
	AdminPass    string
	RedirectURL  string
}

type KeycloakUseCase struct {
	client *gocloak.GoCloak
	cfg    KeycloakConfig
	logger logging.Logger

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

func NewKeycloakUseCase(cfg KeycloakConfig, logger logging.Logger) *KeycloakUseCase {
	client := gocloak.NewClient(cfg.BaseURL)
	return &KeycloakUseCase{
		client: client,
		cfg:    cfg,
		logger: logger,
	}
}

func (uc *KeycloakUseCase) getAdminToken(ctx context.Context) (string, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if uc.cachedToken != "" && time.Now().Before(uc.tokenExpiry.Add(-30*time.Second)) {
		return uc.cachedToken, nil
	}

	token, err := uc.client.LoginAdmin(ctx, uc.cfg.AdminUser, uc.cfg.AdminPass, "master")
	if err != nil {
		return "", fmt.Errorf("keycloak admin login: %w", err)
	}

	uc.cachedToken = token.AccessToken
	uc.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return uc.cachedToken, nil
}

func (uc *KeycloakUseCase) GetUsers(ctx context.Context, first, max int) ([]*entity.User, error) {
	token, err := uc.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	users, err := uc.client.GetUsers(ctx, token, uc.cfg.Realm, gocloak.GetUsersParams{
		First: &first,
		Max:   &max,
	})
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	result := make([]*entity.User, 0, len(users))
	for _, u := range users {
		result = append(result, mapGocloakUser(u))
	}
	return result, nil
}

func (uc *KeycloakUseCase) GetUserByID(ctx context.Context, userID string) (*entity.User, error) {
	token, err := uc.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	u, err := uc.client.GetUserByID(ctx, token, uc.cfg.Realm, userID)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return mapGocloakUser(u), nil
}

func (uc *KeycloakUseCase) GetRealmRoles(ctx context.Context) ([]*gocloak.Role, error) {
	token, err := uc.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	roles, err := uc.client.GetRealmRoles(ctx, token, uc.cfg.Realm, gocloak.GetRoleParams{})
	if err != nil {
		return nil, fmt.Errorf("get realm roles: %w", err)
	}
	return roles, nil
}

func (uc *KeycloakUseCase) GetUserRealmRoles(ctx context.Context, userID string) ([]*gocloak.Role, error) {
	token, err := uc.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	roles, err := uc.client.GetRealmRolesByUserID(ctx, token, uc.cfg.Realm, userID)
	if err != nil {
		return nil, fmt.Errorf("get user realm roles: %w", err)
	}
	return roles, nil
}

func (uc *KeycloakUseCase) AssignRealmRoles(ctx context.Context, userID string, roles []gocloak.Role) error {
	token, err := uc.getAdminToken(ctx)
	if err != nil {
		return err
	}

	if err := uc.client.AddRealmRoleToUser(ctx, token, uc.cfg.Realm, userID, roles); err != nil {
		return fmt.Errorf("assign realm roles: %w", err)
	}
	return nil
}

func (uc *KeycloakUseCase) RemoveRealmRoles(ctx context.Context, userID string, roles []gocloak.Role) error {
	token, err := uc.getAdminToken(ctx)
	if err != nil {
		return err
	}

	if err := uc.client.DeleteRealmRoleFromUser(ctx, token, uc.cfg.Realm, userID, roles); err != nil {
		return fmt.Errorf("remove realm roles: %w", err)
	}
	return nil
}

func (uc *KeycloakUseCase) Ping(ctx context.Context) error {
	_, err := uc.getAdminToken(ctx)
	return err
}

func mapGocloakUser(u *gocloak.User) *entity.User {
	return &entity.User{
		ID:                derefStr(u.ID),
		Email:             derefStr(u.Email),
		PreferredUsername: derefStr(u.Username),
		FirstName:         derefStr(u.FirstName),
		LastName:          derefStr(u.LastName),
		Enabled:           derefBool(u.Enabled),
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
