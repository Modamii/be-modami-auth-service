package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"be-modami-auth-service/internal/delivery/http/middleware"
	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/pkg/ctxutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockVerifier struct {
	claims *entity.KeycloakClaims
	err    error
}

func (m *mockVerifier) VerifyToken(_ context.Context, _ string) (*entity.KeycloakClaims, error) {
	return m.claims, m.err
}

func TestOIDC_MissingAuthHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	r.Use(middleware.OIDC(&mockVerifier{}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOIDC_InvalidToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	r.Use(middleware.OIDC(&mockVerifier{err: apperror.ErrUnauthorized}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOIDC_ValidToken(t *testing.T) {
	claims := &entity.KeycloakClaims{
		Sub:               "user-123",
		Email:             "test@example.com",
		PreferredUsername: "testuser",
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	var gotClaims *entity.KeycloakClaims
	r.Use(middleware.OIDC(&mockVerifier{claims: claims}))
	r.GET("/test", func(c *gin.Context) {
		gotClaims, _ = ctxutil.GetClaims(c)
		c.Status(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, gotClaims)
	assert.Equal(t, "user-123", gotClaims.Sub)
	assert.Equal(t, "test@example.com", gotClaims.Email)
}
