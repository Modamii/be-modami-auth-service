package http

import (
	"be-modami-auth-service/internal/delivery/http/handler"
	"be-modami-auth-service/internal/delivery/http/middleware"
	"be-modami-auth-service/internal/usecase"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	_ "be-modami-auth-service/docs"
)

type RouterDeps struct {
	Health   *handler.Health
	Auth     *handler.Auth
	User     *handler.User
	Role     *handler.Role
	Verifier usecase.TokenVerifier
	Logger   *zap.Logger
}

func NewRouter(deps RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.ZapLogger(deps.Logger))
	r.Use(gin.Recovery())

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health
	r.GET("/healthz", deps.Health.Liveness)
	r.GET("/readyz", deps.Health.Readiness)

	// Auth (public — no OIDC)
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", deps.Auth.Login)
		auth.POST("/register", deps.Auth.Register)
		auth.POST("/logout", deps.Auth.Logout)
		auth.POST("/refresh", deps.Auth.RefreshToken)
		auth.POST("/forgot-password", deps.Auth.ForgotPassword)
		auth.GET("/social/login", deps.Auth.SocialLogin)
		auth.GET("/social/callback", deps.Auth.SocialCallback)
	}

	// Protected API
	api := r.Group("/api/v1")
	if deps.Verifier != nil {
		api.Use(middleware.OIDC(deps.Verifier))
	}
	{
		api.GET("/me", deps.User.Me)
		api.PUT("/me/password", deps.Auth.ChangePassword)
		api.PUT("/me/profile", deps.Auth.UpdateProfile)

		admin := api.Group("/admin", middleware.RequireRealmRole("admin"))
		{
			admin.GET("/users", deps.User.List)
			admin.GET("/users/:id", deps.User.GetByID)
			admin.GET("/users/:id/roles", deps.Role.GetUserRoles)
			admin.POST("/users/:id/roles", deps.Role.AssignRoles)
			admin.DELETE("/users/:id/roles", deps.Role.RemoveRoles)
			admin.GET("/roles", deps.Role.ListRealmRoles)
		}
	}

	return r
}
