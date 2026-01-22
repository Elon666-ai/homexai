package routes

import (
	"homexai/internal/handler"
	"homexai/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupAdminRoutes sets up routes that require admin privileges
func SetupAdminRoutes(
	rg *gin.RouterGroup,
	userHandler *handler.UserHandler,
	unitHandler *handler.UnitHandler,
	billHandler *handler.BillHandler,
	forumHandler *handler.ForumHandler,
) {
	// User management routes (super admin only)
	superAdmin := rg.Group("/admin/users")
	superAdmin.Use(middleware.AuthMiddleware())
	superAdmin.Use(middleware.RequireSuperAdmin())
	superAdmin.Use(middleware.RateLimitMiddleware())
	{
		superAdmin.GET("", userHandler.ListUsers)
		superAdmin.GET("/:id", userHandler.GetUser)
		superAdmin.POST("/:id/activate", userHandler.ActivateUser)
		superAdmin.POST("/:id/suspend", userHandler.SuspendUser)
		superAdmin.POST("/:id/deactivate", userHandler.DeactivateUser)
	}

	// User search routes (allow property staff and others for tenant selection)
	userSearch := rg.Group("/admin/users")
	userSearch.Use(middleware.AuthMiddleware())
	userSearch.Use(middleware.RequireRole("super_admin", "property_admin", "property_staff", "landlord", "spa"))
	userSearch.Use(middleware.RateLimitMiddleware())
	{
		userSearch.GET("/search", userHandler.SearchUsers)
	}

	// Unit management routes (allow property_admin, property_staff, and landlord)
	unitAdmin := rg.Group("/admin")
	unitAdmin.Use(middleware.AuthMiddleware())
	unitAdmin.Use(middleware.RequireRole("super_admin", "property_admin", "property_staff", "landlord"))
	unitAdmin.Use(middleware.RateLimitMiddleware())
	unitProperty := unitAdmin.Group("")
	unitProperty.Use(middleware.TenantMiddleware())
	{
		// Unit management
		SetupAdminUnitRoutes(unitProperty, unitHandler)
		// Parking management (under units)
		SetupAdminParkingRoutes(unitProperty, unitHandler)
	}

	// Forum management routes (property admin only)
	forumAdmin := rg.Group("/admin")
	forumAdmin.Use(middleware.AuthMiddleware())
	forumAdmin.Use(middleware.RequirePropertyAdmin())
	forumAdmin.Use(middleware.RateLimitMiddleware())
	forumProperty := forumAdmin.Group("")
	forumProperty.Use(middleware.TenantMiddleware())
	{
		// Forum management
		SetupAdminForumRoutes(forumProperty, forumHandler)
	}

	// Bill management routes (allow property_account in addition to property_admin)
	// Create a separate group that doesn't use RequirePropertyAdmin
	billAdmin := rg.Group("/admin")
	billAdmin.Use(middleware.AuthMiddleware())
	billAdmin.Use(middleware.RequireRole("super_admin", "property_admin", "property_account"))
	billAdmin.Use(middleware.RateLimitMiddleware())
	billProperty := billAdmin.Group("")
	billProperty.Use(middleware.TenantMiddleware())
	{
		SetupAdminBillRoutes(billProperty, billHandler)
	}
}

// SetupAdminUnitRoutes sets up admin unit management routes
func SetupAdminUnitRoutes(rg *gin.RouterGroup, unitHandler *handler.UnitHandler) {
	units := rg.Group("/units")
	{
		// CRUD operations
		units.GET("", unitHandler.ListUnits)
		units.GET("/:id", unitHandler.GetUnit)
		units.POST("", unitHandler.CreateUnit)
		units.PUT("/:id", unitHandler.UpdateUnit)
		units.DELETE("/:id", unitHandler.DeleteUnit)

		// Tenant management for units
		units.GET("/:id/tenants", unitHandler.GetUnitTenants)
		units.POST("/:id/tenants", unitHandler.CreateUnitTenant)

		// Batch operations (for future implementation)
		units.POST("/import", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Batch import not implemented yet"})
		})
		units.POST("/export", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Export not implemented yet"})
		})
	}
}

// SetupAdminBillRoutes sets up admin bill management routes
// Allows property_admin, property_account, and super_admin to access
// Note: The parent route group already has RequireRole middleware
func SetupAdminBillRoutes(rg *gin.RouterGroup, billHandler *handler.BillHandler) {
	bills := rg.Group("/bills")
	{
		// CRUD operations
		bills.POST("", billHandler.CreateBill)
		bills.GET("", billHandler.ListBills)
		bills.PUT("/:id", billHandler.UpdateBill)
		bills.GET("/overdue", billHandler.ListOverdueBills)
		bills.GET("/due-soon", billHandler.ListDueSoonBills)
		bills.GET("/statistics", billHandler.GetStatistics)

		// Bill operations
		bills.POST("/:id/pay", billHandler.MarkBillAsPaid)
		bills.POST("/:id/cancel", billHandler.CancelBill)

		// Batch operations (for future implementation)
		bills.POST("/generate", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Batch bill generation not implemented yet"})
		})
		bills.POST("/export", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Export not implemented yet"})
		})
	}
}

// SetupAdminParkingRoutes sets up admin parking management routes
func SetupAdminParkingRoutes(rg *gin.RouterGroup, unitHandler *handler.UnitHandler) {
	parking := rg.Group("/parking")
	{
		// Parking operations
		parking.GET("", unitHandler.ListParkingSpaces)
	}
}

// SetupAdminForumRoutes sets up admin forum management routes
func SetupAdminForumRoutes(rg *gin.RouterGroup, forumHandler *handler.ForumHandler) {
	forum := rg.Group("/forum")
	{
		posts := forum.Group("/posts")
		{
			// Admin actions
			posts.POST("/:id/pin", forumHandler.PinPost)
			posts.POST("/:id/unpin", forumHandler.UnpinPost)
			posts.DELETE("/:id", forumHandler.DeletePost)
		}

		replies := forum.Group("/replies")
		{
			replies.DELETE("/:id", forumHandler.DeleteReply)
		}
	}
}
