package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	propertyRepo "homexai/internal/repository/property"
	"homexai/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ForumHandler handles forum-related requests
type ForumHandler struct {
	masterDB     *gorm.DB
	forumService *service.ForumService
	forumRepo    *propertyRepo.ForumRepository
}

// NewForumHandler creates a new ForumHandler
func NewForumHandler(masterDB *gorm.DB) *ForumHandler {
	forumRepo := propertyRepo.NewForumRepository(masterDB)
	forumService := service.NewForumService(forumRepo)
	return &ForumHandler{
		masterDB:     masterDB,
		forumService: forumService,
		forumRepo:    forumRepo,
	}
}

// CreatePostRequest represents the request body for creating a post
type CreatePostRequest struct {
	PostType     string                 `json:"post_type" binding:"required"`
	Title        string                 `json:"title" binding:"required,max=200"`
	Content      string                 `json:"content" binding:"required"`
	TemplateData map[string]interface{} `json:"template_data" binding:"required"`
}

// UpdatePostRequest represents the request body for updating a post
type UpdatePostRequest struct {
	Title        string                 `json:"title" binding:"max=200"`
	Content      string                 `json:"content"`
	TemplateData map[string]interface{} `json:"template_data"`
}

// CreateReplyRequest represents the request body for creating a reply
type CreateReplyRequest struct {
	Content string `json:"content" binding:"required"`
}

// VoteRequest represents the request body for voting
type VoteRequest struct {
	Options []string `json:"options" binding:"required"`
}

