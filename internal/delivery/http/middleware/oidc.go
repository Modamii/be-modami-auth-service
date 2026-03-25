package middleware

import (
	"github.com/cenfit/be-cenfit-auth-service/internal/usecase"
	"github.com/cenfit/be-cenfit-auth-service/pkg/ctxutil"
	"github.com/cenfit/be-cenfit-auth-service/pkg/response"
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
