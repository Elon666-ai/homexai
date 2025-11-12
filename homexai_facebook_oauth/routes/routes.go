package routes

import (
	"homexai_oauth/controllers"
	"homexai_oauth/middleware"
	"homexai_oauth/services"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, authService *services.AuthService) {
	authController := controllers.NewAuthController(authService)

	// Google OAuth路由
	googleAuth := router.Group("/auth/google")
	{
		googleAuth.GET("/login", authController.GoogleLogin)
		googleAuth.GET("/callback", authController.GoogleCallback)
	}

	// Facebook OAuth路由
	facebookAuth := router.Group("/auth/facebook")
	{
		facebookAuth.GET("/login", authController.FacebookLogin)
		facebookAuth.GET("/callback", authController.FacebookCallback)
	}

	// 通用认证路由
	auth := router.Group("/auth")
	{
		auth.POST("/verify", authController.VerifyToken)
	}

	// 需要认证的API路由
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
