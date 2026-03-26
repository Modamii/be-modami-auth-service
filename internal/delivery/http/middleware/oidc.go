package middleware

import (
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/ctxutil"
	"be-modami-auth-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func OIDC(verifier usecase.TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, err := usecase.ExtractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			response.Error(c, err)
			return
		}

		claims, err := verifier.VerifyToken(c.Request.Context(), rawToken)
		if err != nil {
			response.Error(c, err)
			return
		}

		ctxutil.SetClaims(c, claims)
		c.Next()
	}
}
