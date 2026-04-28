package middleware

import (
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/ctxutil"

	"github.com/gin-gonic/gin"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
	pkgresponse "gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

func OIDC(verifier usecase.TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, err := usecase.ExtractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			if ae := apperror.AsAppError(err); ae != nil {
				pkgresponse.Err(c.Writer, ae)
			} else {
				pkgresponse.Unauthorized(c.Writer, "unauthorized")
			}
			c.Abort()
			return
		}

		claims, err := verifier.VerifyToken(c.Request.Context(), rawToken)
		if err != nil {
			if ae := apperror.AsAppError(err); ae != nil {
				pkgresponse.Err(c.Writer, ae)
			} else {
				pkgresponse.Unauthorized(c.Writer, "unauthorized")
			}
			c.Abort()
			return
		}

		ctxutil.SetClaims(c, claims)
		c.Next()
	}
}
