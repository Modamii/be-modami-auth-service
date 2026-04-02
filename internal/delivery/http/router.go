package http

import (
	"be-modami-auth-service/config"
	"be-modami-auth-service/internal/delivery/http/handler"
	"be-modami-auth-service/internal/delivery/http/middleware"
	"be-modami-auth-service/internal/usecase"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"

	_ "be-modami-auth-service/docs"
)

type RouterDeps struct {
	Health   *handler.Health
	Auth     *handler.Auth
	User     *handler.User
	Role     *handler.Role
	OTP      *handler.OTPHandler
	Verifier usecase.TokenVerifier
	Logger   logging.Logger
	CORS     config.CORSConfig
}

func NewRouter(deps RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.ZapLogger(deps.Logger))
	r.Use(gin.Recovery())

	origins := deps.CORS.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{
			"http://localhost:5173",
			"http://localhost:3000",
			"http://localhost:8080",
			"http://localhost:8081",
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		AllowCredentials: deps.CORS.AllowCredentials,
		MaxAge:           300,
	}))

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health
	r.GET("/healthz", deps.Health.Liveness)
	r.GET("/readyz", deps.Health.Readiness)

	// Auth (public — no OIDC)
	auth := r.Group("/v1/auth-services/auth")
	{
		auth.POST("/login", deps.Auth.Login)
		auth.POST("/register", deps.Auth.Register)
		auth.POST("/logout", deps.Auth.Logout)
		auth.POST("/refresh", deps.Auth.RefreshToken)
		auth.POST("/forgot-password", deps.Auth.ForgotPassword)
		auth.GET("/social/login", deps.Auth.SocialLogin)
		auth.GET("/social/callback", deps.Auth.SocialCallback)
		auth.GET("/auth/me", deps.User.Me)
		auth.PUT("/auth/password", deps.Auth.ChangePassword)
		auth.PUT("/auth/profile", deps.Auth.UpdateProfile)

		// Unified OTP endpoints (purpose dispatched inside handler)
		if deps.OTP != nil {
			auth.POST("/otp/send", deps.OTP.SendOTP)
			auth.POST("/otp/verify", deps.OTP.VerifyOTP)
			auth.POST("/reset-password", deps.OTP.ResetPassword)
		}
	}

	// Protected API
	api := r.Group("/v1/auth-services")
	if deps.Verifier != nil {
		api.Use(middleware.OIDC(deps.Verifier))
	}
	{
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
