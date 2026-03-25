package handler

import (
	"context"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type HealthChecker func(ctx context.Context) error

type Health struct {
	mu     sync.RWMutex
	checks []HealthChecker
}

func NewHealth() *Health {
	return &Health{}
}

func (h *Health) AddCheck(check HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, check)
}

// Liveness godoc
// @Summary      Liveness check
// @Description  Returns 200 if the service is running
// @Tags         health
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /healthz [get]
func (h *Health) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// Readiness godoc
// @Summary      Readiness check
// @Description  Returns 200 if all dependencies (DB, Keycloak) are healthy
// @Tags         health
// @Produce      json
// @Success      200 {object} map[string]string
// @Failure      503 {object} map[string]string
// @Router       /readyz [get]
func (h *Health) Readiness(c *gin.Context) {
	h.mu.RLock()
	checks := make([]HealthChecker, len(h.checks))
	copy(checks, h.checks)
	h.mu.RUnlock()

	for _, check := range checks {
		if err := check(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
