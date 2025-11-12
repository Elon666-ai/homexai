package routes

import (
	"homexai_email_auth/controllers"
	"homexai_email_auth/middleware"
	"homexai_email_auth/services"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, authService *services.AuthService) {
	authController := controllers.NewAuthController(authService)

	// 公开路由
	auth := router.Group("/auth")
	{
		auth.POST("/send-code", authController.SendVerificationCode)
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
		auth.POST("/request-reset", authController.RequestPasswordReset)
		auth.POST("/reset-password", authController.ResetPassword)
		auth.POST("/verify-token", authController.VerifyToken)
	}

	// 需要认证的路由
	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware(authService))
	{
		api.GET("/profile", authController.GetProfile)
	}

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}
