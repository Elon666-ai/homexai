package master

import (
	"homexai/internal/models/master"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *master.User) error {
	return r.db.Create(user).Error
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(id uint) (*master.User, error) {
	var user master.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail finds a user by email
func (r *UserRepository) FindByEmail(email string) (*master.User, error) {
	var user master.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByPhone finds a user by phone
func (r *UserRepository) FindByPhone(phone string) (*master.User, error) {
	var user master.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmailOrPhone finds a user by email or phone
func (r *UserRepository) FindByEmailOrPhone(identifier string) (*master.User, error) {
	var user master.User
	err := r.db.Where("email = ? OR phone = ?", identifier, identifier).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates a user
func (r *UserRepository) Update(user *master.User) error {
	return r.db.Save(user).Error
}

// UpdateFields updates specific fields
func (r *UserRepository) UpdateFields(id uint, fields map[string]interface{}) error {
	return r.db.Model(&master.User{}).Where("id = ?", id).Updates(fields).Error
}

// UpdatePasswordAndSalt updates password hash and salt
func (r *UserRepository) UpdatePasswordAndSalt(id uint, passwordHash, salt string) error {
	return r.db.Model(&master.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password_hash": passwordHash,
		"salt":          salt,
	}).Error
}

// Delete deletes a user
func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&master.User{}, id).Error
}

// List lists users with pagination
func (r *UserRepository) List(page, perPage int) ([]master.User, int64, error) {
	var users []master.User
	var total int64

	offset := (page - 1) * perPage

	err := r.db.Model(&master.User{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.Preload("UserRoles").
		Offset(offset).Limit(perPage).Find(&users).Error
	return users, total, err
}

// ExistsByEmail checks if email exists
func (r *UserRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&master.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// ExistsByPhone checks if phone exists
func (r *UserRepository) ExistsByPhone(phone string) (bool, error) {
	var count int64
	err := r.db.Model(&master.User{}).Where("phone = ?", phone).Count(&count).Error
	return count > 0, err
}

// VerifyEmail marks email as verified
func (r *UserRepository) VerifyEmail(id uint) error {
	return r.db.Model(&master.User{}).Where("id = ?", id).
		Update("email_verified", true).Error
}

// VerifyPhone marks phone as verified
func (r *UserRepository) VerifyPhone(id uint) error {
	return r.db.Model(&master.User{}).Where("id = ?", id).
		Update("phone_verified", true).Error
}

// UpdatePassword updates user password
func (r *UserRepository) UpdatePassword(id uint, passwordHash string) error {
	return r.db.Model(&master.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"password_hash":        passwordHash,
			"must_change_password": false,
		}).Error
}

// FindWithRoles finds user with their roles
func (r *UserRepository) FindWithRoles(id uint) (*master.User, error) {
	var user master.User
	err := r.db.Preload("UserRoles").
		Preload("UserRoles.Property").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Search searches users by name or email
func (r *UserRepository) Search(query string, page, perPage int) ([]master.User, int64, error) {
	var users []master.User
	var total int64

	offset := (page - 1) * perPage

	queryBuilder := r.db.Model(&master.User{})
	if query != "" {
		searchPattern := "%" + query + "%"
		queryBuilder = queryBuilder.Where("full_name LIKE ? OR email LIKE ?", searchPattern, searchPattern)
	}

	err := queryBuilder.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = queryBuilder.Preload("UserRoles").
		Offset(offset).Limit(perPage).Find(&users).Error
	return users, total, err
}

func (r *UserRepository) SearchByRole(query string, role string, page, perPage int) ([]master.User, int64, error) {
	var users []master.User
	var total int64

	offset := (page - 1) * perPage

	queryBuilder := r.db.Model(&master.User{}).Joins("JOIN user_roles ur ON users.id = ur.user_id").Where("ur.role = ? AND ur.status = ?", role, "active")

	if query != "" {
		searchPattern := "%" + query + "%"
		queryBuilder = queryBuilder.Where("users.full_name LIKE ? OR users.email LIKE ?", searchPattern, searchPattern)
	}

	err := queryBuilder.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = queryBuilder.Preload("UserRoles").
		Offset(offset).Limit(perPage).Find(&users).Error
	return users, total, err
}

// GetUserNamesByRole gets user names list by role for a specific property
func (r *UserRepository) GetUserNamesByRole(propertyID uint, role string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// Build query to get users with specific role for the property
	query := r.db.Table("users").
		Select("users.id, users.full_name, users.email, users.phone").
		Joins("JOIN user_roles ur ON users.id = ur.user_id").
		Where("ur.property_id = ? AND ur.role = ? AND ur.status = ? AND users.status = ?",
			propertyID, role, "active", "active")

	err := query.Scan(&results).Error
	return results, err
}