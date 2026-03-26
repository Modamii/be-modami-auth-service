package main

import (
	"context"

	"be-modami-auth-service/config"
	"be-modami-auth-service/internal/delivery/http/handler"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type connections struct {
	dbPool         *pgxpool.Pool
	keycloakUC     *usecase.KeycloakUseCase
	authKeycloakUC *usecase.AuthKeycloakUseCase
	tokenVerifier  usecase.TokenVerifier
}

func initConnections(ctx context.Context, cfg *config.Config, health *handler.Health, logger *zap.Logger) (*connections, error) {
	conn := &connections{}

	// Database (optional)
	if cfg.DB.URL != "" {
		pool, err := db.NewPool(ctx, cfg.DB.URL, cfg.DB.MaxConns, cfg.DB.MinConns)
		if err != nil {
			return nil, err
		}
		conn.dbPool = pool
		health.AddCheck(func(ctx context.Context) error {
			return pool.Ping(ctx)
		})
		logger.Info("database connected")
	} else {
		logger.Warn("DATABASE_URL not set, skipping database")
	}

	// Keycloak
	keycloakCfg := usecase.KeycloakConfig{
		BaseURL:      cfg.Keycloak.BaseURL,
		Realm:        cfg.Keycloak.Realm,
		ClientID:     cfg.Keycloak.ClientID,
		ClientSecret: cfg.Keycloak.ClientSecret,
		AdminUser:    cfg.Keycloak.AdminUser,
		AdminPass:    cfg.Keycloak.AdminPass,
		RedirectURL:  cfg.Keycloak.RedirectURL,
	}
	conn.keycloakUC = usecase.NewKeycloakUseCase(keycloakCfg, logger)
	conn.authKeycloakUC = usecase.NewAuthKeycloakUseCase(keycloakCfg, conn.keycloakUC, logger)

	// OIDC token verifier (non-fatal — allows app to start without Keycloak)
	if cfg.Keycloak.BaseURL != "" && cfg.Keycloak.Realm != "" {
		issuerURL := cfg.Keycloak.BaseURL + "/realms/" + cfg.Keycloak.Realm
		uc, err := usecase.NewAuthUseCase(ctx, issuerURL, cfg.Keycloak.ClientID, logger)
		if err != nil {
			logger.Warn("OIDC provider not available, token verification disabled", zap.Error(err))
		} else {
			conn.tokenVerifier = uc
			health.AddCheck(func(ctx context.Context) error {
				return conn.keycloakUC.Ping(ctx)
			})
			logger.Info("OIDC provider initialized", zap.String("issuer", issuerURL))
		}
	} else {
		logger.Warn("Keycloak not configured, OIDC middleware disabled")
	}

	return conn, nil
}

func (c *connections) Close() {
	if c.dbPool != nil {
		c.dbPool.Close()
	}
}
