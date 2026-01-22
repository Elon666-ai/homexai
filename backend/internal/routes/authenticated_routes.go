package routes

import (
	"homexai/internal/database"
	"homexai/internal/handler"
	"homexai/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupAuthenticatedRoutes sets up routes that require authentication
func SetupAuthenticatedRoutes(
	rg *gin.RouterGroup,
	authHandler *handler.AuthHandler,
	oauthHandler *handler.OAuthHandler,
	userHandler *handler.UserHandler,
	unitHandler *handler.UnitHandler,
	billHandler *handler.BillHandler,
	requestHandler *handler.RequestHandler,
	complaintHandler *handler.ComplaintHandler,
	announcementHandler *handler.AnnouncementHandler,
	visitorHandler *handler.VisitorHandler,
	forumHandler *handler.ForumHandler,
	facilityHandler *handler.FacilityHandler,
	tenantHandler *handler.TenantHandler,
	landlordHandler *handler.LandlordHandler,
	spaHandler *handler.SPAHandler,
	masterDB *database.MasterDB,
) {
	// Apply authentication middleware
	authenticated := rg.Group("")
	authenticated.Use(middleware.AuthMiddleware())
	authenticated.Use(middleware.RateLimitMiddleware())

	// Auth routes (authenticated)
	auth := authenticated.Group("/auth")
	{
		auth.POST("/change-password", authHandler.ChangePassword)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/logout-all", authHandler.LogoutAllDevices)

		// OAuth management
		oauth := auth.Group("/oauth")
		{
			oauth.GET("/providers", oauthHandler.GetLinkedProviders)
			oauth.DELETE("/unlink/:provider", oauthHandler.UnlinkProvider)
		}
	}

	// User profile routes
	user := authenticated.Group("/user")
	user.Use(middleware.TenantMiddleware())
	{
		// Profile management
		user.GET("/profile", userHandler.GetProfile)
		user.PUT("/profile", userHandler.UpdateProfile)
		user.PUT("/privacy", userHandler.UpdatePrivacySettings)
		user.PUT("/language", userHandler.UpdateLanguage)
	}

	// Property-scoped routes (require tenant context)
	property := authenticated.Group("")
	property.Use(middleware.TenantMiddleware())
	{
		// Tenant management routes
		tenantGroup := property.Group("/tenant")
		tenantGroup.Use(middleware.RequireRole("property_staff", "property_admin"))
		{
			tenantGroup.POST("/create", tenantHandler.CreateTenant)
			tenantGroup.GET("/list", tenantHandler.ListTenants)
			tenantGroup.GET("/:id", tenantHandler.GetTenant)
			tenantGroup.PUT("/:id", tenantHandler.UpdateTenant)
			tenantGroup.DELETE("/:id", tenantHandler.DeleteTenant)
		}

		// Get user by ID (for landlord viewing tenant, or admin)
		// This route needs property context for landlord permission checks
		property.GET("/user/:id", userHandler.GetUser)

		// User lists by role
		property.GET("/user/owners/names", userHandler.GetOwnerNames)
		property.GET("/user/tenants/names", userHandler.GetTenantNames)
		property.GET("/user/spas/names", userHandler.GetSPANames)
		// Unit routes
		SetupUnitRoutes(property, unitHandler, landlordHandler, spaHandler)

		// Bill routes (includes create for accountant)
		SetupBillRoutes(property, billHandler)

		// Request routes
		SetupRequestRoutes(property, requestHandler)

		// Complaint routes
		SetupComplaintRoutes(property, complaintHandler)

		// Announcement routes
		SetupAnnouncementRoutes(property, announcementHandler)

		// Visitor routes
		SetupVisitorRoutes(property, visitorHandler)

		// Forum routes
		SetupForumRoutes(property, forumHandler)

		// Forum ad public view route (all authenticated users can view)
		SetupForumAdPublicRoutes(property, masterDB)

		// Facility & reservation routes
		SetupFacilityRoutes(property, facilityHandler)

		// Marketplace routes
		SetupMarketplaceRoutes(property)

		// Notification routes
		SetupNotificationRoutes(property)
	}
}

// SetupFacilityRoutes sets up facility-related routes
func SetupFacilityRoutes(rg *gin.RouterGroup, facilityHandler *handler.FacilityHandler) {
	f := rg.Group("/facilities")
	{
		// List / detail for all authenticated users in property
		f.GET("", facilityHandler.ListFacilities)
		f.GET("/:id", facilityHandler.GetFacility)

		// Admin/staff management
		f.POST("", facilityHandler.CreateFacility)
		f.PUT("/:id", facilityHandler.UpdateFacility)
		f.DELETE("/:id", facilityHandler.DeleteFacility)

		// Reservations (住户侧 + 物业审批)
		f.POST("/reservations", facilityHandler.CreateReservation)
		f.GET("/my-reservations", facilityHandler.ListMyReservations)
		f.POST("/reservations/:id/cancel", facilityHandler.CancelReservation)

		// 物业端查看 & 审批
		f.GET("/reservations/all", facilityHandler.ListAllReservations)
		f.GET("/:id/reservations", facilityHandler.ListFacilityReservations)
		f.POST("/reservations/:id/approve", facilityHandler.ApproveReservation)
		f.POST("/reservations/:id/reject", facilityHandler.RejectReservation)
		f.POST("/reservations/:id/complete", facilityHandler.CompleteReservation)
	}
}

// SetupUnitRoutes sets up unit-related routes
func SetupUnitRoutes(rg *gin.RouterGroup, unitHandler *handler.UnitHandler, landlordHandler *handler.LandlordHandler, spaHandler *handler.SPAHandler) {
	units := rg.Group("/units")
	{
		// List and search
		units.GET("", unitHandler.ListUnits)
		units.GET("/apartments", unitHandler.ListApartments)
		units.GET("/parking", unitHandler.ListParkingSpaces)
		units.GET("/search", unitHandler.SearchUnits)
		units.GET("/statistics", unitHandler.GetStatistics)
		units.GET("/my", unitHandler.GetMyUnits) // Get current user's associated units

		// Single unit operations (accessible to tenants)
		units.GET("/:id", unitHandler.GetUnit)
		units.GET("/:id/service-fee", unitHandler.GetUnitServiceFee) // Calculate service fee for unit
		units.GET("/:id/tenants", unitHandler.GetUnitTenants)        // Get tenants for a unit
		units.POST("/:id/tenants", unitHandler.CreateUnitTenant)     // Create tenant for unit
		units.PUT("/:id", unitHandler.UpdateUnit)                    // Update unit (for landlord/spa)
	}

	// Parking slots routes
	parkingSlots := rg.Group("/parking-slots")
	{
		parkingSlots.GET("/my", unitHandler.GetMyParkingSlots)  // Get current user's parking slots
		parkingSlots.GET("/:id", unitHandler.GetParkingSlot)    // Get parking slot by ID
		parkingSlots.POST("", unitHandler.CreateParkingSlot)    // Create parking slot
		parkingSlots.PUT("/:id", unitHandler.UpdateParkingSlot) // Update parking slot
	}

	// Parking management routes for staff
	parking := rg.Group("/parking")
	parking.Use(middleware.RequireRole("property_staff", "property_admin"))
	{
		parking.POST("/create", unitHandler.CreateParkingSlot) // Create parking slot for staff
	}

	// Landlord management routes (admin only)
	landlord := rg.Group("/landlord")
	landlord.Use(middleware.RequireRole("property_admin"))
	{
		landlord.GET("/list", landlordHandler.ListLandlords)
		landlord.POST("/create", landlordHandler.CreateLandlord)
		landlord.GET("/:id", landlordHandler.GetLandlord)
		landlord.PUT("/:id", landlordHandler.UpdateLandlord)
		landlord.DELETE("/:id", landlordHandler.DeleteLandlord)

		// SPA management routes
		spa := rg.Group("/spa")
		spa.Use(middleware.RequireRole("property_admin"))
		{
			spa.GET("/:id", spaHandler.GetSPA)
		}
	}
}

// SetupBillRoutes sets up bill-related routes
func SetupBillRoutes(rg *gin.RouterGroup, billHandler *handler.BillHandler) {
	bills := rg.Group("/bills")
	{
		// Create bill (only for accountant)
		bills.POST("", billHandler.CreateBill)

		// User's own bills
		bills.GET("/my", billHandler.GetMyBills)
		bills.GET("/my/total-due", billHandler.GetMyTotalDue)
		bills.GET("/pending-count", billHandler.GetPendingBillsCount)
		bills.GET("/pending", billHandler.GetPendingBills)

		// Single bill operations
		bills.GET("/:id", billHandler.GetBill)
		bills.PUT("/:id", billHandler.UpdateBill)

		// Submit payment for bill
		bills.POST("/:id/payments", billHandler.SubmitPayment)
	}
}

// SetupRequestRoutes sets up request-related routes
func SetupRequestRoutes(rg *gin.RouterGroup, requestHandler *handler.RequestHandler) {
	requests := rg.Group("/requests")
	{
		// List and statistics
		requests.GET("", requestHandler.ListRequests)
		requests.GET("/my", requestHandler.GetMyRequests)
		requests.GET("/stats", requestHandler.GetRequestStats)
		requests.GET("/types", requestHandler.GetRequestTypes)

		// CRUD operations
		requests.POST("", requestHandler.CreateRequest)
		requests.GET("/:id", requestHandler.GetRequest)
		requests.PUT("/:id", requestHandler.UpdateRequest)

		// Attachments
		requests.POST("/:id/attachments", requestHandler.UploadRequestAttachment)
		requests.GET("/:id/attachments", requestHandler.GetRequestAttachments)
		requests.DELETE("/:id/attachments/:attachmentId", requestHandler.DeleteRequestAttachment)

		// Traces (workflow history)
		requests.GET("/:id/traces", requestHandler.GetRequestTraces)
		requests.POST("/:id/comments", requestHandler.AddRequestComment)

		// Staff actions
		requests.POST("/:id/resolve", requestHandler.ResolveRequest)

		// Property staff approval actions
		requests.POST("/:id/approve", requestHandler.ApproveRequest)
		requests.POST("/:id/reject", requestHandler.RejectRequest)

		// Requester resubmit action (for rejected requests)
		requests.POST("/:id/resubmit", requestHandler.ResubmitRequest)

		// Work permit specific routes
		requests.POST("/work-permit", requestHandler.CreateWorkPermit)
		requests.GET("/:id/work-permit", requestHandler.GetWorkPermit)
		requests.PUT("/:id/work-permit", requestHandler.UpdateWorkPermit)

		// Gate pass specific routes
		requests.POST("/gate-pass", requestHandler.CreateGatePass)
		requests.GET("/:id/gate-pass", requestHandler.GetGatePass)
		requests.PUT("/:id/gate-pass", requestHandler.UpdateGatePass)

		// Vehicle sticker specific routes
		requests.POST("/vehicle-sticker", requestHandler.CreateVehicleSticker)
		requests.GET("/:id/vehicle-sticker", requestHandler.GetVehicleSticker)
		requests.PUT("/:id/vehicle-sticker", requestHandler.UpdateVehicleSticker)

		// Pet registration specific routes
		requests.POST("/pet-registration", requestHandler.CreatePetRegistration)
		requests.GET("/:id/pet-registration", requestHandler.GetPetRegistration)
		requests.PUT("/:id/pet-registration", requestHandler.UpdatePetRegistration)

		// Household staff registration specific routes
		requests.POST("/household-staff-registration", requestHandler.CreateHouseholdStaffRegistration)
		requests.GET("/:id/household-staff-registration", requestHandler.GetHouseholdStaffRegistration)
		requests.PUT("/:id/household-staff-registration", requestHandler.UpdateHouseholdStaffRegistration)

		// Move-in specific routes
		requests.POST("/move-in", requestHandler.CreateMoveIn)
		requests.GET("/:id/move-in", requestHandler.GetMoveIn)
		requests.PUT("/:id/move-in", requestHandler.UpdateMoveIn)

		// Move-out specific routes
		requests.POST("/move-out", requestHandler.CreateMoveOut)
		requests.GET("/:id/move-out", requestHandler.GetMoveOut)
		requests.PUT("/:id/move-out", requestHandler.UpdateMoveOut)
		requests.GET("/unit-move-in-occupants", requestHandler.GetUnitMoveInOccupants)
	}
}

// SetupComplaintRoutes sets up complaint-related routes
func SetupComplaintRoutes(rg *gin.RouterGroup, complaintHandler *handler.ComplaintHandler) {
	complaints := rg.Group("/complaints")
	{
		// User's own complaints
		complaints.GET("/my", complaintHandler.ListMyComplaints)

		// All complaints (for staff)
		complaints.GET("", complaintHandler.ListAllComplaints)

		// CRUD operations
		complaints.POST("", complaintHandler.CreateComplaint)
		complaints.GET("/:id", complaintHandler.GetComplaint)

		// Message operations
		complaints.GET("/:id/messages", complaintHandler.GetComplaintMessages)
		complaints.POST("/:id/messages", complaintHandler.SendComplaintMessage)

		// Close complaint (only by creator)
		complaints.POST("/:id/close", complaintHandler.CloseComplaint)
	}
}

// SetupAnnouncementRoutes sets up announcement-related routes
func SetupAnnouncementRoutes(rg *gin.RouterGroup, announcementHandler *handler.AnnouncementHandler) {
	announcements := rg.Group("/announcements")
	{
		// List announcements (all authenticated users can view)
		announcements.GET("", announcementHandler.List)
		announcements.GET("/:id", announcementHandler.Get)

		// Create/Update/Delete (staff only - enforced in handler)
		announcements.POST("", announcementHandler.Create)
		announcements.PUT("/:id", announcementHandler.Update)
		announcements.DELETE("/:id", announcementHandler.Delete)
	}
}

// SetupVisitorRoutes sets up visitor-related routes
func SetupVisitorRoutes(rg *gin.RouterGroup, visitorHandler *handler.VisitorHandler) {
	visitors := rg.Group("/visitors")
	{
		// List and statistics
		visitors.GET("", visitorHandler.ListVisitors)
		visitors.GET("/stats", visitorHandler.GetVisitorStats)

		// CRUD operations
		visitors.POST("", visitorHandler.CreateVisitor)
		visitors.GET("/:id", visitorHandler.GetVisitor)
		visitors.PUT("/:id", visitorHandler.UpdateVisitor)

		// Approval actions (host or staff)
		visitors.POST("/:id/approve", visitorHandler.ApproveVisitor)
		visitors.POST("/:id/reject", visitorHandler.RejectVisitor)
		visitors.POST("/:id/cancel", visitorHandler.CancelVisitor)

		// Staff-only check-in/check-out actions
		visitors.POST("/:id/checkin", visitorHandler.CheckInVisitor)
		visitors.POST("/:id/checkout", visitorHandler.CheckOutVisitor)
	}
}

// SetupForumRoutes sets up forum-related routes
func SetupForumRoutes(rg *gin.RouterGroup, forumHandler *handler.ForumHandler) {
	forum := rg.Group("/forum")
	{
		// User image count
		forum.GET("/user/image-count", forumHandler.GetUserImageCount)

		// Post routes
		posts := forum.Group("/posts")
		{
			// List and get posts
			posts.GET("", forumHandler.List)
			posts.GET("/:id", forumHandler.Get)

			// Create and update posts (authenticated users)
			posts.POST("", forumHandler.Create)
			posts.PUT("/:id", forumHandler.Update)

			// Vote on posts
			posts.POST("/:id/vote", forumHandler.Vote)
			posts.GET("/:id/vote-results", forumHandler.GetVoteResults)

			// Reply routes
			posts.POST("/:id/replies", forumHandler.CreateReply)
			posts.GET("/:id/replies", forumHandler.ListReplies)
		}

		// Reply routes (for deletion)
		replies := forum.Group("/replies")
		{
			replies.DELETE("/:id", forumHandler.DeleteReply)
		}
	}

}

// SetupMarketplaceRoutes sets up marketplace-related routes
func SetupMarketplaceRoutes(rg *gin.RouterGroup) {
	marketplaceHandler := handler.NewMarketplaceHandler()
	marketplace := rg.Group("/marketplace")
	{
		// Browse active service listings
		marketplace.GET("/services", marketplaceHandler.ListActiveServiceListings)

		// Service orders
		marketplace.POST("/orders", marketplaceHandler.CreateServiceOrder)
		marketplace.GET("/orders", marketplaceHandler.ListServiceOrders)
		marketplace.GET("/orders/my", marketplaceHandler.GetMyServiceOrders)
		marketplace.GET("/orders/:id", marketplaceHandler.GetServiceOrder)
		marketplace.POST("/orders/:id/confirm", marketplaceHandler.ConfirmServiceOrder)
		marketplace.POST("/orders/:id/cancel", marketplaceHandler.CancelServiceOrder)
	}

	// Property staff/admin routes for order management
	propertyStaff := rg.Group("/property-admin/marketplace/orders")
	propertyStaff.Use(middleware.RequireRole("property_staff", "property_admin", "super_admin"))
	{
		propertyStaff.POST("/:id/complete", marketplaceHandler.CompleteServiceOrder)
	}
}

// SetupNotificationRoutes sets up notification-related routes
func SetupNotificationRoutes(rg *gin.RouterGroup) {
	notifications := rg.Group("/notifications")
	{
		notifications.GET("", func(c *gin.Context) {
			propertyDB := middleware.GetPropertyDB(c)
			handler := handler.NewNotificationHandler(propertyDB)
			handler.ListNotifications(c)
		})
		notifications.GET("/unread-count", func(c *gin.Context) {
			propertyDB := middleware.GetPropertyDB(c)
			handler := handler.NewNotificationHandler(propertyDB)
			handler.GetUnreadCount(c)
		})
		notifications.PUT("/:id/read", func(c *gin.Context) {
			propertyDB := middleware.GetPropertyDB(c)
			handler := handler.NewNotificationHandler(propertyDB)
			handler.MarkAsRead(c)
		})
		notifications.PUT("/read-all", func(c *gin.Context) {
			propertyDB := middleware.GetPropertyDB(c)
			handler := handler.NewNotificationHandler(propertyDB)
			handler.MarkAllAsRead(c)
		})
	}
}

// SetupForumAdPublicRoutes sets up public forum ad viewing routes for all authenticated users
func SetupForumAdPublicRoutes(rg *gin.RouterGroup, masterDB *database.MasterDB) {
	forumAds := rg.Group("/forum-ads")
	{
		// Public view - all authenticated users can view ad details
		forumAds.GET("/:id", func(c *gin.Context) {
			adHandler := handler.NewForumAdHandler(masterDB.DB)
			adHandler.Get(c)
		})
	}
}
