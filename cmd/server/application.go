package main

import (
	"context"
	"time"

	"be-modami-auth-service/config"
	deliveryhttp "be-modami-auth-service/internal/delivery/http"
	"be-modami-auth-service/internal/delivery/http/handler"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

type application struct {
	router *gin.Engine
	conn   *connections
	logger logging.Logger
}

func newApplication(cfg *config.Config) (*application, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	l := logger.L()

	health := handler.NewHealth()

	conn, err := initConnections(ctx, cfg, health, l)
	if err != nil {
		logger.FromContext(ctx).Error("failed to initialize connections", err)
		return nil, err
	}

	// Handlers
	authHandler := handler.NewAuth(conn.authKeycloakUC)
	userHandler := handler.NewUser(conn.keycloakUC)
	roleHandler := handler.NewRole(conn.keycloakUC)
	otpHandler := handler.NewOTPHandler(conn.otpUseCase, validator.New())

	// Router
	r := deliveryhttp.NewRouter(deliveryhttp.RouterDeps{
		Health:           health,
		Auth:             authHandler,
		User:             userHandler,
		Role:             roleHandler,
		OTP:              otpHandler,
		Verifier:         conn.tokenVerifier,
		AllowedOrigins:   cfg.App.AllowedOrigins,
		AllowCredentials: cfg.App.AllowCredentials,
	})

	return &application{
		router: r,
		conn:   conn,
		logger: l,
	}, nil
}

func (a *application) Close() {
	a.logger.Info("shutting down connections...")

	done := make(chan struct{})
	go func() {
		defer close(done)
		a.conn.Close()
	}()

	select {
	case <-done:
		a.logger.Info("all connections closed")
	case <-time.After(5 * time.Second):
		a.logger.Warn("connection shutdown timed out, forcing exit")
	}
}
