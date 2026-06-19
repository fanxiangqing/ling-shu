package repository

import (
	"context"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type PermissionRepository interface {
	ListRoles(ctx context.Context) ([]model.Role, error)
	ListPermissions(ctx context.Context) ([]model.Permission, error)
	GetRoleByCode(ctx context.Context, code string) (*model.Role, error)
	CreateRoleBinding(ctx context.Context, binding *model.RoleBinding) error
	ListRoleBindings(ctx context.Context, filter RoleBindingFilter, page Page) ([]RoleBindingRow, int64, error)
	GetUserRoles(ctx context.Context, filter RoleBindingFilter) ([]RoleBindingRow, error)
	GetUserPermissions(ctx context.Context, filter RoleBindingFilter) ([]model.Permission, error)
}

type RoleBindingFilter struct {
	UserID    uint64
	TenantID  uint64
	ProjectID uint64
}

type RoleBindingRow struct {
	ID        uint64 `json:"id"`
	UserID    uint64 `json:"user_id"`
	RoleID    uint64 `json:"role_id"`
	RoleCode  string `json:"role_code"`
	RoleName  string `json:"role_name"`
	ScopeType string `json:"scope_type"`
	TenantID  uint64 `json:"tenant_id,omitempty"`
	ProjectID uint64 `json:"project_id,omitempty"`
	CreatedBy uint64 `json:"created_by,omitempty"`
}

type GormPermissionRepository struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &GormPermissionRepository{db: db}
}

func (r *GormPermissionRepository) ListRoles(ctx context.Context) ([]model.Role, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var roles []model.Role
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *GormPermissionRepository) ListPermissions(ctx context.Context) ([]model.Permission, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var permissions []model.Permission
	if err := r.db.WithContext(ctx).Order("resource ASC, action ASC").Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func (r *GormPermissionRepository) GetRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, "code = ?", code).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *GormPermissionRepository) CreateRoleBinding(ctx context.Context, binding *model.RoleBinding) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(binding).Error
}

func (r *GormPermissionRepository) ListRoleBindings(ctx context.Context, filter RoleBindingFilter, page Page) ([]RoleBindingRow, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := roleBindingQuery(r.db.WithContext(ctx), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []RoleBindingRow
	if err := query.Order("role_bindings.id DESC").Offset(page.Offset()).Limit(page.Limit()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormPermissionRepository) GetUserRoles(ctx context.Context, filter RoleBindingFilter) ([]RoleBindingRow, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var rows []RoleBindingRow
	if err := roleBindingQuery(r.db.WithContext(ctx), filter).Order("roles.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormPermissionRepository) GetUserPermissions(ctx context.Context, filter RoleBindingFilter) ([]model.Permission, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var permissions []model.Permission
	query := r.db.WithContext(ctx).Table("role_bindings").
		Select("DISTINCT permissions.id, permissions.code, permissions.resource, permissions.action, permissions.description").
		Joins("JOIN roles ON roles.id = role_bindings.role_id").
		Joins("JOIN role_permissions ON role_permissions.role_id = roles.id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id")
	query = applyRoleBindingScope(query, filter)
	if err := query.Order("permissions.resource ASC, permissions.action ASC").Scan(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func roleBindingQuery(db *gorm.DB, filter RoleBindingFilter) *gorm.DB {
	query := db.Table("role_bindings").
		Select("role_bindings.id, role_bindings.user_id, role_bindings.role_id, roles.code AS role_code, roles.name AS role_name, roles.scope_type, role_bindings.tenant_id, role_bindings.project_id, role_bindings.created_by").
		Joins("JOIN roles ON roles.id = role_bindings.role_id")
	return applyRoleBindingScope(query, filter)
}

func applyRoleBindingScope(query *gorm.DB, filter RoleBindingFilter) *gorm.DB {
	if filter.UserID > 0 {
		query = query.Where("role_bindings.user_id = ?", filter.UserID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("(role_bindings.project_id = ? OR (role_bindings.project_id IS NULL OR role_bindings.project_id = 0))", filter.ProjectID)
	}
	if filter.TenantID > 0 {
		query = query.Where("(role_bindings.tenant_id = ? OR (role_bindings.tenant_id IS NULL OR role_bindings.tenant_id = 0))", filter.TenantID)
	}
	return query
}
