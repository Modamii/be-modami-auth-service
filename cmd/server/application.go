package main

import (
	"context"
	"time"

	"github.com/cenfit/be-cenfit-auth-service/config"
	"github.com/cenfit/be-cenfit-auth-service/internal/command"
	deliveryhttp "github.com/cenfit/be-cenfit-auth-service/internal/delivery/http"
	"github.com/cenfit/be-cenfit-auth-service/internal/delivery/http/handler"
	"go.uber.org/zap"
)

type application struct {
	server *command.Server
	conn   *connections
	logger *zap.Logger
}

func newApplication(cfg *config.Config, logger *zap.Logger) (*application, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	health := handler.NewHealth()

	conn, err := initConnections(ctx, cfg, health, logger)
	if err != nil {
		return nil, err
	}

	// Handlers
	authHandler := handler.NewAuth(conn.authKeycloakUC)
	userHandler := handler.NewUser(conn.keycloakUC)
	roleHandler := handler.NewRole(conn.keycloakUC)

	// Router
	r := deliveryhttp.NewRouter(deliveryhttp.RouterDeps{
		Health:   health,
		Auth:     authHandler,
		User:     userHandler,
		Role:     roleHandler,
		Verifier: conn.tokenVerifier,
		Logger:   logger,
	})

	// Server
	srv := command.NewServer(cfg.Server.Port, r, cfg.Server.GetShutdownTimeout(), logger)

	return &application{
		server: srv,
		conn:   conn,
		logger: logger,
	}, nil
}

func (a *application) Run() {
	if err := a.server.Run(); err != nil {
		a.logger.Fatal("server error", zap.Error(err))
	}
}

func (a *application) Close() {
	a.conn.Close()
}
