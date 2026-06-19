package model

type Role struct {
	BaseModel
	Code        string `gorm:"column:code;size:64;not null;uniqueIndex:uk_roles_code" json:"code"`
	Name        string `gorm:"column:name;size:128;not null" json:"name"`
	ScopeType   string `gorm:"column:scope_type;size:32;not null" json:"scope_type"`
	Builtin     bool   `gorm:"column:builtin;not null;default:false" json:"builtin"`
	Description string `gorm:"column:description;size:512" json:"description,omitempty"`
}

func (Role) TableName() string {
	return "roles"
}

type Permission struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	Code        string `gorm:"column:code;size:128;not null;uniqueIndex:uk_permissions_code" json:"code"`
	Resource    string `gorm:"column:resource;size:64;not null" json:"resource"`
	Action      string `gorm:"column:action;size:64;not null" json:"action"`
	Description string `gorm:"column:description;size:512" json:"description,omitempty"`
}

func (Permission) TableName() string {
	return "permissions"
}

type RolePermission struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	RoleID       uint64 `gorm:"column:role_id;not null;uniqueIndex:uk_role_permissions_role_perm" json:"role_id"`
	PermissionID uint64 `gorm:"column:permission_id;not null;uniqueIndex:uk_role_permissions_role_perm" json:"permission_id"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

type RoleBinding struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	UserID    uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_role_bindings_scope" json:"user_id"`
	RoleID    uint64 `gorm:"column:role_id;not null;uniqueIndex:uk_role_bindings_scope" json:"role_id"`
	TenantID  uint64 `gorm:"column:tenant_id;uniqueIndex:uk_role_bindings_scope" json:"tenant_id,omitempty"`
	ProjectID uint64 `gorm:"column:project_id;uniqueIndex:uk_role_bindings_scope" json:"project_id,omitempty"`
	CreatedBy uint64 `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (RoleBinding) TableName() string {
	return "role_bindings"
}
