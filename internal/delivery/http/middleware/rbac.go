package middleware

import (
	"be-modami-auth-service/pkg/ctxutil"

	"github.com/gin-gonic/gin"
	pkgresponse "gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

func RequireRealmRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ctxutil.GetClaims(c)
		if !ok {
			pkgresponse.Unauthorized(c.Writer, "unauthorized")
			c.Abort()
			return
		}

		for _, role := range roles {
			if claims.HasRealmRole(role) {
				c.Next()
				return
			}
		}

		pkgresponse.Forbidden(c.Writer, "forbidden")
		c.Abort()
	}
}

func RequireClientRole(clientID string, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ctxutil.GetClaims(c)
		if !ok {
			pkgresponse.Unauthorized(c.Writer, "unauthorized")
			c.Abort()
			return
		}

		for _, role := range roles {
			if claims.HasClientRole(clientID, role) {
				c.Next()
				return
			}
		}

		pkgresponse.Forbidden(c.Writer, "forbidden")
		c.Abort()
	}
}
