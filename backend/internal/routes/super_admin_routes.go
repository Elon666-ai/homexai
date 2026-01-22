package routes

import (
	"homexai/internal/handler"
	"homexai/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupSuperAdminRoutes sets up routes for super admin operations
func SetupSuperAdminRoutes(
	rg *gin.RouterGroup,
	adminHandler *handler.PropertyAdminHandler,
	superAdminHandler *handler.SuperAdminHandler,
	forumAdHandler *handler.ForumAdHandler,
) {
	// Super admin routes - requires super_admin role
	superAdmin := rg.Group("/super-admin")
	superAdmin.Use(middleware.AuthMiddleware())
	superAdmin.Use(middleware.RequireSuperAdmin())
	superAdmin.Use(middleware.RateLimitMiddleware())

	// Property management routes (create, list, get, update)
	properties := superAdmin.Group("/properties")
	{
		properties.POST("", superAdminHandler.CreateProperty)
		properties.GET("", superAdminHandler.ListProperties)
		properties.GET("/:id", superAdminHandler.GetProperty)
		properties.PUT("/:id", superAdminHandler.UpdateProperty)
	}

	// Property admin assignment routes
	propertyAdmins := superAdmin.Group("/property-admins")
	{
		propertyAdmins.GET("", adminHandler.ListPropertiesWithAdmins)
		propertyAdmins.GET("/:property_id", adminHandler.GetPropertyAdmin)
		propertyAdmins.POST("/:property_id", adminHandler.AssignPropertyAdmin)
		propertyAdmins.PUT("/:property_id", adminHandler.ReplacePropertyAdmin)
		propertyAdmins.DELETE("/:property_id", adminHandler.RemovePropertyAdmin)
	}

	// Forum ad management routes (B2B advertising system)
	forumAds := superAdmin.Group("/forum-ads")
	{
		forumAds.GET("", forumAdHandler.List)
		forumAds.POST("", forumAdHandler.Create)
		forumAds.GET("/:id", forumAdHandler.Get)
		forumAds.PUT("/:id", forumAdHandler.Update)
		forumAds.DELETE("/:id", forumAdHandler.Delete)
		forumAds.POST("/:id/activate", forumAdHandler.Activate)
		forumAds.POST("/:id/deactivate", forumAdHandler.Deactivate)
	}
}
