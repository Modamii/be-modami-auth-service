package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"be-modami-auth-service/internal/delivery/http/middleware"
	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/pkg/ctxutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireRealmRole_HasRole(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		ctxutil.SetClaims(c, &entity.KeycloakClaims{
			RealmAccess: entity.RealmAccess{Roles: []string{"admin", "user"}},
		})
		c.Next()
	})
	r.Use(middleware.RequireRealmRole("admin"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRealmRole_NoRole(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		ctxutil.SetClaims(c, &entity.KeycloakClaims{
			RealmAccess: entity.RealmAccess{Roles: []string{"user"}},
		})
		c.Next()
	})
	r.Use(middleware.RequireRealmRole("admin"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRealmRole_NoClaims(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(middleware.RequireRealmRole("admin"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireClientRole_HasRole(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		ctxutil.SetClaims(c, &entity.KeycloakClaims{
			ResourceAccess: map[string]entity.ResourceAccess{
				"cenfit-api": {Roles: []string{"manage-users"}},
			},
		})
		c.Next()
	})
	r.Use(middleware.RequireClientRole("cenfit-api", "manage-users"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
