package handler

import (
	"strconv"

	"homexai/internal/middleware"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotificationHandler handles notification-related HTTP requests
type NotificationHandler struct {
	propertyDB          *gorm.DB
	notificationService *service.NotificationService
}

// NewNotificationHandler creates a new NotificationHandler
func NewNotificationHandler(propertyDB *gorm.DB) *NotificationHandler {
	return &NotificationHandler{
		propertyDB:          propertyDB,
		notificationService: service.NewNotificationService(propertyDB),
	}
}

// ListNotifications gets user's notifications
// @Summary List notifications
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /notifications [get]
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	propertyID := middleware.GetPropertyID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	notificationService := service.NewNotificationService(propertyDB)
	notifications, total, err := notificationService.GetUserNotifications(userID, propertyID, page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve notifications", err)
		return
	}

	utils.SuccessResponseWithPagination(c, notifications, total, page, perPage, "Notifications retrieved successfully")
}

// GetUnreadCount gets unread notification count
// @Summary Get unread count
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /notifications/unread-count [get]
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	propertyID := middleware.GetPropertyID(c)

	notificationService := service.NewNotificationService(propertyDB)
	count, err := notificationService.GetUnreadCount(userID, propertyID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get unread count", err)
		return
	}

	utils.SuccessResponse(c, "Unread count retrieved successfully", gin.H{"count": count})
}

// MarkAsRead marks a notification as read
// @Summary Mark notification as read
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param id path int true "Notification ID"
// @Success 200 {object} map[string]interface{}
// @Router /notifications/{id}/read [put]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid notification ID", nil)
		return
	}

	notificationService := service.NewNotificationService(propertyDB)
	if err := notificationService.MarkAsRead(uint(id), userID); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to mark notification as read", err)
		return
	}

	utils.SuccessResponse(c, "Notification marked as read", nil)
}

// MarkAllAsRead marks all notifications as read
// @Summary Mark all notifications as read
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /notifications/read-all [put]
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	propertyID := middleware.GetPropertyID(c)

	notificationService := service.NewNotificationService(propertyDB)
	if err := notificationService.MarkAllAsRead(userID, propertyID); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to mark all notifications as read", err)
		return
	}

	utils.SuccessResponse(c, "All notifications marked as read", nil)
}
