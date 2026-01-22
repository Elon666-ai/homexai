package service

import (
	"time"
	"homexai/internal/models/property"
	"gorm.io/gorm"
)

type NotificationService struct {
	db *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{
		db: db,
	}
}

// CreateNotification creates a new notification
func (s *NotificationService) CreateNotification(notification *property.Notification) error {
	return s.db.Create(notification).Error
}

// GetUserNotifications gets notifications for a user
func (s *NotificationService) GetUserNotifications(userID uint, propertyID uint, page, perPage int) ([]property.Notification, int64, error) {
	var notifications []property.Notification
	var total int64

	query := s.db.Where("user_id = ? AND property_id = ?", userID, propertyID)

	// Count total
	if err := query.Model(&property.Notification{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * perPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&notifications).Error; err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

// GetUnreadCount gets unread notification count for a user
func (s *NotificationService) GetUnreadCount(userID uint, propertyID uint) (int64, error) {
	var count int64
	err := s.db.Model(&property.Notification{}).
		Where("user_id = ? AND property_id = ? AND is_read = ?", userID, propertyID, false).
		Count(&count).Error
	return count, err
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(notificationID uint, userID uint) error {
	now := time.Now()
	return s.db.Model(&property.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": &now,
		}).Error
}

// MarkAllAsRead marks all notifications as read for a user
func (s *NotificationService) MarkAllAsRead(userID uint, propertyID uint) error {
	now := time.Now()
	return s.db.Model(&property.Notification{}).
		Where("user_id = ? AND property_id = ? AND is_read = ?", userID, propertyID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": &now,
		}).Error
}

// CreateRequestStatusChangeNotification creates a notification when request status changes
func (s *NotificationService) CreateRequestStatusChangeNotification(
	userID uint,
	propertyID uint,
	requestID uint,
	requestTitle string,
	oldStatus string,
	newStatus string,
) error {
	title := "Request Status Updated"
	content := ""
	
	// Generate content based on status change
	if newStatus == "in_progress" {
		content = "Your request \"" + requestTitle + "\" is now in progress."
	} else if newStatus == "completed" {
		content = "Your request \"" + requestTitle + "\" has been completed."
	} else if newStatus == "rejected" {
		content = "Your request \"" + requestTitle + "\" has been rejected."
	} else if newStatus == "cancelled" {
		content = "Your request \"" + requestTitle + "\" has been cancelled."
	} else {
		content = "Your request \"" + requestTitle + "\" status changed from " + oldStatus + " to " + newStatus + "."
	}

	relatedType := property.RelatedTypeRequest
	notification := &property.Notification{
		UserID:      userID,
		PropertyID:  propertyID,
		Type:        property.NotificationTypeRequestStatusChange,
		Title:       title,
		Content:     content,
		RelatedID:   &requestID,
		RelatedType: &relatedType,
		IsRead:      false,
	}

	return s.CreateNotification(notification)
}

// CreateAnnouncementNotification creates a notification when an announcement is published
func (s *NotificationService) CreateAnnouncementNotification(
	userID uint,
	propertyID uint,
	announcementID uint,
	announcementTitle string,
) error {
	relatedType := property.RelatedTypeAnnouncement
	notification := &property.Notification{
		UserID:      userID,
		PropertyID:  propertyID,
		Type:        property.NotificationTypeAnnouncementPublished,
		Title:       "New Announcement",
		Content:     "A new announcement \"" + announcementTitle + "\" has been published.",
		RelatedID:   &announcementID,
		RelatedType: &relatedType,
		IsRead:      false,
	}

	return s.CreateNotification(notification)
}

