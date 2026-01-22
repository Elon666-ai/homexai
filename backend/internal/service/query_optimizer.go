package service

import (
	"fmt"
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

// QueryOptimizer provides optimized query helpers to avoid N+1 problems
type QueryOptimizer struct {
	db *gorm.DB
}

// NewQueryOptimizer creates a new QueryOptimizer instance
func NewQueryOptimizer(db *gorm.DB) *QueryOptimizer {
	return &QueryOptimizer{db: db}
}

// ============================================================================
// REQUEST QUERIES WITH PRELOAD
// ============================================================================

// GetRequestsWithRelations retrieves requests with all related data preloaded
func (q *QueryOptimizer) GetRequestsWithRelations(propertyID uint, conditions map[string]interface{}) ([]property.Request, error) {
	var requests []property.Request
	
	query := q.db.Where("property_id = ?", propertyID)
	
	// Apply additional conditions
	for key, value := range conditions {
		query = query.Where(key, value)
	}
	
	err := query.
		Preload("Requester").                    // User who created the request
		Preload("AssignedToUser").               // User assigned to handle the request
		Preload("Attachments").                  // File attachments
		Preload("Traces", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")   // Order traces by creation time
		}).
		Preload("Traces.User").                  // User who created each trace
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")    // Order comments chronologically
		}).
		Preload("Comments.User").                // User who created each comment
		Order("created_at DESC").
		Find(&requests).Error
	
	return requests, err
}

// GetRequestByIDWithRelations retrieves a single request with all relations
func (q *QueryOptimizer) GetRequestByIDWithRelations(propertyID uint, requestID uint) (*property.Request, error) {
	var request property.Request
	
	err := q.db.
		Where("property_id = ? AND id = ?", propertyID, requestID).
		Preload("Requester").
		Preload("AssignedToUser").
		Preload("Attachments").
		Preload("Traces", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Traces.User").
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Comments.User").
		First(&request).Error
	
	if err != nil {
		return nil, err
	}
	
	return &request, nil
}

// ============================================================================
// BILL QUERIES WITH PRELOAD
// ============================================================================

// GetBillsWithRelations retrieves bills with related data preloaded
func (q *QueryOptimizer) GetBillsWithRelations(propertyID uint, conditions map[string]interface{}) ([]property.Bill, error) {
	var bills []property.Bill
	
	query := q.db.Where("property_id = ?", propertyID)
	
	// Apply additional conditions
	for key, value := range conditions {
		query = query.Where(key, value)
	}
	
	err := query.
		Preload("Items").                        // Bill items
		Preload("Payments").                     // Payment records
		Preload("Unit").                         // Associated unit
		Preload("Unit.Landlord").                // Landlord of the unit
		Preload("Unit.Tenant").                  // Tenant of the unit
		Order("billing_period DESC").
		Find(&bills).Error
	
	return bills, err
}

// GetBillByIDWithRelations retrieves a single bill with all relations
func (q *QueryOptimizer) GetBillByIDWithRelations(propertyID uint, billID uint) (*property.Bill, error) {
	var bill property.Bill
	
	err := q.db.
		Where("property_id = ? AND id = ?", propertyID, billID).
		Preload("Items").
		Preload("Payments").
		Preload("Unit").
		Preload("Unit.Landlord").
		Preload("Unit.Tenant").
		First(&bill).Error
	
	if err != nil {
		return nil, err
	}
	
	return &bill, nil
}

// ============================================================================
// VISITOR QUERIES WITH PRELOAD
// ============================================================================

// GetVisitorsWithRelations retrieves visitors with related data preloaded
func (q *QueryOptimizer) GetVisitorsWithRelations(propertyID uint, conditions map[string]interface{}) ([]property.VisitorRegistration, error) {
	var visitors []property.VisitorRegistration
	
	query := q.db.Where("property_id = ?", propertyID)
	
	// Apply additional conditions
	for key, value := range conditions {
		query = query.Where(key, value)
	}
	
	err := query.
		Preload("Unit").                         // Associated unit
		Preload("RegisteredBy").                 // User who registered the visitor
		Preload("ApprovedBy").                   // User who approved (if applicable)
		Preload("CheckedInBy").                  // Security staff who checked in
		Order("visit_date DESC").
		Find(&visitors).Error
	
	return visitors, err
}

// ============================================================================
// ANNOUNCEMENT QUERIES WITH PRELOAD
// ============================================================================

// GetAnnouncementsWithRelations retrieves announcements with related data
func (q *QueryOptimizer) GetAnnouncementsWithRelations(propertyID uint, conditions map[string]interface{}) ([]property.Announcement, error) {
	var announcements []property.Announcement
	
	query := q.db.Where("property_id = ?", propertyID)
	
	// Apply additional conditions
	for key, value := range conditions {
		query = query.Where(key, value)
	}
	
	err := query.
		Preload("CreatedByUser").                // User who created the announcement
		Preload("Attachments").                  // File attachments
		Order("created_at DESC").
		Find(&announcements).Error
	
	return announcements, err
}

// ============================================================================
// FORUM POST QUERIES WITH PRELOAD
// ============================================================================

// GetForumPostsWithRelations retrieves forum posts with related data
func (q *QueryOptimizer) GetForumPostsWithRelations(propertyID uint, conditions map[string]interface{}) ([]property.ForumPost, error) {
	var posts []property.ForumPost
	
	query := q.db.Where("property_id = ?", propertyID)
	
	// Apply additional conditions
	for key, value := range conditions {
		query = query.Where(key, value)
	}
	
	err := query.
		Preload("Author").                       // Post author
		Preload("Attachments").                  // File attachments
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Limit(5) // Limit initial replies for performance
		}).
		Preload("Replies.Author").               // Reply authors
		Order("created_at DESC").
		Find(&posts).Error
	
	return posts, err
}

