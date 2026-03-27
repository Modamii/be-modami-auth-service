package main

import (
	"context"

	"be-modami-auth-service/config"
	"be-modami-auth-service/internal/delivery/http/handler"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/db"
	"be-modami-auth-service/pkg/kafka"

	"github.com/jackc/pgx/v5/pgxpool"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type connections struct {
	dbPool         *pgxpool.Pool
	keycloakUC     *usecase.KeycloakUseCase
	authKeycloakUC *usecase.AuthKeycloakUseCase
	tokenVerifier  usecase.TokenVerifier
	kafkaService   *kafka.KafkaService
}

func initConnections(ctx context.Context, cfg *config.Config, health *handler.Health, logger logging.Logger) (*connections, error) {
	conn := &connections{}

	// Database (optional)
	if cfg.Postgres.Host != "" {
		pool, err := db.NewPool(ctx, cfg.Postgres.WriterURL(), cfg.Postgres.MaxActiveConns, cfg.Postgres.MaxIdleConns)
		if err != nil {
			return nil, err
		}
		conn.dbPool = pool
		health.AddCheck(func(ctx context.Context) error {
			return pool.Ping(ctx)
		})
		logger.Info("database connected")
	} else {
		logger.Warn("postgres host not set, skipping database")
	}

	// Kafka (optional)
	var kafkaProducer kafka.Producer
	if cfg.Kafka.Enable && len(cfg.Kafka.GetBrokers()) > 0 {
		kafkaSvc, err := kafka.NewKafkaService(nil, cfg)
		if err != nil {
			logger.Warn("failed to initialize Kafka, events will be disabled", logging.Any("error", err.Error()))
		} else {
			if err := kafkaSvc.EnsureTopics(ctx); err != nil {
				logger.Warn("failed to ensure Kafka topics", logging.Any("error", err.Error()))
			}
			conn.kafkaService = kafkaSvc
			kafkaProducer = kafkaSvc
			health.AddCheck(func(ctx context.Context) error {
				return kafkaSvc.Ping(ctx)
			})
			logger.Info("Kafka connected", logging.Any("brokers", cfg.Kafka.GetBrokers()))
		}
	} else {
		logger.Warn("Kafka brokers not configured, events will be disabled")
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
	conn.authKeycloakUC = usecase.NewAuthKeycloakUseCase(keycloakCfg, conn.keycloakUC, logger, kafkaProducer)

	// OIDC token verifier (optional)
	if cfg.Keycloak.BaseURL != "" && cfg.Keycloak.Realm != "" {
		issuerURL := cfg.Keycloak.BaseURL + "/realms/" + cfg.Keycloak.Realm
		uc, err := usecase.NewAuthUseCase(ctx, issuerURL, cfg.Keycloak.ClientID, logger)
		if err != nil {
			logger.Warn("OIDC provider not available, token verification disabled", logging.Any("error", err.Error()))
		} else {
			conn.tokenVerifier = uc
			health.AddCheck(func(ctx context.Context) error {
				return conn.keycloakUC.Ping(ctx)
			})
			logger.Info("OIDC provider initialized", logging.String("issuer", issuerURL))
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
	if c.kafkaService != nil {
		c.kafkaService.Close()
	}
}
