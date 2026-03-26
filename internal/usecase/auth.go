package usecase

import (
	"context"
	"fmt"
	"strings"

	"be-modami-auth-service/internal/entity"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
)

type TokenVerifier interface {
	VerifyToken(ctx context.Context, rawToken string) (*entity.KeycloakClaims, error)
}

type AuthUseCase struct {
	verifier *oidc.IDTokenVerifier
	logger   *zap.Logger
}

func NewAuthUseCase(ctx context.Context, issuerURL, clientID string, logger *zap.Logger) (*AuthUseCase, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}

	cfg := &oidc.Config{
		ClientID:                       clientID,
		SkipClientIDCheck:              true,
		InsecureSkipSignatureCheck:     false,
	}

	return &AuthUseCase{
		verifier: provider.Verifier(cfg),
		logger:   logger,
	}, nil
}

func (uc *AuthUseCase) VerifyToken(ctx context.Context, rawToken string) (*entity.KeycloakClaims, error) {
	idToken, err := uc.verifier.Verify(ctx, rawToken)
	if err != nil {
		uc.logger.Debug("token verification failed", zap.Error(err))
		return nil, entity.ErrUnauthorized
	}

	var claims entity.KeycloakClaims
	if err := idToken.Claims(&claims); err != nil {
		uc.logger.Error("failed to parse token claims", zap.Error(err))
		return nil, entity.ErrUnauthorized
	}

	claims.Sub = idToken.Subject
	return &claims, nil
}

func ExtractBearerToken(authHeader string) (string, error) {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", entity.ErrUnauthorized
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", entity.ErrUnauthorized
	}
	return token, nil
}
