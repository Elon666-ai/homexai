package routes

import (
	"homexai/internal/handler"
	"homexai/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupPublicRoutes sets up public routes (no authentication required)
func SetupPublicRoutes(rg *gin.RouterGroup, authHandler *handler.AuthHandler, oauthHandler *handler.OAuthHandler) {
	// Authentication routes
	auth := rg.Group("/auth")
	{
		// Login endpoints
		auth.POST("/login", middleware.TurnstileMiddleware(), authHandler.Login)

		// Registration
		auth.POST("/register", middleware.TurnstileMiddleware(), authHandler.Register)

		// Verification code
		auth.POST("/send-code", middleware.RateLimitByIP(5), authHandler.SendCode)

		// Password reset
		auth.POST("/reset-password", middleware.TurnstileMiddleware(), authHandler.ResetPassword)
		
		// Token refresh
		auth.POST("/refresh", authHandler.RefreshToken)

		// Email/Phone verification
		auth.POST("/verify-email", authHandler.VerifyEmail)
		auth.POST("/verify-phone", authHandler.VerifyPhone)

		// OAuth routes
		auth.GET("/google", oauthHandler.GoogleLogin)
		auth.GET("/google/callback", oauthHandler.GoogleCallback)
		auth.GET("/facebook", oauthHandler.FacebookLogin)
		auth.GET("/facebook/callback", oauthHandler.FacebookCallback)
	}
}
