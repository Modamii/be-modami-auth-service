package handler

import (
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/ctxutil"
	"be-modami-auth-service/pkg/response"

	"github.com/gin-gonic/gin"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
)

type User struct {
	keycloak *usecase.KeycloakUseCase
}

func NewUser(keycloak *usecase.KeycloakUseCase) *User {
	return &User{keycloak: keycloak}
}


func (h *User) Me(c *gin.Context) {
	claims, ok := ctxutil.GetClaims(c)
	if !ok {
		response.Error(c, apperror.ErrUnauthorized)
		return
	}
	response.OK(c, claims)
}


func (h *User) List(c *gin.Context) {
	users, err := h.keycloak.GetUsers(c.Request.Context(), 0, 50)
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeBadGateway, "failed to fetch users").WithError(err))
		return
	}
	response.OK(c, users)
}


func (h *User) GetByID(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.Error(c, apperror.ErrBadRequest)
		return
	}

	user, err := h.keycloak.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeBadGateway, "failed to fetch user").WithError(err))
		return
	}
	response.OK(c, user)
}
