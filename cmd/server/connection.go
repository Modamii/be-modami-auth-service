package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"be-modami-auth-service/config"
	"be-modami-auth-service/internal/consumer"
	"be-modami-auth-service/internal/delivery/http/handler"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/migrations"
	"be-modami-auth-service/pkg/auth"
	"be-modami-auth-service/pkg/email"

	"github.com/jackc/pgx/v5/pgxpool"
	pkgkafka "gitlab.com/lifegoeson-libs/pkg-gokit/kafka"
	pkgpostgres "gitlab.com/lifegoeson-libs/pkg-gokit/postgresql"
	pkgredis "gitlab.com/lifegoeson-libs/pkg-gokit/redis"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type connections struct {
	dbPool         *pgxpool.Pool
	keycloakUC     *usecase.KeycloakUseCase
	authKeycloakUC *usecase.AuthKeycloakUseCase
	tokenVerifier  usecase.TokenVerifier
	otpUseCase     usecase.OTPUseCase
	kafkaService   *pkgkafka.KafkaService
	cacheAdapter   pkgredis.CachePort
	cdcConsumer    *consumer.UserCDCConsumer
}

func initConnections(ctx context.Context, cfg *config.Config, health *handler.Health, logger logging.Logger) (*connections, error) {
	conn := &connections{}

	// Database``
	pool, _, err := pkgpostgres.Connect(ctx, pkgpostgres.Config{
		Host:           cfg.Postgres.Host,
		Port:           cfg.Postgres.Port,
		User:           cfg.Postgres.UserWriter,
		Password:       cfg.Postgres.PasswordWriter,
		Database:       cfg.Postgres.Database,
		SSLMode:        cfg.Postgres.SSLMode,
		MaxConns:       cfg.Postgres.MaxActiveConns,
		MinConns:       cfg.Postgres.MaxIdleConns,
		ConnectTimeout: 30 * time.Second,
	})
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", err)
		return nil, err
	}
	conn.dbPool = pool
	health.AddCheck(func(ctx context.Context) error {
		return pool.Ping(ctx)
	})
	if err := migrations.RunMigrations(pool); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	logger.Info("database migrations applied")

	// Redis
	adapter, err := pkgredis.NewAdapter(pkgredis.Config{
		Addrs:       []string{fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)},
		Password:    cfg.Redis.Pass,
		DB:          cfg.Redis.Database,
		PoolSize:    cfg.Redis.PoolSize,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logger.Warn("failed to connect to Redis, OTP features will be disabled", logging.Any("error", err.Error()))
	} else {
		conn.cacheAdapter = adapter
		health.AddCheck(func(ctx context.Context) error {
			return adapter.Ping(ctx)
		})
		logger.Info("Redis connected", logging.String("addr", fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)))
	}

	// Kafka
	kafkaSvc, err := pkgkafka.NewKafkaService(&pkgkafka.Config{
		Brokers:          cfg.Kafka.GetBrokers(),
		ClientID:         cfg.Kafka.ClientID,
		ProducerOnlyMode: true,
	})
	if err != nil {
		logger.Warn("failed to initialize Kafka, events will be disabled", logging.Any("error", err.Error()))
	} else {
		conn.kafkaService = kafkaSvc
		health.AddCheck(func(ctx context.Context) error {
			return kafkaSvc.Ping(ctx)
		})
		logger.Info("Kafka connected", logging.Any("brokers", cfg.Kafka.GetBrokers()))
	}

	// CDC consumer — reads Debezium user_entity changes and re-emits app events.
	cdcConsumer, err := consumer.NewUserCDCConsumer(
		cfg.Kafka.GetBrokers(),
		cfg.Kafka.ConsumerGroupID+"-cdc",
		conn.kafkaService,
		cfg.App.Environment,
		logger,
	)
	if err != nil {
		logger.Warn("failed to initialize CDC consumer, CDC events will be disabled", logging.Any("error", err.Error()))
	} else {
		conn.cdcConsumer = cdcConsumer
		conn.cdcConsumer.Start()
		logger.Info("CDC consumer started", logging.String("group", cfg.Kafka.ConsumerGroupID+"-cdc"))
	}

	conn.keycloakUC = usecase.NewKeycloakUseCase(cfg, logger)
	conn.authKeycloakUC = usecase.NewAuthKeycloakUseCase(cfg, conn.keycloakUC, logger, conn.kafkaService, conn.cacheAdapter)

	// OIDC token verifier
	issuerURL := cfg.Keycloak.BaseURL + "/realms/" + cfg.Keycloak.Realm
	uc, err := usecase.NewAuthUseCase(ctx, issuerURL, cfg.Keycloak.ClientID, logger)
	if err != nil {
		return nil, fmt.Errorf("init OIDC provider: %w", err)
	}
	conn.tokenVerifier = uc
	health.AddCheck(func(ctx context.Context) error {
		return conn.keycloakUC.Ping(ctx)
	})
	logger.Info("OIDC provider initialized", logging.String("issuer", issuerURL))

	// OTP
	otpService := auth.NewOTPService(conn.cacheAdapter)
	resetTokenService := auth.NewResetTokenService(conn.cacheAdapter)
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
		conn.cacheAdapter,
	)
	logger.Info("OTP service initialized")

	return conn, nil
}

func (c *connections) Close() {
	if c.cdcConsumer != nil {
		c.cdcConsumer.Close()
	}
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
