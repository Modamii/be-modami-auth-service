package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cenfit/be-cenfit-auth-service/internal/delivery/http/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealth_Liveness(t *testing.T) {
	h := handler.NewHealth()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.Liveness(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "alive")
}

func TestHealth_Readiness_AllHealthy(t *testing.T) {
	h := handler.NewHealth()
	h.AddCheck(func(ctx context.Context) error { return nil })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readiness(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ready")
}

func TestHealth_Readiness_Unhealthy(t *testing.T) {
	h := handler.NewHealth()
	h.AddCheck(func(ctx context.Context) error {
		return errors.New("db connection failed")
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readiness(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "not ready")
}
