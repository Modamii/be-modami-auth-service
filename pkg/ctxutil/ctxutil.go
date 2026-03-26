package ctxutil

import (
	"be-modami-auth-service/internal/entity"

	"github.com/gin-gonic/gin"
)

const claimsKey = "keycloak_claims"

func SetClaims(c *gin.Context, claims *entity.KeycloakClaims) {
	c.Set(claimsKey, claims)
}

func GetClaims(c *gin.Context) (*entity.KeycloakClaims, bool) {
	v, exists := c.Get(claimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := v.(*entity.KeycloakClaims)
	return claims, ok
}
