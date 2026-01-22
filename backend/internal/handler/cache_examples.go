package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"homexai/internal/models/property"
	"homexai/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Example: Handler with cache integration
type CachedAnnouncementHandler struct {
	db           *gorm.DB
	cacheService *service.CacheService
}

// NewCachedAnnouncementHandler creates a handler with cache support
func NewCachedAnnouncementHandler(db *gorm.DB, cacheService *service.CacheService) *CachedAnnouncementHandler {
	return &CachedAnnouncementHandler{
		db:           db,
		cacheService: cacheService,
	}
}

// GetAnnouncementWithCache demonstrates caching a single item
func (h *CachedAnnouncementHandler) GetAnnouncementWithCache(c *gin.Context) {
	propertyID := c.GetUint("property_id")
	announcementID := c.Param("id")

	ctx := c.Request.Context()

	var announcement property.Announcement

	// Try to get from cache using GetOrSet
	err := h.cacheService.GetOrSet(
		ctx,
		fmt.Sprintf("announcement:%d:%s", propertyID, announcementID),
		10*time.Minute,
		func(ctx context.Context) (interface{}, error) {
			// Loader function - called only on cache miss
			var ann property.Announcement
			err := h.db.WithContext(ctx).
				Where("id = ? AND property_id = ?", announcementID, propertyID).
				First(&ann).Error
			if err != nil {
				return nil, err
			}
			return ann, nil
		},
		&announcement,
	)

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Announcement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   announcement,
		"cached": true,
	})
}

// ListAnnouncementsWithCache demonstrates caching a list
func (h *CachedAnnouncementHandler) ListAnnouncementsWithCache(c *gin.Context) {
	propertyID := c.GetUint("property_id")
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")

	ctx := c.Request.Context()
	cacheKey := fmt.Sprintf("announcements:list:%d:page_%s:size_%s", propertyID, page, pageSize)

	type AnnouncementListResponse struct {
		Announcements []property.Announcement `json:"announcements"`
		Total         int64                   `json:"total"`
		Page          string                  `json:"page"`
		PageSize      string                  `json:"page_size"`
	}

	var response AnnouncementListResponse

	// Try cache first
	err := h.cacheService.GetOrSet(
		ctx,
		cacheKey,
		5*time.Minute, // Cache lists for shorter time
		func(ctx context.Context) (interface{}, error) {
			// Load from database
			var announcements []property.Announcement
			var total int64

			db := h.db.WithContext(ctx).
				Where("property_id = ?", propertyID)

			// Get total count
			if err := db.Model(&property.Announcement{}).Count(&total).Error; err != nil {
				return nil, err
			}

			// Get paginated results
			if err := db.Order("created_at DESC").
				Limit(20).
				Offset(0).
				Find(&announcements).Error; err != nil {
				return nil, err
			}

			return AnnouncementListResponse{
				Announcements: announcements,
				Total:         total,
				Page:          page,
				PageSize:      pageSize,
			}, nil
		},
		&response,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcements"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CreateAnnouncementWithCache demonstrates cache invalidation on create
func (h *CachedAnnouncementHandler) CreateAnnouncementWithCache(c *gin.Context) {
	propertyID := c.GetUint("property_id")

	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Create announcement
	announcement := property.Announcement{
		// PropertyID: propertyID,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := h.db.WithContext(ctx).Create(&announcement).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create announcement"})
		return
	}

	// Invalidate list cache
	if err := h.cacheService.InvalidateAnnouncementList(ctx, propertyID); err != nil {
		// Log warning but don't fail the request
		fmt.Printf("Warning: failed to invalidate cache: %v\n", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Announcement created successfully",
		"data":    announcement,
	})
}

// UpdateAnnouncementWithCache demonstrates cache invalidation on update
func (h *CachedAnnouncementHandler) UpdateAnnouncementWithCache(c *gin.Context) {
	propertyID := c.GetUint("property_id")
	announcementID := c.Param("id")

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Update announcement
	result := h.db.WithContext(ctx).
		Model(&property.Announcement{}).
		Where("id = ? AND property_id = ?", announcementID, propertyID).
		Updates(map[string]interface{}{
			"title":   req.Title,
			"content": req.Content,
		})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update announcement"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Announcement not found"})
		return
	}

	// Invalidate both item cache and list cache
	announcementIDUint, _ := c.Params.Get("id")
	if err := h.cacheService.InvalidateAnnouncement(ctx, propertyID, uint(announcementIDUint[0])); err != nil {
		fmt.Printf("Warning: failed to invalidate announcement cache: %v\n", err)
	}

	if err := h.cacheService.InvalidateAnnouncementList(ctx, propertyID); err != nil {
		fmt.Printf("Warning: failed to invalidate list cache: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Announcement updated successfully",
	})
}

