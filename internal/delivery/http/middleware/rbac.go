package middleware

import (
	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/pkg/ctxutil"
	"be-modami-auth-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequireRealmRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ctxutil.GetClaims(c)
		if !ok {
			response.Error(c, entity.ErrUnauthorized)
			return
		}

		for _, role := range roles {
			if claims.HasRealmRole(role) {
				c.Next()
				return
			}
		}

		response.Error(c, entity.ErrForbidden)
	}
}

func RequireClientRole(clientID string, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ctxutil.GetClaims(c)
		if !ok {
			response.Error(c, entity.ErrUnauthorized)
			return
		}

		for _, role := range roles {
			if claims.HasClientRole(clientID, role) {
				c.Next()
				return
			}
		}

		response.Error(c, entity.ErrForbidden)
	}
}
