package routes

import (
	"homexai/internal/config"
	"homexai/internal/database"
	"homexai/internal/handler"
	"homexai/internal/middleware"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

// SetupRoutes sets up all application routes
func SetupRoutes(router *gin.Engine, masterDB *database.MasterDB) {

	// Initialize SMTP service (Gmail)
	smtpSvc := service.NewSmtpService(&config.Yaml.Email)

	// Initialize SMS service (Twilio)
	smsSvc := service.NewSMSService(config.Yaml.SMS.AccountSID, config.Yaml.SMS.AuthToken, config.Yaml.SMS.FromNumber)

	// Initialize verification service
	verificationSvc := service.NewVerificationService(database.GetRedisClient())

	// Initialize import service
	importSvc := service.NewImportService(masterDB.DB, verificationSvc, smtpSvc)

	// Initialize request service
	requestSvc := service.NewRequestService(masterDB.DB)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(smtpSvc, smsSvc)
	oauthHandler := handler.NewOAuthHandler()
	userHandler := handler.NewUserHandler()
	unitHandler := handler.NewUnitHandler()
	billHandler := handler.NewBillHandler()
	staffHandler := handler.NewPropertyStaffHandler(smtpSvc)
	propertyAdminHandler := handler.NewPropertyAdminHandler()
	superAdminHandler := handler.NewSuperAdminHandler(masterDB.DB)
	importHandler := handler.NewImportHandler(masterDB.DB, importSvc)
	requestHandler := handler.NewRequestHandler(masterDB.DB, requestSvc, smtpSvc)
	complaintHandler := handler.NewComplaintHandler(masterDB.DB)
	announcementHandler := handler.NewAnnouncementHandler(masterDB.DB, smtpSvc)
	visitorHandler := handler.NewVisitorHandler(masterDB.DB)
	forumHandler := handler.NewForumHandler(masterDB.DB)
	facilityHandler := handler.NewFacilityHandler()
	marketplaceHandler := handler.NewMarketplaceHandler()
	tenantHandler := handler.NewTenantHandler(masterDB.DB)
	landlordHandler := handler.NewLandlordHandler(masterDB.DB)
	spaHandler := handler.NewSPAHandler(masterDB.DB)

	// Apply global middleware
	router.Use(middleware.RecoveryMiddleware())
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.I18nMiddleware())

	// Health check endpoint
	router.GET("/health", HealthCheck)
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "HomeX API Server",
			"version": utils.VERSION,
			"status":  "running",
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Static file serving for uploads (inside v1 to match API base URL)
		v1.Static("/static/uploads", "./uploads")

		// Public routes (no authentication required)
		SetupPublicRoutes(v1, authHandler, oauthHandler)

		// Authenticated routes
		SetupAuthenticatedRoutes(
			v1,
			authHandler,
			oauthHandler,
			userHandler,
			unitHandler,
			billHandler,
			requestHandler,
			complaintHandler,
			announcementHandler,
			visitorHandler,
			forumHandler,
			facilityHandler,
			tenantHandler,
			landlordHandler,
			spaHandler,
			masterDB,
		)

		// Admin routes
		SetupAdminRoutes(v1, userHandler, unitHandler, billHandler, forumHandler)

		// Property admin routes (staff management + import + settings)
		settingsHandler := handler.NewSettingsHandler(masterDB.DB)
		SetupPropertyAdminRoutes(v1, staffHandler, importHandler, marketplaceHandler, settingsHandler)

		// Super admin routes (property management + admin assignment + forum ads)
		forumAdHandler := handler.NewForumAdHandler(masterDB.DB)
		SetupSuperAdminRoutes(v1, propertyAdminHandler, superAdminHandler, forumAdHandler)

		// Landlord routes (property management)
		SetupLandlordRoutes(v1, superAdminHandler)
	}
}

// HealthCheck handles health check requests
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "healthy",
		"message": "Server is running",
		"app":     utils.APP_NAME,
		"version": utils.VERSION,
	})
}
