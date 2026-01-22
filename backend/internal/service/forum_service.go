package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"homexai/internal/models/property"
	propertyRepo "homexai/internal/repository/property"

	"gorm.io/gorm"
)

const (
	MaxImagesPerUser = 20              // 每个用户最多20张图片
	MaxPinnedPosts   = 5               // 最多5个置顶帖子
	HardDeleteDays   = 15              // 15天后硬删除
	MaxImageSize     = 2 * 1024 * 1024 // 2MB
)

type ForumService struct {
	repo *propertyRepo.ForumRepository
}

func NewForumService(repo *propertyRepo.ForumRepository) *ForumService {
	return &ForumService{
		repo: repo,
	}
}

// CreatePost creates a new forum post
func (s *ForumService) CreatePost(db *gorm.DB, post *property.ForumPost) error {
	// Validate template data based on post type
	if err := s.validateTemplateData(post.PostType, post.TemplateData); err != nil {
		return err
	}

	// Check user image count
	imageCount, err := s.repo.GetUserImageCount(post.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user image count: %w", err)
	}
	if imageCount >= MaxImagesPerUser {
		return errors.New("user has reached the maximum image limit (20 images)")
	}

	return s.repo.CreatePost(post)
}

// ValidateTemplateData validates template data based on post type
func (s *ForumService) validateTemplateData(postType string, templateData property.JSONB) error {
	if templateData == nil {
		return errors.New("template data is required")
	}

	switch postType {
	case property.PostTypeVote:
		options, ok := templateData["options"].([]interface{})
		if !ok || len(options) == 0 {
			return errors.New("vote post requires at least one option")
		}
		deadlineStr, ok := templateData["deadline"].(string)
		if !ok || deadlineStr == "" {
			return errors.New("vote post requires a deadline")
		}
		deadline, err := time.Parse(time.RFC3339, deadlineStr)
		if err != nil {
			return errors.New("invalid deadline format")
		}
		if deadline.Before(time.Now()) {
			return errors.New("deadline must be in the future")
		}

	case property.PostTypeActivity:
		eventTimeStr, ok := templateData["event_time"].(string)
		if !ok || eventTimeStr == "" {
			return errors.New("activity post requires event_time")
		}
		_, err := time.Parse(time.RFC3339, eventTimeStr)
		if err != nil {
			return errors.New("invalid event_time format")
		}
		if _, ok := templateData["location"].(string); !ok {
			return errors.New("activity post requires location")
		}

	case property.PostTypeHelp:
		urgency, ok := templateData["urgency"].(string)
		if !ok || (urgency != "low" && urgency != "medium" && urgency != "high") {
			return errors.New("help post requires valid urgency (low/medium/high)")
		}

	case property.PostTypeMarketplace:
		if _, ok := templateData["price"].(string); !ok {
			return errors.New("marketplace post requires price")
		}
		condition, ok := templateData["condition"].(string)
		if !ok || (condition != "new" && condition != "like_new" && condition != "good" && condition != "fair") {
			return errors.New("marketplace post requires valid condition (new/like_new/good/fair)")
		}

	case property.PostTypeSocial:
		if _, ok := templateData["introduction"].(string); !ok {
			return errors.New("social post requires introduction")
		}

	case property.PostTypeRent:
		rentType, ok := templateData["rent_type"].(string)
		if !ok || (rentType != "unit" && rentType != "room" && rentType != "parking_slot") {
			return errors.New("rent post requires valid rent_type (unit/room/parking_slot)")
		}
		if _, ok := templateData["price"].(string); !ok {
			return errors.New("rent post requires price")
		}
		if _, ok := templateData["description"].(string); !ok {
			return errors.New("rent post requires description")
		}

	default:
		return fmt.Errorf("unknown post type: %s", postType)
	}

	return nil
}

