package routes

import (
	"homexai/internal/handler"
	"homexai/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupLandlordRoutes sets up routes for landlord property management
func SetupLandlordRoutes(
	rg *gin.RouterGroup,
	superAdminHandler *handler.SuperAdminHandler,
) {
	// Landlord routes - requires landlord role
	landlord := rg.Group("/landlord")
	landlord.Use(middleware.AuthMiddleware())
	landlord.Use(middleware.RequireRole("landlord"))
	landlord.Use(middleware.RateLimitMiddleware())

	// Property management routes (create, list, get, update)
	properties := landlord.Group("/properties")
	{
		properties.POST("", superAdminHandler.CreatePropertyByLandlord)
		properties.GET("", superAdminHandler.ListPropertiesByLandlord)
		properties.GET("/:id", superAdminHandler.GetPropertyByLandlord)
		properties.PUT("/:id", superAdminHandler.UpdatePropertyByLandlord)
	}
}

