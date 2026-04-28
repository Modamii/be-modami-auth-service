package handler

import (
	"be-modami-auth-service/internal/usecase"

	"github.com/Nerzal/gocloak/v13"
	"github.com/gin-gonic/gin"
	"gitlab.com/lifegoeson-libs/pkg-gokit/apperror"
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
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} map[string]interface{}
// @Failure      403 {object} map[string]interface{}
// @Router       /admin/roles [get]
func (h *Role) ListRealmRoles(c *gin.Context) {
	roles, err := h.keycloak.GetRealmRoles(c.Request.Context())
	if err != nil {
		respondError(c, apperror.New(apperror.CodeBadGateway, "failed to fetch roles").WithError(err))
		return
	}
	respondOK(c, roles)
}

// GetUserRoles godoc
// @Summary      Get user roles
// @Description  Returns realm roles assigned to a specific user (admin only)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} map[string]interface{}
// @Failure      403 {object} map[string]interface{}
// @Router       /admin/users/{id}/roles [get]
func (h *Role) GetUserRoles(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		respondError(c, apperror.ErrBadRequest)
		return
	}

	roles, err := h.keycloak.GetUserRealmRoles(c.Request.Context(), userID)
	if err != nil {
		respondError(c, apperror.New(apperror.CodeBadGateway, "failed to fetch user roles").WithError(err))
		return
	}
	respondOK(c, roles)
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
// @Failure      401 {object} map[string]interface{}
// @Failure      403 {object} map[string]interface{}
// @Router       /admin/users/{id}/roles [post]
func (h *Role) AssignRoles(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		respondError(c, apperror.ErrBadRequest)
		return
	}

	var req assignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	if err := h.keycloak.AssignRealmRoles(c.Request.Context(), userID, req.Roles); err != nil {
		respondError(c, apperror.New(apperror.CodeBadGateway, "failed to assign roles").WithError(err))
		return
	}

	respondNoContent(c)
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
// @Failure      401 {object} map[string]interface{}
// @Failure      403 {object} map[string]interface{}
// @Router       /admin/users/{id}/roles [delete]
func (h *Role) RemoveRoles(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		respondError(c, apperror.ErrBadRequest)
		return
	}

	var req assignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperror.New(apperror.CodeBadRequest, "invalid request body").WithError(err))
		return
	}

	if err := h.keycloak.RemoveRealmRoles(c.Request.Context(), userID, req.Roles); err != nil {
		respondError(c, apperror.New(apperror.CodeBadGateway, "failed to remove roles").WithError(err))
		return
	}

	respondNoContent(c)
}