// PostResponse represents a post in the response
type PostResponse struct {
	ID           uint            `json:"id"`
	PostType     string          `json:"post_type"`
	Title        string          `json:"title"`
	Content      string          `json:"content"`
	TemplateData property.JSONB  `json:"template_data"`
	UserID       uint            `json:"user_id"`
	AuthorName   string          `json:"author_name"`
	ViewCount    int64           `json:"view_count"`
	ReplyCount   int64           `json:"reply_count"`
	IsPinned     bool            `json:"is_pinned"`
	PinnedAt     *time.Time      `json:"pinned_at"`
	IsLocked     bool            `json:"is_locked"`
	IsEdited     bool            `json:"is_edited"`
	EditedAt     *time.Time      `json:"edited_at"`
	Images       []ImageResponse `json:"images"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	IsAd         bool            `json:"is_ad"`         // true if this is an ad (B2B advertising)
	AdID         *uint           `json:"ad_id"`         // ad ID if this is an ad
}

// ReplyResponse represents a reply in the response
type ReplyResponse struct {
	ID         uint      `json:"id"`
	PostID     uint      `json:"post_id"`
	Content    string    `json:"content"`
	UserID     uint      `json:"user_id"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ImageResponse represents an image in the response
type ImageResponse struct {
	ID           uint   `json:"id"`
	DocumentName string `json:"document_name"`
	DocumentPath string `json:"document_path"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
}

// Create creates a new forum post (supports multipart form for image upload)
// @Summary Create a new forum post
// @Tags Forum
// @Accept json,multipart/form-data
// @Produce json
// @Param post_type formData string true "Post type (vote,activity,help,marketplace,social)"
// @Param title formData string true "Post title"
// @Param content formData string true "Post content"
// @Param template_data formData string true "Template data (JSON string)"
// @Param images formData file false "Images (max 20 per user, 2MB each)"
// @Success 201 {object} map[string]interface{}
// @Router /forum/posts [post]
func (h *ForumHandler) Create(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only authenticated users can create posts
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CreatePostRequest

	// Parse request based on content type
	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Multipart form
		req.PostType = c.PostForm("post_type")
		req.Title = c.PostForm("title")
		req.Content = c.PostForm("content")
		templateDataStr := c.PostForm("template_data")
		if templateDataStr != "" {
			if err := json.Unmarshal([]byte(templateDataStr), &req.TemplateData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template_data JSON"})
				return
			}
		}
	} else {
		// JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Validate post type
	validTypes := map[string]bool{
		property.PostTypeVote:        true,
		property.PostTypeActivity:    true,
		property.PostTypeHelp:        true,
		property.PostTypeMarketplace: true,
		property.PostTypeSocial:      true,
		property.PostTypeRent:        true,
	}
	if !validTypes[req.PostType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post_type"})
		return
	}

	// Create post
	post := &property.ForumPost{
		PostType:     req.PostType,
		Title:        req.Title,
		Content:      req.Content,
		TemplateData: property.JSONB(req.TemplateData),
		UserID:       userID,
		ViewCount:    0,
		ReplyCount:   0,
		IsPinned:     false,
		IsLocked:     false,
		IsEdited:     false,
	}

	// Use property database for repository operations
	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	if err := forumService.CreatePost(propertyDB, post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle image uploads if present (multipart form only)
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		if err := h.savePostImages(c, propertyDB, post.ID, userID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save images: %v\n", err)
		}
	}

	// Reload post with images
	post, _ = forumRepo.FindPostByID(post.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Post created successfully",
		"data":    h.buildPostResponse(propertyDB, post, userRole),
	})
}

// savePostImages saves uploaded images for a post
func (h *ForumHandler) savePostImages(c *gin.Context, db *gorm.DB, postID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	files := form.File["images"]
	if len(files) == 0 {
		return nil
	}

	// Check user's total image count
	forumRepo := propertyRepo.NewForumRepository(db)
	currentCount, err := forumRepo.GetUserImageCount(userID)
	if err != nil {
		return err
	}

	// Check if adding these files would exceed the limit
	if int(currentCount)+len(files) > service.MaxImagesPerUser {
		return fmt.Errorf("uploading %d images would exceed the limit of %d images per user", len(files), service.MaxImagesPerUser)
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/forum/posts/%d", postID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	maxSize := int64(service.MaxImageSize) // 2MB

	for _, file := range files {
		// Check file size
		if file.Size > maxSize {
			continue // Skip oversized files (should be handled by frontend, but double-check)
		}

		// Check file type
		contentType := file.Header.Get("Content-Type")
		if !allowedTypes[contentType] {
			continue
		}

		// Generate unique filename
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitizeFilename(file.Filename), ext)
		filePath := filepath.Join(uploadDir, filename)

		// Save file
		src, err := file.Open()
		if err != nil {
			continue
		}
		defer src.Close()

		dst, err := os.Create(filePath)
		if err != nil {
			continue
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			continue
		}

		// Create document record
		fileSize := file.Size
		doc := property.Document{
			EntityType:   property.DocEntityForumPost,
			EntityID:     postID,
			DocumentType: property.DocTypePhoto,
			DocumentName: file.Filename,
			DocumentPath: filePath,
			FileSize:     &fileSize,
			MimeType:     &contentType,
			UploadedBy:   userID,
			IsActive:     true,
		}

		if err := db.Create(&doc).Error; err != nil {
			continue
		}
	}

	return nil
}

// // sanitizeFilename removes special characters from filename
// func sanitizeFilename(name string) string {
// 	result := ""
// 	for _, r := range name {
// 		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
// 			result += string(r)
// 		} else if r == ' ' {
// 			result += "_"
// 		}
// 	}
// 	return result
// }

// Update updates a forum post (only by owner)
// @Summary Update a forum post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Param request body UpdatePostRequest true "Update request"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id} [put]
func (h *ForumHandler) Update(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	post, err := forumRepo.FindPostByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Check ownership
	if post.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only post owner can edit the post"})
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	if req.Title != "" {
		post.Title = req.Title
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.TemplateData != nil {
		post.TemplateData = property.JSONB(req.TemplateData)
	}

	forumService := service.NewForumService(forumRepo)
	if err := forumService.UpdatePost(post, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reload post
	post, _ = forumRepo.FindPostByID(post.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Post updated successfully",
		"data":    h.buildPostResponse(propertyDB, post, userRole),
	})
}

// List lists forum posts with pagination
// @Summary List forum posts
// @Tags Forum
// @Accept json
// @Produce json
// @Param post_type query string false "Filter by post type"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts [get]
func (h *ForumHandler) List(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userRole := middleware.GetUserRole(c)
	propertyID := middleware.GetPropertyID(c)

	postType := c.Query("post_type")
	topic := c.Query("topic")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	posts, total, err := forumRepo.ListPosts(postType, topic, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list posts"})
		return
	}

	// Get applicable ads and merge them into the post list
	var adResponses []PostResponse
	if propertyID > 0 {
		// Get property info from master DB
		var propertyInfo master.Property
		if err := h.masterDB.First(&propertyInfo, propertyID).Error; err == nil {
			// Get applicable ads
			forumAdHandler := NewForumAdHandler(h.masterDB)
			ads, err := forumAdHandler.GetApplicableAds(propertyID, propertyInfo.City, nil) // userTags not yet implemented
			if err == nil {
				// Convert ads to PostResponse and prepend them (ads are automatically pinned)
				for _, ad := range ads {
					adPost := forumAdHandler.ConvertAdToForumPost(&ad)
					adResponse := h.buildAdResponse(propertyDB, adPost, &ad, userRole)
					adResponses = append(adResponses, adResponse)
				}
			}
		}
	}

	var postResponses []PostResponse
	// Add ads first (they are automatically pinned/top)
	postResponses = append(postResponses, adResponses...)
	// Then add regular posts
	for _, post := range posts {
		postResponses = append(postResponses, h.buildPostResponse(propertyDB, &post, userRole))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": postResponses,
		"meta": gin.H{
			"page":     page,
			"per_page": perPage,
			"total":    total + int64(len(adResponses)), // Include ads in total
		},
	})
}

// Get gets a forum post by ID
// @Summary Get a forum post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id} [get]
func (h *ForumHandler) Get(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	post, err := forumRepo.FindPostByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Increment view count (only once per user)
	if userID > 0 {
		forumService.ViewPost(propertyDB, post.ID, userID)
		// Reload post to get updated view count
		post, _ = forumRepo.FindPostByID(post.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": h.buildPostResponse(propertyDB, post, userRole),
	})
}

// CreateReply creates a reply to a post
// @Summary Create a reply to a post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Param request body CreateReplyRequest true "Reply request"
// @Success 201 {object} map[string]interface{}
// @Router /forum/posts/{id}/replies [post]
func (h *ForumHandler) CreateReply(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	post, err := forumRepo.FindPostByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	if post.IsDeleted() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	var req CreateReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply := &property.ForumReply{
		PostID:  post.ID,
		Content: req.Content,
		UserID:  userID,
	}

	if err := forumRepo.CreateReply(reply); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reply"})
		return
	}

	// Increment reply count
	forumRepo.IncrementReplyCount(post.ID)

	// Get author name
	var authorName string
	var user master.User
	if err := h.masterDB.First(&user, userID).Error; err == nil {
		authorName = user.FullName
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Reply created successfully",
		"data": ReplyResponse{
			ID:         reply.ID,
			PostID:     reply.PostID,
			Content:    reply.Content,
			UserID:     reply.UserID,
			AuthorName: authorName,
			CreatedAt:  reply.CreatedAt,
			UpdatedAt:  reply.UpdatedAt,
		},
	})
}

// ListReplies lists replies for a post
// @Summary List replies for a post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id}/replies [get]
func (h *ForumHandler) ListReplies(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	replies, total, err := forumRepo.ListReplies(uint(id), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list replies"})
		return
	}

	var replyResponses []ReplyResponse
	for _, reply := range replies {
		var authorName string
		var user master.User
		if err := h.masterDB.First(&user, reply.UserID).Error; err == nil {
			authorName = user.FullName
		}

		replyResponses = append(replyResponses, ReplyResponse{
			ID:         reply.ID,
			PostID:     reply.PostID,
			Content:    reply.Content,
			UserID:     reply.UserID,
			AuthorName: authorName,
			CreatedAt:  reply.CreatedAt,
			UpdatedAt:  reply.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": replyResponses,
		"meta": gin.H{
			"page":     page,
			"per_page": perPage,
			"total":    total,
		},
	})
}

// Vote votes on a post
// @Summary Vote on a vote post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Param request body VoteRequest true "Vote request"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id}/vote [post]
func (h *ForumHandler) Vote(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	var req VoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	if err := forumService.VoteOnPost(propertyDB, uint(id), userID, req.Options); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Vote submitted successfully",
	})
}

// GetVoteResults gets vote results for a post
// @Summary Get vote results for a post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id}/vote-results [get]
func (h *ForumHandler) GetVoteResults(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	results, err := forumService.GetVoteResults(propertyDB, uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user's vote if exists
	var hasVoted bool
	var userVote interface{}
	if userID > 0 {
		vote, err := forumRepo.FindVoteByPostAndUser(uint(id), userID)
		if err == nil && vote != nil {
			hasVoted = true
			var selectedOptions []string
			if err := json.Unmarshal([]byte(vote.Options), &selectedOptions); err == nil {
				userVote = selectedOptions
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"results":   results,
			"has_voted": hasVoted,
			"user_vote": userVote,
		},
	})
}

// PinPost pins a post (admin only)
// @Summary Pin a post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id}/pin [post]
func (h *ForumHandler) PinPost(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	if err := forumService.PinPost(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Post pinned successfully",
	})
}

// UnpinPost unpins a post (admin only)
// @Summary Unpin a post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id}/unpin [post]
func (h *ForumHandler) UnpinPost(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	if err := forumService.UnpinPost(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Post unpinned successfully",
	})
}

// DeletePost deletes a post (admin only)
// @Summary Delete a post
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Success 200 {object} map[string]interface{}
// @Router /forum/posts/{id} [delete]
func (h *ForumHandler) DeletePost(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	forumService := service.NewForumService(forumRepo)

	if err := forumService.SoftDeletePost(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Post deleted successfully",
	})
}

// DeleteReply deletes a reply (admin or owner)
// @Summary Delete a reply
// @Tags Forum
// @Accept json
// @Produce json
// @Param id path int true "Reply ID"
// @Success 200 {object} map[string]interface{}
// @Router /forum/replies/{id} [delete]
func (h *ForumHandler) DeleteReply(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reply ID"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	reply, err := forumRepo.FindReplyByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reply not found"})
		return
	}

	isAdmin := userRole == "super_admin" || userRole == "property_admin"
	forumService := service.NewForumService(forumRepo)

	if err := forumService.SoftDeleteReply(uint(id), userID, isAdmin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Decrement reply count
	forumRepo.DecrementReplyCount(reply.PostID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Reply deleted successfully",
	})
}

// GetUserImageCount gets the current image count for the authenticated user
// @Summary Get user image count
// @Tags Forum
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /forum/user/image-count [get]
func (h *ForumHandler) GetUserImageCount(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	forumRepo := propertyRepo.NewForumRepository(propertyDB)
	count, err := forumRepo.GetUserImageCount(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get image count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"count":       count,
			"max_allowed": service.MaxImagesPerUser,
		},
	})
}

// buildPostResponse builds a PostResponse from a ForumPost
func (h *ForumHandler) buildPostResponse(db *gorm.DB, post *property.ForumPost, userRole string) PostResponse {
	// Get author name
	var authorName string
	var user master.User
	if err := h.masterDB.First(&user, post.UserID).Error; err == nil {
		authorName = user.FullName
	}

	// Get images
	var images []ImageResponse
	var documents []property.Document
	db.Where("entity_type = ? AND entity_id = ? AND is_active = ?",
		property.DocEntityForumPost, post.ID, true).
		Find(&documents)

	for _, doc := range documents {
		var fileSize int64
		if doc.FileSize != nil {
			fileSize = *doc.FileSize
		}
		var mimeType string
		if doc.MimeType != nil {
			mimeType = *doc.MimeType
		}
		images = append(images, ImageResponse{
			ID:           doc.ID,
			DocumentName: doc.DocumentName,
			DocumentPath: doc.DocumentPath,
			FileSize:     fileSize,
			MimeType:     mimeType,
		})
	}

	return PostResponse{
		ID:           post.ID,
		PostType:     post.PostType,
		Title:        post.Title,
		Content:      post.Content,
		TemplateData: post.TemplateData,
		UserID:       post.UserID,
		AuthorName:   authorName,
		ViewCount:    post.ViewCount,
		ReplyCount:   post.ReplyCount,
		IsPinned:     post.IsPinned,
		PinnedAt:     post.PinnedAt,
		IsLocked:     post.IsLocked,
		IsEdited:     post.IsEdited,
		EditedAt:     post.EditedAt,
		Images:       images,
		CreatedAt:    post.CreatedAt,
		UpdatedAt:    post.UpdatedAt,
		IsAd:         false,
		AdID:         nil,
	}
}

// buildAdResponse builds a PostResponse from an ad (ForumAd converted to ForumPost)
func (h *ForumHandler) buildAdResponse(db *gorm.DB, post *property.ForumPost, ad *master.ForumAd, userRole string) PostResponse {
	// Get author name (super_admin who created the ad)
	var authorName string
	var user master.User
	if err := h.masterDB.First(&user, ad.CreatedBy).Error; err == nil {
		authorName = user.FullName
	}

	// Ads don't have images
	return PostResponse{
		ID:           post.ID, // This will be 0 for virtual posts, but we use AdID for reference
		PostType:     post.PostType,
		Title:        post.Title,
		Content:      post.Content,
		TemplateData: property.JSONB(nil), // Ads don't have template data
		UserID:       post.UserID,
		AuthorName:   authorName,
		ViewCount:    0, // Ads don't track views
		ReplyCount:   0, // Ads cannot be replied to
		IsPinned:     true, // Ads are always pinned
		PinnedAt:     post.PinnedAt,
		IsLocked:     true, // Ads are locked (cannot be replied to)
		IsEdited:     false,
		EditedAt:     nil,
		Images:       []ImageResponse{}, // Ads don't have images
		CreatedAt:    post.CreatedAt,
		UpdatedAt:    post.UpdatedAt,
		IsAd:         true,
		AdID:         &ad.ID,
	}
}