// UpdatePost updates a forum post (only by owner)
func (s *ForumService) UpdatePost(post *property.ForumPost, userID uint) error {
	if post.UserID != userID {
		return errors.New("only post owner can edit the post")
	}

	if post.IsDeleted() {
		return errors.New("cannot edit deleted post")
	}

	if post.IsLocked {
		return errors.New("cannot edit locked post")
	}

	// Vote posts cannot be edited
	if post.PostType == property.PostTypeVote {
		return errors.New("vote posts cannot be edited")
	}

	// Validate template data if changed
	if err := s.validateTemplateData(post.PostType, post.TemplateData); err != nil {
		return err
	}

	// Mark as edited
	now := time.Now()
	post.IsEdited = true
	post.EditedAt = &now

	return s.repo.UpdatePost(post)
}

// ViewPost increments view count (only once per user)
func (s *ForumService) ViewPost(db *gorm.DB, postID, userID uint) error {
	// Check if user has already viewed this post
	hasViewed, err := s.repo.HasUserViewedPost(postID, userID)
	if err != nil {
		return fmt.Errorf("failed to check view log: %w", err)
	}

	if !hasViewed {
		// Create view log
		viewLog := &property.ForumViewLog{
			PostID:   postID,
			UserID:   userID,
			ViewedAt: time.Now(),
		}
		if err := s.repo.CreateViewLog(viewLog); err != nil {
			return fmt.Errorf("failed to create view log: %w", err)
		}

		// Increment view count
		if err := s.repo.IncrementViewCount(postID); err != nil {
			return fmt.Errorf("failed to increment view count: %w", err)
		}
	}

	return nil
}

// VoteOnPost votes on a post
func (s *ForumService) VoteOnPost(db *gorm.DB, postID, userID uint, selectedOptions []string) error {
	// Get post
	post, err := s.repo.FindPostByID(postID)
	if err != nil {
		return errors.New("post not found")
	}

	if post.PostType != property.PostTypeVote {
		return errors.New("only vote posts can be voted on")
	}

	if post.IsLocked {
		return errors.New("post is locked, voting is closed")
	}

	// Check if deadline has passed
	var templateData property.VoteTemplateData
	templateDataBytes, _ := json.Marshal(post.TemplateData)
	json.Unmarshal(templateDataBytes, &templateData)

	if templateData.Deadline != nil && time.Now().After(*templateData.Deadline) {
		// Auto-lock the post
		s.repo.LockPost(postID)
		return errors.New("voting deadline has passed")
	}

	// Check if user has already voted
	existingVote, err := s.repo.FindVoteByPostAndUser(postID, userID)
	if err == nil && existingVote != nil {
		return errors.New("you have already voted on this post")
	}

	// Validate selected options
	if len(selectedOptions) == 0 {
		return errors.New("at least one option must be selected")
	}

	allowMultiple := false
	if val, ok := post.TemplateData["allow_multiple"].(bool); ok {
		allowMultiple = val
	}

	if !allowMultiple && len(selectedOptions) > 1 {
		return errors.New("this vote only allows one option")
	}

	// Check if all selected options exist
	options, ok := post.TemplateData["options"].([]interface{})
	if !ok {
		return errors.New("invalid vote options")
	}
	optionMap := make(map[string]bool)
	for _, opt := range options {
		if optStr, ok := opt.(string); ok {
			optionMap[optStr] = true
		}
	}
	for _, selected := range selectedOptions {
		if !optionMap[selected] {
			return fmt.Errorf("invalid option: %s", selected)
		}
	}

	// Create vote record
	optionsJSON, _ := json.Marshal(selectedOptions)
	vote := &property.ForumVote{
		PostID:  postID,
		UserID:  userID,
		Options: string(optionsJSON),
	}

	return s.repo.CreateVote(vote)
}

