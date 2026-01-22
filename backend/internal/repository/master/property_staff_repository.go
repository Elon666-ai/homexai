package master

import (
	"time"

	"gorm.io/gorm"
)

type PropertyStaffRepository struct {
	db *gorm.DB
}

func NewPropertyStaffRepository(db *gorm.DB) *PropertyStaffRepository {
	return &PropertyStaffRepository{db: db}
}

// PropertyStaffInfo represents staff information from joined tables
type PropertyStaffInfo struct {
	UserID      uint       `json:"user_id"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Phone       string     `json:"phone"`
	Status      string     `json:"status"`
	Role        string     `json:"role"`
	PropertyID  uint       `json:"property_id"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

// ListByPropertyAndRoles lists users by property and roles with pagination
func (r *PropertyStaffRepository) ListByPropertyAndRoles(propertyID uint, roles []string, page, perPage int) ([]PropertyStaffInfo, int64, error) {
	var staff []PropertyStaffInfo
	var total int64

	offset := (page - 1) * perPage

	query := r.db.Table("user_roles upr").
		Select(`
			upr.user_id,
			u.email,
			u.full_name,
			u.phone,
			upr.status,
			upr.role,
			upr.property_id,
			upr.created_at,
			u.last_login_at
		`).
		Joins("JOIN users u ON u.id = upr.user_id").
		Where("upr.property_id = ?", propertyID).
		Where("upr.role IN ?", roles)

	// Count total
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err = query.Order("upr.created_at DESC").
		Offset(offset).
		Limit(perPage).
		Scan(&staff).Error

	return staff, total, err
}

// GetByPropertyAndUser gets staff info by property and user ID
func (r *PropertyStaffRepository) GetByPropertyAndUser(propertyID, userID uint) (*PropertyStaffInfo, error) {
	var staff PropertyStaffInfo

	err := r.db.Table("user_roles upr").
		Select(`
			upr.user_id,
			u.email,
			u.full_name,
			u.phone,
			upr.status,
			upr.role,
			upr.property_id,
			upr.created_at,
			u.last_login_at
		`).
		Joins("JOIN users u ON u.id = upr.user_id").
		Where("upr.property_id = ? AND upr.user_id = ?", propertyID, userID).
		First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

// AssignRole assigns a role to a user in a property
func (r *PropertyStaffRepository) AssignRole(userID, propertyID uint, role string, assignedBy uint) error {
	// Check if role assignment already exists
	var count int64
	r.db.Table("user_roles").
		Where("user_id = ? AND property_id = ? AND role = ?", userID, propertyID, role).
		Count(&count)

	if count > 0 {
		return nil // Already assigned
	}

	// Insert new role assignment
	return r.db.Exec(`
		INSERT INTO user_roles (user_id, property_id, role, status, assigned_by, assigned_at, created_at, updated_at)
		VALUES (?, ?, ?, 'active', ?, NOW(), NOW(), NOW())
	`, userID, propertyID, role, assignedBy).Error
}

// RemoveRole removes a role assignment
func (r *PropertyStaffRepository) RemoveRole(userID, propertyID uint) error {
	result := r.db.Exec(`
		DELETE FROM user_roles 
		WHERE user_id = ? AND property_id = ? AND role IN ('property_account', 'property_staff')
	`, userID, propertyID)

	if result.Error != nil {
		return result.Error
	}

	// Check if any rows were actually deleted
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// UpdateRoleStatus updates the status of a role assignment
func (r *PropertyStaffRepository) UpdateRoleStatus(userID, propertyID uint, status string) error {
	return r.db.Exec(`
		UPDATE user_roles 
		SET status = ?, updated_at = NOW()
		WHERE user_id = ? AND property_id = ? AND role IN ('property_account', 'property_staff')
	`, status, userID, propertyID).Error
}

// GetPropertyAdminCount gets the count of property_admin for a property
func (r *PropertyStaffRepository) GetPropertyAdminCount(propertyID uint) (int64, error) {
	var count int64

	err := r.db.Table("user_roles").
		Where("property_id = ? AND role = 'property_admin' AND status = 'active'", propertyID).
		Count(&count).Error

	return count, err
}

// GetPropertyAdmin gets the property_admin for a property
func (r *PropertyStaffRepository) GetPropertyAdmin(propertyID uint) (*PropertyStaffInfo, error) {
	var staff PropertyStaffInfo

	err := r.db.Table("user_roles upr").
		Select(`
			upr.user_id,
			u.email,
			u.full_name,
			u.phone,
			upr.status,
			upr.role,
			upr.property_id,
			upr.created_at,
			u.last_login_at
		`).
		Joins("JOIN users u ON u.id = upr.user_id").
		Where("upr.property_id = ? AND upr.role = 'property_admin' AND upr.status = 'active'", propertyID).
		First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

// RemovePropertyAdminRole removes the property_admin role from a user in a property
func (r *PropertyStaffRepository) RemovePropertyAdminRole(userID, propertyID uint) error {
	return r.db.Exec(`
		DELETE FROM user_roles 
		WHERE user_id = ? AND property_id = ? AND role = 'property_admin'
	`, userID, propertyID).Error
}

// HasOtherActiveRoles checks if user has any other active roles (in any property)
func (r *PropertyStaffRepository) HasOtherActiveRoles(userID, excludePropertyID uint) (bool, error) {
	var count int64

	err := r.db.Table("user_roles").
		Where("user_id = ? AND status = 'active'", userID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
