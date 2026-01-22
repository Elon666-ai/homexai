package property

import (
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type ForumRepository struct {
	db *gorm.DB
}

func NewForumRepository(db *gorm.DB) *ForumRepository {
	return &ForumRepository{db: db}
}

// CreatePost creates a new forum post
func (r *ForumRepository) CreatePost(post *property.ForumPost) error {
	return r.db.Create(post).Error
}

// FindPostByID finds a post by ID (excluding deleted)
func (r *ForumRepository) FindPostByID(id uint) (*property.ForumPost, error) {
	var post property.ForumPost
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// FindPostByIDWithDeleted finds a post by ID (including deleted, for admin)
func (r *ForumRepository) FindPostByIDWithDeleted(id uint) (*property.ForumPost, error) {
	var post property.ForumPost
	err := r.db.First(&post, id).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// UpdatePost updates a post
func (r *ForumRepository) UpdatePost(post *property.ForumPost) error {
	return r.db.Save(post).Error
}

// UpdatePostFields updates specific fields of a post
func (r *ForumRepository) UpdatePostFields(id uint, fields map[string]interface{}) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).Updates(fields).Error
}

// SoftDeletePost soft deletes a post (sets deleted_at)
func (r *ForumRepository) SoftDeletePost(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

// HardDeletePost permanently deletes a post and its related data
func (r *ForumRepository) HardDeletePost(id uint) error {
	// Delete replies first (CASCADE will handle this, but we do it explicitly for clarity)
	if err := r.db.Where("post_id = ?", id).Delete(&property.ForumReply{}).Error; err != nil {
		return err
	}
	// Delete votes
	if err := r.db.Where("post_id = ?", id).Delete(&property.ForumVote{}).Error; err != nil {
		return err
	}
	// Delete view logs
	if err := r.db.Where("post_id = ?", id).Delete(&property.ForumViewLog{}).Error; err != nil {
		return err
	}
	// Delete the post
	return r.db.Delete(&property.ForumPost{}, id).Error
}

// ListPosts lists posts with pagination and filters
// Sort by view_count DESC, then created_at DESC
func (r *ForumRepository) ListPosts(postType string, topic string, page, perPage int) ([]property.ForumPost, int64, error) {
	var posts []property.ForumPost
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.ForumPost{}).Where("deleted_at IS NULL")

	if postType != "" {
		query = query.Where("post_type = ?", postType)
	}

	if topic != "" {
		// Fuzzy search on title field
		searchPattern := "%" + topic + "%"
		query = query.Where("title LIKE ?", searchPattern)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Sort: pinned posts first (by pinned_at DESC), then by view_count DESC, then created_at DESC
	err = query.Order("is_pinned DESC, pinned_at DESC, view_count DESC, created_at DESC").
		Offset(offset).Limit(perPage).Find(&posts).Error
	return posts, total, err
}

// GetPinnedPostsCount gets the count of pinned posts
func (r *ForumRepository) GetPinnedPostsCount() (int64, error) {
	var count int64
	err := r.db.Model(&property.ForumPost{}).
		Where("is_pinned = ? AND deleted_at IS NULL", true).
		Count(&count).Error
	return count, err
}

// IncrementViewCount increments the view count of a post
func (r *ForumRepository) IncrementViewCount(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

// IncrementReplyCount increments the reply count of a post
func (r *ForumRepository) IncrementReplyCount(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).
		UpdateColumn("reply_count", gorm.Expr("reply_count + ?", 1)).Error
}

// DecrementReplyCount decrements the reply count of a post
func (r *ForumRepository) DecrementReplyCount(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ? AND reply_count > 0", id).
		UpdateColumn("reply_count", gorm.Expr("reply_count - ?", 1)).Error
}

// PinPost pins a post
func (r *ForumRepository) PinPost(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_pinned": true,
			"pinned_at": gorm.Expr("NOW()"),
		}).Error
}

// UnpinPost unpins a post
func (r *ForumRepository) UnpinPost(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_pinned": false,
			"pinned_at": nil,
		}).Error
}

// LockPost locks a post
func (r *ForumRepository) LockPost(id uint) error {
	return r.db.Model(&property.ForumPost{}).Where("id = ?", id).
		Update("is_locked", true).Error
}

// GetPostsReadyForHardDelete gets posts that were soft deleted 15+ days ago
func (r *ForumRepository) GetPostsReadyForHardDelete() ([]property.ForumPost, error) {
	var posts []property.ForumPost
	err := r.db.Where("deleted_at IS NOT NULL AND deleted_at <= DATE_SUB(NOW(), INTERVAL 15 DAY)").
		Find(&posts).Error
	return posts, err
}

// CreateReply creates a new reply
func (r *ForumRepository) CreateReply(reply *property.ForumReply) error {
	return r.db.Create(reply).Error
}

// FindReplyByID finds a reply by ID (excluding deleted)
func (r *ForumRepository) FindReplyByID(id uint) (*property.ForumReply, error) {
	var reply property.ForumReply
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&reply).Error
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

// ListReplies lists replies for a post
func (r *ForumRepository) ListReplies(postID uint, page, perPage int) ([]property.ForumReply, int64, error) {
	var replies []property.ForumReply
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.ForumReply{}).
		Where("post_id = ? AND deleted_at IS NULL", postID)

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("created_at ASC").
		Offset(offset).Limit(perPage).Find(&replies).Error
	return replies, total, err
}

// SoftDeleteReply soft deletes a reply
func (r *ForumRepository) SoftDeleteReply(id uint) error {
	return r.db.Model(&property.ForumReply{}).Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

// HardDeleteReply permanently deletes a reply
func (r *ForumRepository) HardDeleteReply(id uint) error {
	return r.db.Delete(&property.ForumReply{}, id).Error
}

// CreateViewLog creates a view log entry
func (r *ForumRepository) CreateViewLog(log *property.ForumViewLog) error {
	return r.db.Create(log).Error
}

// HasUserViewedPost checks if a user has viewed a post
func (r *ForumRepository) HasUserViewedPost(postID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&property.ForumViewLog{}).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Count(&count).Error
	return count > 0, err
}

// CreateVote creates a vote record
func (r *ForumRepository) CreateVote(vote *property.ForumVote) error {
	return r.db.Create(vote).Error
}

// FindVoteByPostAndUser finds a vote by post and user
func (r *ForumRepository) FindVoteByPostAndUser(postID, userID uint) (*property.ForumVote, error) {
	var vote property.ForumVote
	err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&vote).Error
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

// GetVoteCountsByPost gets vote counts for each option in a post
func (r *ForumRepository) GetVoteCountsByPost(postID uint) (map[string]int64, error) {
	var votes []property.ForumVote
	err := r.db.Where("post_id = ?", postID).Find(&votes).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, vote := range votes {
		// Parse the JSON options string and count each option
		// For simplicity, we'll count based on the options field
		// This requires parsing the JSON in the service layer
		_ = vote
	}
	return counts, nil
}

// GetUserImageCount gets the count of images uploaded by a user
func (r *ForumRepository) GetUserImageCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&property.Document{}).
		Where("entity_type = ? AND uploaded_by = ? AND is_active = ?",
			property.DocEntityForumPost, userID, true).
		Count(&count).Error
	return count, err
}


