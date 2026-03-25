package handler

import (
	"net/http"

	"github.com/Nerzal/gocloak/v13"
	"github.com/cenfit/be-cenfit-auth-service/internal/entity"
	"github.com/cenfit/be-cenfit-auth-service/internal/usecase"
	"github.com/cenfit/be-cenfit-auth-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type Role struct {
	keycloak *usecase.KeycloakUseCase
}

func NewRole(keycloak *usecase.KeycloakUseCase) *Role {
	return &Role{keycloak: keycloak}
}

// ListRealmRoles godoc
// @Summary      List realm roles
// @Description  Returns all realm roles from Keycloak (admin only)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /api/v1/admin/roles [get]
func (h *Role) ListRealmRoles(c *gin.Context) {
	roles, err := h.keycloak.GetRealmRoles(c.Request.Context())
	if err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadGateway, "failed to fetch roles", err))
		return
	}
	response.OK(c, roles)
}

// GetUserRoles godoc
// @Summary      Get user roles
// @Description  Returns realm roles assigned to a specific user (admin only)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Success      200 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /api/v1/admin/users/{id}/roles [get]
func (h *Role) GetUserRoles(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.Error(c, entity.ErrBadRequest)
		return
	}

	roles, err := h.keycloak.GetUserRealmRoles(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadGateway, "failed to fetch user roles", err))
		return
	}
	response.OK(c, roles)
}

type assignRolesRequest struct {
	Roles []gocloak.Role `json:"roles" binding:"required"`
}

// AssignRoles godoc
// @Summary      Assign roles to user
// @Description  Assign realm roles to a specific user (admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Param        request body assignRolesRequest true "Roles to assign"
// @Success      204
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /api/v1/admin/users/{id}/roles [post]
func (h *Role) AssignRoles(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.Error(c, entity.ErrBadRequest)
		return
	}

	var req assignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadRequest, "invalid request body", err))
		return
	}

	if err := h.keycloak.AssignRealmRoles(c.Request.Context(), userID, req.Roles); err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadGateway, "failed to assign roles", err))
		return
	}

	response.NoContent(c)
}

// RemoveRoles godoc
// @Summary      Remove roles from user
// @Description  Remove realm roles from a specific user (admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Param        request body assignRolesRequest true "Roles to remove"
// @Success      204
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /api/v1/admin/users/{id}/roles [delete]
func (h *Role) RemoveRoles(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.Error(c, entity.ErrBadRequest)
		return
	}

	var req assignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadRequest, "invalid request body", err))
		return
	}

	if err := h.keycloak.RemoveRealmRoles(c.Request.Context(), userID, req.Roles); err != nil {
		response.Error(c, entity.NewAppError(http.StatusBadGateway, "failed to remove roles", err))
		return
	}

	response.NoContent(c)
}