// GetForumPostByIDWithAllReplies retrieves a post with all replies (for detail view)
func (q *QueryOptimizer) GetForumPostByIDWithAllReplies(propertyID uint, postID uint) (*property.ForumPost, error) {
	var post property.ForumPost
	
	err := q.db.
		Where("property_id = ? AND id = ?", propertyID, postID).
		Preload("Author").
		Preload("Attachments").
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC") // All replies, ordered
		}).
		Preload("Replies.Author").
		First(&post).Error
	
	if err != nil {
		return nil, err
	}
	
	return &post, nil
}

// ============================================================================
// UNIT QUERIES WITH PRELOAD
// ============================================================================

// GetUnitsWithRelations retrieves units with related data preloaded
func (q *QueryOptimizer) GetUnitsWithRelations(propertyID uint, conditions map[string]interface{}) ([]property.Unit, error) {
	var units []property.Unit
	
	query := q.db.Where("property_id = ?", propertyID)
	
	// Apply additional conditions
	for key, value := range conditions {
		query = query.Where(key, value)
	}
	
	err := query.
		Preload("Landlord").                     // Unit landlord
		Preload("Tenant").                       // Current tenant
		Preload("SPAs").                         // Special Power of Attorney representatives
		Order("unit_number ASC").
		Find(&units).Error
	
	return units, err
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

// BatchUpdateStatus updates status for multiple records efficiently
func (q *QueryOptimizer) BatchUpdateStatus(model interface{}, ids []uint, status string, propertyID uint) error {
	if len(ids) == 0 {
		return fmt.Errorf("no IDs provided for batch update")
	}
	
	return q.db.Model(model).
		Where("id IN ? AND property_id = ?", ids, propertyID).
		Update("status", status).Error
}

// BatchDelete soft deletes multiple records efficiently
func (q *QueryOptimizer) BatchDelete(model interface{}, ids []uint, propertyID uint) error {
	if len(ids) == 0 {
		return fmt.Errorf("no IDs provided for batch delete")
	}
	
	return q.db.
		Where("id IN ? AND property_id = ?", ids, propertyID).
		Delete(model).Error
}

// ============================================================================
// AGGREGATION QUERIES (Optimized)
// ============================================================================

// GetRequestCountByStatus returns count of requests grouped by status
func (q *QueryOptimizer) GetRequestCountByStatus(propertyID uint) (map[string]int64, error) {
	type StatusCount struct {
		Status string
		Count  int64
	}
	
	var results []StatusCount
	err := q.db.Model(&property.Request{}).
		Select("status, COUNT(*) as count").
		Where("property_id = ?", propertyID).
		Group("status").
		Find(&results).Error
	
	if err != nil {
		return nil, err
	}
	
	counts := make(map[string]int64)
	for _, result := range results {
		counts[result.Status] = result.Count
	}
	
	return counts, nil
}

// GetBillSummaryByStatus returns bill summary statistics
func (q *QueryOptimizer) GetBillSummaryByStatus(propertyID uint) (map[string]interface{}, error) {
	type BillSummary struct {
		Status      string
		Count       int64
		TotalAmount float64
	}
	
	var results []BillSummary
	err := q.db.Model(&property.Bill{}).
		Select("status, COUNT(*) as count, SUM(total_amount) as total_amount").
		Where("property_id = ?", propertyID).
		Group("status").
		Find(&results).Error
	
	if err != nil {
		return nil, err
	}
	
	summary := make(map[string]interface{})
	for _, result := range results {
		summary[result.Status] = map[string]interface{}{
			"count":        result.Count,
			"total_amount": result.TotalAmount,
		}
	}
	
	return summary, nil
}

// ============================================================================
// PAGINATION HELPER
// ============================================================================

// PaginateResult represents paginated query result
type PaginateResult struct {
	Data       interface{}
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// Paginate applies pagination to a query
func (q *QueryOptimizer) Paginate(query *gorm.DB, page, pageSize int, result interface{}) (*PaginateResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // Max page size
	}
	
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(result).Error; err != nil {
		return nil, err
	}
	
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	
	return &PaginateResult{
		Data:       result,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}
