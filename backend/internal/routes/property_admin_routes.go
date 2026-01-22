package routes

import (
	"homexai/internal/handler"
	"homexai/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupPropertyAdminRoutes sets up routes for property admin to manage staff
func SetupPropertyAdminRoutes(
	rg *gin.RouterGroup,
	staffHandler *handler.PropertyStaffHandler,
	importHandler *handler.ImportHandler,
	marketplaceHandler *handler.MarketplaceHandler,
	settingsHandler *handler.SettingsHandler,
) {
	// Property admin routes - requires property_admin role
	propertyAdmin := rg.Group("/property-admin")
	propertyAdmin.Use(middleware.AuthMiddleware())
	propertyAdmin.Use(middleware.TenantMiddleware())
	propertyAdmin.Use(middleware.RequirePropertyAdmin())
	propertyAdmin.Use(middleware.RateLimitMiddleware())
	
	// Property staff/admin routes for marketplace services
	propertyStaffServices := rg.Group("/property-admin/marketplace/services")
	propertyStaffServices.Use(middleware.AuthMiddleware())
	propertyStaffServices.Use(middleware.TenantMiddleware())
	propertyStaffServices.Use(middleware.RequireRole("property_staff", "property_admin", "super_admin"))
	propertyStaffServices.Use(middleware.RateLimitMiddleware())
	{
		propertyStaffServices.GET("", marketplaceHandler.ListServiceListings)
		propertyStaffServices.POST("", marketplaceHandler.CreateServiceListing)
		propertyStaffServices.GET("/:id", marketplaceHandler.GetServiceListing)
		propertyStaffServices.PUT("/:id", marketplaceHandler.UpdateServiceListing)
		propertyStaffServices.DELETE("/:id", marketplaceHandler.DeleteServiceListing)
		propertyStaffServices.PUT("/:id/status", marketplaceHandler.UpdateServiceListingStatus)
	}
	
	// Property staff/admin routes for marketplace orders (view only)
	propertyStaffOrders := rg.Group("/property-admin/marketplace/orders")
	propertyStaffOrders.Use(middleware.AuthMiddleware())
	propertyStaffOrders.Use(middleware.TenantMiddleware())
	propertyStaffOrders.Use(middleware.RequireRole("property_staff", "property_admin", "super_admin"))
	propertyStaffOrders.Use(middleware.RateLimitMiddleware())
	{
		propertyStaffOrders.GET("", marketplaceHandler.ListServiceOrders)
		propertyStaffOrders.GET("/:id", marketplaceHandler.GetServiceOrder)
	}

	// Staff management routes
	staff := propertyAdmin.Group("/staff")
	{
		// Staff CRUD operations
		staff.GET("", staffHandler.ListStaff)
		staff.POST("", staffHandler.CreateStaff)
		staff.GET("/:id", staffHandler.GetStaff)
		staff.PUT("/:id", staffHandler.UpdateStaff)
		staff.DELETE("/:id", staffHandler.DeleteStaff)
	}

	// Import routes
	importRoutes := propertyAdmin.Group("/import")
	{
		// Units import
		importRoutes.POST("/units", importHandler.ImportUnits)
		importRoutes.POST("/units/:id/start", importHandler.StartImport)
		importRoutes.GET("/template", importHandler.DownloadTemplate)

		// Parking import
		importRoutes.POST("/parking", importHandler.ImportParking)
		importRoutes.POST("/parking/:id/start", importHandler.StartImport)
		importRoutes.GET("/parking/template", importHandler.DownloadParkingTemplate)

		// Import tasks
		importRoutes.GET("/tasks", importHandler.GetImportTasks)
		importRoutes.GET("/tasks/:id", importHandler.GetImportTask)
	}

	// Marketplace management routes (admin only - assign staff)
	marketplace := propertyAdmin.Group("/marketplace")
	{
		// Service order management (admin only - assign staff)
		orders := marketplace.Group("/orders")
		{
			orders.POST("/:id/assign-staff", marketplaceHandler.AssignStaff)
		}
	}

	// Settings routes (payment methods - for property_account only)
	paymentMethodsSettings := rg.Group("/property-admin/settings")
	paymentMethodsSettings.Use(middleware.AuthMiddleware())
	paymentMethodsSettings.Use(middleware.TenantMiddleware())
	paymentMethodsSettings.Use(middleware.RateLimitMiddleware())
	{
		// GET endpoint: Allow all authenticated users (tenant/landlord need to see payment instructions)
		paymentMethodsSettings.GET("/payment-methods", settingsHandler.GetPaymentMethods)
		// POST endpoint: Only allow property_account and super_admin to update
		paymentMethodsSettingsUpdate := paymentMethodsSettings.Group("")
		paymentMethodsSettingsUpdate.Use(middleware.RequireRole("property_account", "super_admin"))
		{
			paymentMethodsSettingsUpdate.POST("/payment-methods", settingsHandler.UpdatePaymentMethods)
		}
	}
}
