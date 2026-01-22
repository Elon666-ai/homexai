package master

import (
	"homexai/internal/models/master"

	"gorm.io/gorm"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// Create creates a new role
func (r *RoleRepository) Create(role *master.Role) error {
	return r.db.Create(role).Error
}

// FindByID finds a role by ID
func (r *RoleRepository) FindByID(id uint) (*master.Role, error) {
	var role master.Role
	err := r.db.First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// FindByCode finds a role by code
func (r *RoleRepository) FindByCode(code string) (*master.Role, error) {
	var role master.Role
	err := r.db.Where("role_code = ?", code).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// Update updates a role
func (r *RoleRepository) Update(role *master.Role) error {
	return r.db.Save(role).Error
}

// Delete deletes a role
func (r *RoleRepository) Delete(id uint) error {
	return r.db.Delete(&master.Role{}, id).Error
}

// List lists all roles
func (r *RoleRepository) List() ([]master.Role, error) {
	var roles []master.Role
	err := r.db.Find(&roles).Error
	return roles, err
}

// FindWithPermissions finds role with permissions
func (r *RoleRepository) FindWithPermissions(id uint) (*master.Role, error) {
	var role master.Role
	err := r.db.Preload("RolePermissions.Permission").First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// AssignPermission assigns a permission to role
func (r *RoleRepository) AssignPermission(roleID, permissionID uint) error {
	rolePermission := &master.RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}
	return r.db.Create(rolePermission).Error
}

// RemovePermission removes a permission from role
func (r *RoleRepository) RemovePermission(roleID, permissionID uint) error {
	return r.db.Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Delete(&master.RolePermission{}).Error
}

// GetPermissions gets all permissions for a role
func (r *RoleRepository) GetPermissions(roleID uint) ([]master.Permission, error) {
	var permissions []master.Permission
	err := r.db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).
		Find(&permissions).Error
	return permissions, err
}
