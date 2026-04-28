package main

import (
	"context"
	"fmt"
	"strconv"

	"be-modami-auth-service/config"
	"be-modami-auth-service/internal/delivery/http/handler"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/auth"
	"be-modami-auth-service/pkg/db"
	"be-modami-auth-service/pkg/email"
	"github.com/jackc/pgx/v5/pgxpool"
	pkgkafka "gitlab.com/lifegoeson-libs/pkg-gokit/kafka"
	pkgredis "gitlab.com/lifegoeson-libs/pkg-gokit/redis"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type connections struct {
	dbPool         *pgxpool.Pool
	keycloakUC     *usecase.KeycloakUseCase
	authKeycloakUC *usecase.AuthKeycloakUseCase
	tokenVerifier  usecase.TokenVerifier
	kafkaService   *pkgkafka.KafkaService
	cacheAdapter   pkgredis.CachePort
	otpUseCase     usecase.OTPUseCase
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

		if err := db.RunMigrations(pool); err != nil {
			return nil, fmt.Errorf("run migrations: %w", err)
		}
		logger.Info("database migrations applied")
	} else {
		logger.Warn("postgres host not set, skipping database")
	}

	// Redis
	var cacheService pkgredis.CachePort
	if cfg.Redis.Host != "" {
		redisCfg := pkgredis.Config{
			Addrs:    []string{fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)},
			Password: cfg.Redis.Pass,
			DB:       cfg.Redis.Database,
			PoolSize: cfg.Redis.PoolSize,
		}
		adapter, err := pkgredis.NewAdapter(redisCfg)
		if err != nil {
			return nil, fmt.Errorf("redis connect: %w", err)
		}
		conn.cacheAdapter = adapter
		cacheService = adapter
		health.AddCheck(func(ctx context.Context) error {
			return adapter.Ping(ctx)
		})
		logger.Info("Redis connected", logging.String("addr", fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)))
	} else {
		logger.Warn("redis host not set, OTP features will be disabled")
	}

	// Kafka (optional)
	var kafkaProducer pkgkafka.Producer
	if cfg.Kafka.Enable && len(cfg.Kafka.GetBrokers()) > 0 {
		kafkaCfg := pkgkafka.Config{
			Brokers:          cfg.Kafka.GetBrokers(),
			ClientID:         cfg.Kafka.ClientID,
			ProducerOnlyMode: true,
		}
		kafkaSvc, err := pkgkafka.NewKafkaService(&kafkaCfg)
		if err != nil {
			logger.Warn("failed to initialize Kafka, events will be disabled", logging.Any("error", err.Error()))
		} else {
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
		BaseURL:             cfg.Keycloak.BaseURL,
		Realm:               cfg.Keycloak.Realm,
		ClientID:            cfg.Keycloak.ClientID,
		ClientSecret:        cfg.Keycloak.ClientSecret,
		AdminUser:           cfg.Keycloak.AdminUser,
		AdminPass:           cfg.Keycloak.AdminPass,
		RedirectURL:         cfg.Keycloak.RedirectURL,
		FrontendCallbackURL: cfg.Keycloak.FrontendCallbackURL,
	}
	conn.keycloakUC = usecase.NewKeycloakUseCase(keycloakCfg, logger)
	conn.authKeycloakUC = usecase.NewAuthKeycloakUseCase(keycloakCfg, conn.keycloakUC, logger, kafkaProducer, cacheService)

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

	// OTP (requires Redis + Email config)
	if cacheService != nil && cfg.Email.SMTP.Host != "" {
		otpService := auth.NewOTPService(cacheService)
		resetTokenService := auth.NewResetTokenService(cacheService)
		emailService := email.NewEmailService(&email.EmailConfig{
			SMTPHost:     cfg.Email.SMTP.Host,
			SMTPPort:     strconv.Itoa(cfg.Email.SMTP.Port),
			SMTPUsername: cfg.Email.SMTP.Username,
			SMTPPassword: cfg.Email.SMTP.Password,
			FromEmail:    cfg.Email.SMTP.FromEmail,
			FromName:     cfg.Email.SMTP.FromName,
		}, ctx)

		conn.otpUseCase = usecase.NewOTPUseCase(
			otpService,
			resetTokenService,
			emailService,
			conn.authKeycloakUC,
			cacheService,
		)
		logger.Info("OTP service initialized")
	} else {
		logger.Warn("OTP disabled (requires Redis + Email config)")
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
	if c.cacheAdapter != nil {
		c.cacheAdapter.Close()
	}
}
