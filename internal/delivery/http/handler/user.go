package handler

import (
	"net/http"

	"be-modami-auth-service/internal/entity"
	"be-modami-auth-service/internal/usecase"
	"be-modami-auth-service/pkg/ctxutil"
	"be-modami-auth-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type User struct {
	keycloak *usecase.KeycloakUseCase
}

func NewUser(keycloak *usecase.KeycloakUseCase) *User {
	return &User{keycloak: keycloak}
}

// Me godoc
// @Summary      Get current user
// @Description  Returns the authenticated user's claims from JWT token
// @Tags         user
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response{data=entity.KeycloakClaims}
// @Failure      401 {object} response.Response
// @Router       /api/v1/me [get]
func (h *User) Me(c *gin.Context) {
	claims, ok := ctxutil.GetClaims(c)
	if !ok {
		response.Error(c, entity.ErrUnauthorized)
		return
	}
	response.OK(c, claims)
}

// List godoc
// @Summary      List users
// @Description  Returns a list of users from Keycloak (admin only)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response{data=[]entity.User}
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /api/v1/admin/users [get]
func (h *User) List(c *gin.Context) {
	users, err := h.keycloak.GetUsers(c.Request.Context(), 0, 50)
	if err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadGateway, "failed to fetch users", err))
		return
	}
	response.OK(c, users)
}

// GetByID godoc
// @Summary      Get user by ID
// @Description  Returns a specific user from Keycloak (admin only)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Success      200 {object} response.Response{data=entity.User}
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /api/v1/admin/users/{id} [get]
func (h *User) GetByID(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.Error(c, entity.ErrBadRequest)
		return
	}

	user, err := h.keycloak.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadGateway, "failed to fetch user", err))
		return
	}
	response.OK(c, user)
}
