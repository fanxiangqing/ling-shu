package model

import "time"

type User struct {
	BaseModel
	Username     string     `gorm:"column:username;size:64;not null;uniqueIndex:uk_users_username" json:"username"`
	Email        *string    `gorm:"column:email;size:191;uniqueIndex:uk_users_email" json:"email,omitempty"`
	Mobile       *string    `gorm:"column:mobile;size:32" json:"mobile,omitempty"`
	PasswordHash string     `gorm:"column:password_hash;size:255;not null" json:"-"`
	DisplayName  string     `gorm:"column:display_name;size:128;not null" json:"display_name"`
	Status       string     `gorm:"column:status;size:32;not null;default:active" json:"status"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at" json:"last_login_at,omitempty"`
}

func (User) TableName() string {
	return "users"
}

type TenantMember struct {
	BaseModel
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_members_tenant_user" json:"tenant_id"`
	UserID   uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_tenant_members_tenant_user" json:"user_id"`
	Status   string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (TenantMember) TableName() string {
	return "tenant_members"
}

type ProjectMember struct {
	BaseModel
	TenantID  uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID uint64 `gorm:"column:project_id;not null;uniqueIndex:uk_project_members_project_user" json:"project_id"`
	UserID    uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_project_members_project_user" json:"user_id"`
	Status    string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (ProjectMember) TableName() string {
	return "project_members"
}