// Example: Dashboard with aggressive caching
type CachedDashboardHandler struct {
	db           *gorm.DB
	cacheService *service.CacheService
}

// GetDashboardStats demonstrates caching dashboard statistics
func (h *CachedDashboardHandler) GetDashboardStats(c *gin.Context) {
	propertyID := c.GetUint("property_id")
	userRole := c.GetString("user_role")

	ctx := c.Request.Context()

	type DashboardStats struct {
		TotalUnits          int64   `json:"total_units"`
		OccupiedUnits       int64   `json:"occupied_units"`
		PendingRequests     int64   `json:"pending_requests"`
		UnpaidBills         int64   `json:"unpaid_bills"`
		TotalRevenue        float64 `json:"total_revenue"`
		RecentAnnouncements int64   `json:"recent_announcements"`
	}

	var stats DashboardStats

	err := h.cacheService.GetOrSet(
		ctx,
		fmt.Sprintf("dashboard:stats:%d:%s", propertyID, userRole),
		5*time.Minute, // Cache for 5 minutes
		func(ctx context.Context) (interface{}, error) {
			var s DashboardStats

			// Collect statistics from database
			// These are expensive queries that benefit from caching

			// Count total units
			h.db.WithContext(ctx).
				Model(&property.Unit{}).
				Where("property_id = ?", propertyID).
				Count(&s.TotalUnits)

			// Count occupied units
			h.db.WithContext(ctx).
				Model(&property.Unit{}).
				Where("property_id = ? AND status = ?", propertyID, "occupied").
				Count(&s.OccupiedUnits)

			// Count pending requests
			h.db.WithContext(ctx).
				Model(&property.Request{}).
				Where("property_id = ? AND status = ?", propertyID, "pending").
				Count(&s.PendingRequests)

			// Count unpaid bills
			h.db.WithContext(ctx).
				Model(&property.Bill{}).
				Where("property_id = ? AND status = ?", propertyID, "unpaid").
				Count(&s.UnpaidBills)

			// Calculate total revenue (paid bills)
			h.db.WithContext(ctx).
				Model(&property.Bill{}).
				Where("property_id = ? AND status = ?", propertyID, "paid").
				Select("COALESCE(SUM(total_amount), 0)").
				Scan(&s.TotalRevenue)

			// Count recent announcements (last 7 days)
			sevenDaysAgo := time.Now().AddDate(0, 0, -7)
			h.db.WithContext(ctx).
				Model(&property.Announcement{}).
				Where("property_id = ? AND created_at >= ?", propertyID, sevenDaysAgo).
				Count(&s.RecentAnnouncements)

			return s, nil
		},
		&stats,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   stats,
		"cached": true,
	})
}

// Example: Search with caching
type CachedSearchHandler struct {
	db           *gorm.DB
	cacheService *service.CacheService
}

