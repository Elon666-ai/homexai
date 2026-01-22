package routes

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupSwaggerRoutes sets up Swagger documentation routes
func SetupSwaggerRoutes(router *gin.Engine) {
	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API documentation info page
	router.GET("/docs", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "API Documentation",
			"swagger": "/swagger/index.html",
			"postman": "/docs/postman",
			"version": "1.5.6",
		})
	})
}

// SwaggerInfo holds the swagger documentation metadata
// This will be populated by swag init command
/*
// @title HomeX API
// @version 1.5.6
// @description Property management system API for HomeX platform
// @termsOfService http://homex.ph/terms/

// @contact.name API Support
// @contact.url http://homex.ph/support
// @contact.email support@homex.ph

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @tag.name Auth
// @tag.description Authentication and authorization endpoints

// @tag.name User
// @tag.description User management endpoints

// @tag.name Unit
// @tag.description Unit (apartment/parking) management endpoints

// @tag.name Bill
// @tag.description Bill management endpoints
*/
