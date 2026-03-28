package main

import (
	"context"
	"os"
	"time"

	"be-modami-auth-service/config"
	"be-modami-auth-service/internal/command"
	deliveryhttp "be-modami-auth-service/internal/delivery/http"
	"be-modami-auth-service/internal/delivery/http/handler"

	"github.com/go-playground/validator/v10"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

type application struct {
	server *command.Server
	conn   *connections
	logger logging.Logger
}

func newApplication(cfg *config.Config) (*application, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	l := logger.L()

	health := handler.NewHealth()

	conn, err := initConnections(ctx, cfg, health, l)
	if err != nil {
		return nil, err
	}

	// Handlers
	authHandler := handler.NewAuth(conn.authKeycloakUC)
	userHandler := handler.NewUser(conn.keycloakUC)
	roleHandler := handler.NewRole(conn.keycloakUC)

	// OTP handler (optional — depends on Redis + Email)
	var otpHandler *handler.OTPHandler
	if conn.otpUseCase != nil {
		otpHandler = handler.NewOTPHandler(conn.otpUseCase, validator.New())
	}

	// Router
	r := deliveryhttp.NewRouter(deliveryhttp.RouterDeps{
		Health:   health,
		Auth:     authHandler,
		User:     userHandler,
		Role:     roleHandler,
		OTP:      otpHandler,
		Verifier: conn.tokenVerifier,
		Logger:   l,
	})

	// Server
	srv := command.NewServer(cfg.Server.Port, r, cfg.Server.GetShutdownTimeout(), l)

	return &application{
		server: srv,
		conn:   conn,
		logger: l,
	}, nil
}

func (a *application) Run() {
	if err := a.server.Run(); err != nil {
		a.logger.Error("server error", err)
		os.Exit(1)
	}
}

func (a *application) Close() {
	a.conn.Close()
}