// SearchUnitsWithCache demonstrates search result caching
func (h *CachedSearchHandler) SearchUnitsWithCache(c *gin.Context) {
	propertyID := c.GetUint("property_id")
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	ctx := c.Request.Context()

	var units []property.Unit

	err := h.cacheService.GetOrSet(
		ctx,
		fmt.Sprintf("search:units:%d:%s", propertyID, query),
		10*time.Minute,
		func(ctx context.Context) (interface{}, error) {
			var results []property.Unit

			err := h.db.WithContext(ctx).
				Where("property_id = ? AND (unit_number LIKE ? OR block LIKE ?)",
					propertyID,
					"%"+query+"%",
					"%"+query+"%").
				Order("unit_number ASC").
				Limit(50).
				Find(&results).Error

			if err != nil {
				return nil, err
			}

			return results, nil
		},
		&units,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  units,
		"query": query,
		"count": len(units),
	})
}

// Example: Rate limiting with cache
type RateLimitedHandler struct {
	cacheService *service.CacheService
}

// HandleWithRateLimit demonstrates rate limiting using cache
func (h *RateLimitedHandler) HandleWithRateLimit(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := c.Request.Context()

	// Check rate limit: 100 requests per minute
	identifier := fmt.Sprintf("user:%d", userID)
	allowed, remaining, err := h.cacheService.CheckRateLimit(
		ctx,
		identifier,
		100,         // 100 requests
		time.Minute, // per minute
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Rate limit check failed"})
		return
	}

	if !allowed {
		c.Header("X-RateLimit-Limit", "100")
		c.Header("X-RateLimit-Remaining", "0")
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "Rate limit exceeded",
			"retry_after": 60,
		})
		return
	}

	// Add rate limit headers
	c.Header("X-RateLimit-Limit", "100")
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

	// Process request
	c.JSON(http.StatusOK, gin.H{
		"message":            "Request processed",
		"remaining_requests": remaining,
	})
}

// Example: Session management with cache
type SessionHandler struct {
	cacheService *service.CacheService
}

// CreateSession demonstrates session creation
func (h *SessionHandler) CreateSession(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := c.Request.Context()

	sessionData := map[string]interface{}{
		"user_id":    userID,
		"created_at": time.Now(),
		"ip_address": c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}

	sessionID := fmt.Sprintf("%d_%d", userID, time.Now().Unix())

	err := h.cacheService.SetSession(
		ctx,
		sessionID,
		sessionData,
		24*time.Hour, // Session expires in 24 hours
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"expires_at": time.Now().Add(24 * time.Hour),
	})
}

// ValidateSession demonstrates session validation
func (h *SessionHandler) ValidateSession(c *gin.Context) {
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session ID required"})
		return
	}

	ctx := c.Request.Context()

	var sessionData map[string]interface{}
	err := h.cacheService.GetSession(ctx, sessionID, &sessionData)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
		return
	}

	// Refresh session expiration
	h.cacheService.RefreshSession(ctx, sessionID, 24*time.Hour)

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"data":  sessionData,
	})
}

// Example: Distributed lock for critical operations
type CriticalOperationHandler struct {
	db           *gorm.DB
	cacheService *service.CacheService
}

// ProcessPaymentWithLock demonstrates distributed locking
func (h *CriticalOperationHandler) ProcessPaymentWithLock(c *gin.Context) {
	billID := c.Param("bill_id")
	ctx := c.Request.Context()

	lockKey := fmt.Sprintf("payment:process:%s", billID)

	// Try to acquire lock
	acquired, err := h.cacheService.AcquireLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lock acquisition failed"})
		return
	}

	if !acquired {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Payment is being processed by another request",
		})
		return
	}

	// Ensure lock is released
	defer h.cacheService.ReleaseLock(ctx, lockKey)

	// Process payment (critical section)
	// This code is guaranteed to run exclusively
	var bill property.Bill
	if err := h.db.WithContext(ctx).First(&bill, billID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
		return
	}

	if bill.Status == "paid" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bill already paid"})
		return
	}

	// Update bill status
	bill.Status = "paid"
	bill.PaidAt = new(time.Time)
	*bill.PaidAt = time.Now()

	if err := h.db.WithContext(ctx).Save(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update bill"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment processed successfully",
		"bill":    bill,
	})
}
