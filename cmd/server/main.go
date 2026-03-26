package main

import (
	"log"

	"be-modami-auth-service/config"
	"be-modami-auth-service/pkg/logger"

	"go.uber.org/zap"
)

// @title           Cenfit Auth Service API
// @version         1.0
// @description     Authentication service powered by Keycloak
// @host            localhost:8085
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your token in the format: **Bearer {token}**

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	zapLogger, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer zapLogger.Sync()

	app, err := newApplication(cfg, zapLogger)
	if err != nil {
		zapLogger.Fatal("failed to initialize application", zap.Error(err))
	}
	defer app.Close()

	app.Run()
}
