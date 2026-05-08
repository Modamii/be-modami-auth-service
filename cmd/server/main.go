package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"be-modami-auth-service/config"
	"be-modami-auth-service/docs"

	logging "gitlab.com/lifegoeson-libs/pkg-logging"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
	pkgloggingmw "gitlab.com/lifegoeson-libs/pkg-logging/middleware"
)

// @title           Modami Auth Service API
// @version         1.0
// @description     Authentication service for the Modami marketplace platform.
// @host            localhost:8085
// @BasePath        /v1/auth-services

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your token in the format: **Bearer {token}**
type Server struct {
	httpServer *http.Server
	logger     logging.Logger
	shutdown   time.Duration
}

func newServer(addr string, handler http.Handler, shutdownTimeout time.Duration, logger logging.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger:   logger,
		shutdown: shutdownTimeout,
	}
}

func (s *Server) Run() error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", logging.String("addr", s.httpServer.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		s.logger.Info("shutting down", logging.String("signal", sig.String()))
	}

	go func() {
		<-quit
		s.logger.Warn("forced exit")
		os.Exit(1)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.shutdown)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	s.logger.Info("server stopped")
	return nil
}


func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	docs.SwaggerInfo.Host = cfg.App.SwaggerHost
	if err := logger.Init(logging.Config{
		ServiceName:    cfg.Observability.ServiceName,
		ServiceVersion: cfg.Observability.ServiceVersion,
		Environment:    cfg.Observability.Environment,
		Level:          cfg.Observability.LogLevel,
		OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
		Insecure:       cfg.Observability.OTLPInsecure,
	}); err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Shutdown(context.Background())

	app, err := newApplication(cfg)
	if err != nil {
		logger.Error(context.Background(), "failed to initialize application", err)
		log.Fatalf("failed to initialize application: %v", err)
	}
	defer app.Close()

	wrappedRouter := pkgloggingmw.HTTPMiddleware("auth-service", app.router, &pkgloggingmw.HttpLoggingOptions{
		ExceptRoutes: []string{"/healthz", "/readyz"},
	})

	srv := newServer(cfg.App.ListenAddr(), wrappedRouter, cfg.App.GetShutdownTimeout(), app.logger)
	
	if err := srv.Run(); err != nil {
		logger.Error(context.Background(), "server error", err, logging.String("error", err.Error()))
		os.Exit(1)
	}
}
