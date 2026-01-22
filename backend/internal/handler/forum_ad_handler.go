package handler

import (
	"net/http"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ForumAdHandler handles forum ad management (B2B system)
type ForumAdHandler struct {
	masterDB *gorm.DB
}

// NewForumAdHandler creates a new ForumAdHandler
func NewForumAdHandler(masterDB *gorm.DB) *ForumAdHandler {
	return &ForumAdHandler{
		masterDB: masterDB,
	}
}

// CreateForumAdRequest represents the request body for creating a forum ad
type CreateForumAdRequest struct {
	AdType           string   `json:"ad_type"` // Optional, defaults to "general"
	Title            string   `json:"title" binding:"required,max=200"`
	Content          string   `json:"content" binding:"required"`
	Status           string   `json:"status"` // Optional, defaults to "inactive"
	TargetProperties []uint   `json:"target_properties"` // Property IDs, empty means all
	TargetCities     []string `json:"target_cities"`     // City names, empty means all
	TargetTags       []string `json:"target_tags"`       // User tags, empty means all
}

// UpdateForumAdRequest represents the request body for updating a forum ad
type UpdateForumAdRequest struct {
	AdType           string   `json:"ad_type"`
	Title            string   `json:"title" binding:"omitempty,max=200"`
	Content          string   `json:"content"`
	TargetProperties []uint   `json:"target_properties"`
	TargetCities     []string `json:"target_cities"`
	TargetTags       []string `json:"target_tags"`
}

// ForumAdResponse represents a forum ad in the response
type ForumAdResponse struct {
	ID               uint       `json:"id"`
	AdType           string     `json:"ad_type"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	Status           string     `json:"status"`
	TargetProperties []uint     `json:"target_properties"`
	TargetCities     []string   `json:"target_cities"`
	TargetTags       []string   `json:"target_tags"`
	CreatedBy        uint       `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	StartedAt        *time.Time `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
}

// List lists forum ads with pagination and filters
func (h *ForumAdHandler) List(c *gin.Context) {
	status := c.Query("status")
	keyword := c.Query("keyword")
	dateStr := c.Query("date")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage
	query := h.masterDB.Model(&master.ForumAd{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		searchTerm := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", searchTerm, searchTerm)
	}
	if dateStr != "" {
		// Parse date and filter by created_at date
		query = query.Where("DATE(created_at) = ?", dateStr)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count ads"})
		return
	}

	var ads []master.ForumAd
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&ads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list ads"})
		return
	}

	var adResponses []ForumAdResponse
	for _, ad := range ads {
		adResponses = append(adResponses, h.buildAdResponse(&ad))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": adResponses,
		"meta": gin.H{
			"page":     page,
			"per_page": perPage,
			"total":    total,
		},
	})
}

// Get gets a forum ad by ID
func (h *ForumAdHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad ID"})
		return
	}

	var ad master.ForumAd
	if err := h.masterDB.First(&ad, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ad not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ad"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": h.buildAdResponse(&ad),
	})
}

// Create creates a new forum ad
func (h *ForumAdHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req CreateForumAdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert slices to JSONB
	var targetPropertiesJSONB master.JSONB
	if len(req.TargetProperties) > 0 {
		targetPropertiesJSONB = make(master.JSONB, len(req.TargetProperties))
		for i, v := range req.TargetProperties {
			targetPropertiesJSONB[i] = v
		}
	}

	var targetCitiesJSONB master.JSONB
	if len(req.TargetCities) > 0 {
		targetCitiesJSONB = make(master.JSONB, len(req.TargetCities))
		for i, v := range req.TargetCities {
			targetCitiesJSONB[i] = v
		}
	}

	var targetTagsJSONB master.JSONB
	if len(req.TargetTags) > 0 {
		targetTagsJSONB = make(master.JSONB, len(req.TargetTags))
		for i, v := range req.TargetTags {
			targetTagsJSONB[i] = v
		}
	}

	// Set default AdType if not provided
	adType := req.AdType
	if adType == "" {
		adType = "general"
	}

	// Set default Status if not provided
	status := req.Status
	if status == "" {
		status = master.ForumAdStatusInactive
	}

	ad := master.ForumAd{
		AdType:           adType,
		Title:            req.Title,
		Content:          req.Content,
		Status:           status,
		TargetProperties: targetPropertiesJSONB,
		TargetCities:     targetCitiesJSONB,
		TargetTags:       targetTagsJSONB,
		CreatedBy:        userID,
	}

	if err := h.masterDB.Create(&ad).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ad"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Ad created successfully",
		"data":    h.buildAdResponse(&ad),
	})
}

// Update updates a forum ad
func (h *ForumAdHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad ID"})
		return
	}

	var ad master.ForumAd
	if err := h.masterDB.First(&ad, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ad not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ad"})
		return
	}

	var req UpdateForumAdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	if req.AdType != "" {
		ad.AdType = req.AdType
	}
	if req.Title != "" {
		ad.Title = req.Title
	}
	if req.Content != "" {
		ad.Content = req.Content
	}

	// Update target fields if provided
	if req.TargetProperties != nil {
		if len(req.TargetProperties) > 0 {
			targetPropertiesJSONB := make(master.JSONB, len(req.TargetProperties))
			for i, v := range req.TargetProperties {
				targetPropertiesJSONB[i] = v
			}
			ad.TargetProperties = targetPropertiesJSONB
		} else {
			ad.TargetProperties = nil
		}
	}
	if req.TargetCities != nil {
		if len(req.TargetCities) > 0 {
			targetCitiesJSONB := make(master.JSONB, len(req.TargetCities))
			for i, v := range req.TargetCities {
				targetCitiesJSONB[i] = v
			}
			ad.TargetCities = targetCitiesJSONB
		} else {
			ad.TargetCities = nil
		}
	}
	if req.TargetTags != nil {
		if len(req.TargetTags) > 0 {
			targetTagsJSONB := make(master.JSONB, len(req.TargetTags))
			for i, v := range req.TargetTags {
				targetTagsJSONB[i] = v
			}
			ad.TargetTags = targetTagsJSONB
		} else {
			ad.TargetTags = nil
		}
	}

	if err := h.masterDB.Save(&ad).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ad"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ad updated successfully",
		"data":    h.buildAdResponse(&ad),
	})
}

// Activate activates a forum ad (上架)
func (h *ForumAdHandler) Activate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad ID"})
		return
	}

	var ad master.ForumAd
	if err := h.masterDB.First(&ad, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ad not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ad"})
		return
	}

	now := time.Now()
	ad.Status = master.ForumAdStatusActive
	ad.StartedAt = &now
	ad.EndedAt = nil

	if err := h.masterDB.Save(&ad).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate ad"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ad activated successfully",
		"data":    h.buildAdResponse(&ad),
	})
}

// Deactivate deactivates a forum ad (下架)
func (h *ForumAdHandler) Deactivate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad ID"})
		return
	}

	var ad master.ForumAd
	if err := h.masterDB.First(&ad, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ad not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ad"})
		return
	}

	now := time.Now()
	ad.Status = master.ForumAdStatusInactive
	ad.EndedAt = &now

	if err := h.masterDB.Save(&ad).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate ad"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ad deactivated successfully",
		"data":    h.buildAdResponse(&ad),
	})
}

// Delete deletes a forum ad
func (h *ForumAdHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad ID"})
		return
	}

	if err := h.masterDB.Delete(&master.ForumAd{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete ad"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ad deleted successfully",
	})
}

// buildAdResponse builds a ForumAdResponse from a ForumAd
func (h *ForumAdHandler) buildAdResponse(ad *master.ForumAd) ForumAdResponse {
	resp := ForumAdResponse{
		ID:        ad.ID,
		AdType:    ad.AdType,
		Title:     ad.Title,
		Content:   ad.Content,
		Status:    ad.Status,
		CreatedBy: ad.CreatedBy,
		CreatedAt: ad.CreatedAt,
		UpdatedAt: ad.UpdatedAt,
		StartedAt: ad.StartedAt,
		EndedAt:   ad.EndedAt,
	}

	// Convert JSONB to slices
	if ad.TargetProperties != nil {
		resp.TargetProperties = make([]uint, 0, len(ad.TargetProperties))
		for _, v := range ad.TargetProperties {
			if num, ok := v.(float64); ok {
				resp.TargetProperties = append(resp.TargetProperties, uint(num))
			}
		}
	}
	if ad.TargetCities != nil {
		resp.TargetCities = make([]string, 0, len(ad.TargetCities))
		for _, v := range ad.TargetCities {
			if str, ok := v.(string); ok {
				resp.TargetCities = append(resp.TargetCities, str)
			}
		}
	}
	if ad.TargetTags != nil {
		resp.TargetTags = make([]string, 0, len(ad.TargetTags))
		for _, v := range ad.TargetTags {
			if str, ok := v.(string); ok {
				resp.TargetTags = append(resp.TargetTags, str)
			}
		}
	}

	return resp
}

// GetApplicableAds gets ads that are applicable to a property
// This is used by ForumHandler to merge ads into the post list
func (h *ForumAdHandler) GetApplicableAds(propertyID uint, propertyCity *string, userTags []string) ([]master.ForumAd, error) {
	var ads []master.ForumAd

	query := h.masterDB.Model(&master.ForumAd{}).
		Where("status = ?", master.ForumAdStatusActive)

	// Filter by property ID, city, or tags
	// We need to check JSONB fields, which is complex in SQL
	// For now, we'll get all active ads and filter in application code
	// A more efficient approach would use JSON_CONTAINS in MySQL, but let's keep it simple for now

	if err := query.Find(&ads).Error; err != nil {
		return nil, err
	}

	// Filter ads that match this property
	var applicableAds []master.ForumAd
	for _, ad := range ads {
		if h.isAdApplicable(&ad, propertyID, propertyCity, userTags) {
			applicableAds = append(applicableAds, ad)
		}
	}

	return applicableAds, nil
}

// isAdApplicable checks if an ad is applicable to a property/user
func (h *ForumAdHandler) isAdApplicable(ad *master.ForumAd, propertyID uint, propertyCity *string, userTags []string) bool {
	// If no filters are set, ad applies to all
	hasPropertyFilter := ad.TargetProperties != nil && len(ad.TargetProperties) > 0
	hasCityFilter := ad.TargetCities != nil && len(ad.TargetCities) > 0
	hasTagFilter := ad.TargetTags != nil && len(ad.TargetTags) > 0

	if !hasPropertyFilter && !hasCityFilter && !hasTagFilter {
		return true // No filters, applies to all
	}

	// Check property ID filter
	if hasPropertyFilter {
		matches := false
		for _, v := range ad.TargetProperties {
			var targetID uint
			if num, ok := v.(float64); ok {
				targetID = uint(num)
			} else if num, ok := v.(uint); ok {
				targetID = num
			}
			if targetID == propertyID {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	// Check city filter
	if hasCityFilter && propertyCity != nil {
		matches := false
		cityStr := *propertyCity
		for _, v := range ad.TargetCities {
			if str, ok := v.(string); ok && str == cityStr {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	// Check tag filter (for future use - user tags not yet implemented)
	// For now, if tag filter is set but userTags is empty, skip this ad
	if hasTagFilter && len(userTags) == 0 {
		return false
	}
	if hasTagFilter && len(userTags) > 0 {
		matches := false
		for _, adTag := range ad.TargetTags {
			if str, ok := adTag.(string); ok {
				for _, userTag := range userTags {
					if str == userTag {
						matches = true
						break
					}
				}
				if matches {
					break
				}
			}
		}
		if !matches {
			return false
		}
	}

	return true
}

// ConvertAdToForumPost converts a ForumAd to a ForumPost for display
func (h *ForumAdHandler) ConvertAdToForumPost(ad *master.ForumAd) *property.ForumPost {
	now := time.Now()
	return &property.ForumPost{
		ID:        0, // Virtual post, no real ID
		PostType:  ad.AdType,
		Title:     ad.Title,
		Content:   ad.Content,
		UserID:    ad.CreatedBy,
		IsPinned:  true, // Ads are always pinned
		PinnedAt:  &now,
		CreatedAt: ad.CreatedAt,
		UpdatedAt: ad.UpdatedAt,
	}
}

