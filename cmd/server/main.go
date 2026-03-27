package main

import (
	"context"
	"log"

	"be-modami-auth-service/config"
	"be-modami-auth-service/pkg/logger"

	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

// @title           Modami Auth Service API
// @version         1.0
// @description     Authentication service for the Modami marketplace platform.
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

	logCfg := logging.Config{
		ServiceName:    cfg.App.Name,
		ServiceVersion: cfg.App.Version,
		Environment:    cfg.App.Environment,
		Level:          cfg.Log.Level,
		OTLPEndpoint:   cfg.Log.OTLPEndpoint,
		Insecure:       cfg.Log.Insecure,
		TLSCertFile:    cfg.Log.TLSCertFile,
	}

	if err := logger.Init(logCfg); err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Shutdown(context.Background())

	app, err := newApplication(cfg)
	if err != nil {
		logger.Error(context.Background(), "failed to initialize application", err)
		log.Fatalf("failed to initialize application: %v", err)
	}
	defer app.Close()

	app.Run()
}
