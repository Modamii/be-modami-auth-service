package command

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

type Server struct {
	httpServer *http.Server
	logger     logging.Logger
	shutdown   time.Duration
}

func NewServer(addr string, handler http.Handler, shutdownTimeout time.Duration, logger logging.Logger) *Server {
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