// GetVoteResults gets vote results for a post
func (s *ForumService) GetVoteResults(db *gorm.DB, postID uint) ([]property.VoteResult, error) {
	post, err := s.repo.FindPostByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	if post.PostType != property.PostTypeVote {
		return nil, errors.New("only vote posts have vote results")
	}

	// Get all votes for this post
	var votes []property.ForumVote
	if err := db.Where("post_id = ?", postID).Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("failed to get votes: %w", err)
	}

	// Get options from template data
	options, ok := post.TemplateData["options"].([]interface{})
	if !ok {
		return nil, errors.New("invalid vote options")
	}

	// Count votes for each option
	resultMap := make(map[string]int64)
	for _, opt := range options {
		if optStr, ok := opt.(string); ok {
			resultMap[optStr] = 0
		}
	}

	for _, vote := range votes {
		var selectedOptions []string
		if err := json.Unmarshal([]byte(vote.Options), &selectedOptions); err != nil {
			continue
		}
		for _, opt := range selectedOptions {
			resultMap[opt]++
		}
	}

	// Convert to slice
	var results []property.VoteResult
	for option, count := range resultMap {
		results = append(results, property.VoteResult{
			Option: option,
			Count:  count,
		})
	}

	return results, nil
}

// PinPost pins a post (admin only)
func (s *ForumService) PinPost(postID uint) error {
	// Check pinned posts count
	count, err := s.repo.GetPinnedPostsCount()
	if err != nil {
		return fmt.Errorf("failed to get pinned posts count: %w", err)
	}

	if count >= MaxPinnedPosts {
		return fmt.Errorf("maximum number of pinned posts (%d) reached", MaxPinnedPosts)
	}

	return s.repo.PinPost(postID)
}

// UnpinPost unpins a post (admin only)
func (s *ForumService) UnpinPost(postID uint) error {
	return s.repo.UnpinPost(postID)
}

// SoftDeletePost soft deletes a post (admin only)
func (s *ForumService) SoftDeletePost(postID uint) error {
	return s.repo.SoftDeletePost(postID)
}

// HardDeleteOldPosts hard deletes posts that were soft deleted 15+ days ago
func (s *ForumService) HardDeleteOldPosts(db *gorm.DB) error {
	posts, err := s.repo.GetPostsReadyForHardDelete()
	if err != nil {
		return fmt.Errorf("failed to get posts ready for hard delete: %w", err)
	}

	for _, post := range posts {
		// Delete associated documents (images)
		if err := db.Where("entity_type = ? AND entity_id = ?",
			property.DocEntityForumPost, post.ID).
			Delete(&property.Document{}).Error; err != nil {
			return fmt.Errorf("failed to delete documents for post %d: %w", post.ID, err)
		}

		// Hard delete the post (this will cascade delete replies, votes, view logs)
		if err := s.repo.HardDeletePost(post.ID); err != nil {
			return fmt.Errorf("failed to hard delete post %d: %w", post.ID, err)
		}
	}

	return nil
}

// CheckAndLockExpiredVotes checks and locks expired vote posts
func (s *ForumService) CheckAndLockExpiredVotes(db *gorm.DB) error {
	// Get all unlocked vote posts
	var posts []property.ForumPost
	if err := db.Where("post_type = ? AND is_locked = ? AND deleted_at IS NULL",
		property.PostTypeVote, false).Find(&posts).Error; err != nil {
		return err
	}

	now := time.Now()
	for _, post := range posts {
		var templateData property.VoteTemplateData
		templateDataBytes, _ := json.Marshal(post.TemplateData)
		json.Unmarshal(templateDataBytes, &templateData)

		if templateData.Deadline != nil && now.After(*templateData.Deadline) {
			if err := s.repo.LockPost(post.ID); err != nil {
				return fmt.Errorf("failed to lock post %d: %w", post.ID, err)
			}
		}
	}

	return nil
}

// SoftDeleteReply soft deletes a reply (admin or owner)
func (s *ForumService) SoftDeleteReply(replyID, userID uint, isAdmin bool) error {
	reply, err := s.repo.FindReplyByID(replyID)
	if err != nil {
		return errors.New("reply not found")
	}

	if !isAdmin && reply.UserID != userID {
		return errors.New("only reply owner or admin can delete the reply")
	}

	return s.repo.SoftDeleteReply(replyID)
}

