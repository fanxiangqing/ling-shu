package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	CreateMainAccount(ctx context.Context, user *model.User, tenant *model.Tenant, project *model.Project, roleCode string) error
	CreateTenantAccount(ctx context.Context, user *model.User, member *model.TenantMember, roleCode string, createdBy uint64) error
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	HasActiveWorkspace(ctx context.Context, userID uint64) (bool, error)
	List(ctx context.Context, page Page) ([]model.User, int64, error)
	UpdateLastLogin(ctx context.Context, id uint64, at time.Time) error
	AddTenantMember(ctx context.Context, member *model.TenantMember) error
	ListTenantMembers(ctx context.Context, tenantID uint64, page Page) ([]MemberRow, int64, error)
	IsTenantPrimaryAdminMember(ctx context.Context, tenantID uint64, memberID uint64) (bool, error)
	UpdateTenantMemberStatus(ctx context.Context, tenantID uint64, memberID uint64, status string) error
	DeleteTenantMember(ctx context.Context, tenantID uint64, memberID uint64) error
	AddProjectMember(ctx context.Context, member *model.ProjectMember) error
	ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]MemberRow, int64, error)
	IsProjectPrimaryAdminMember(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64) (bool, error)
	UpdateProjectMemberStatus(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64, status string) error
	DeleteProjectMember(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64) error
}

type MemberRow struct {
	ID          uint64  `json:"id"`
	TenantID    uint64  `json:"tenant_id"`
	ProjectID   uint64  `json:"project_id,omitempty"`
	UserID      uint64  `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email,omitempty"`
	Mobile      *string `json:"mobile,omitempty"`
	Status      string  `json:"status"`
}

type GormUserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(ctx context.Context, user *model.User) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormUserRepository) CreateMainAccount(ctx context.Context, user *model.User, tenant *model.Tenant, project *model.Project, roleCode string) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if tenant == nil {
			return fmt.Errorf("tenant is required")
		}
		if tenant.Code == "" {
			tenant.Code = fmt.Sprintf("org-%d", user.ID)
		}
		if tenant.Status == "" {
			tenant.Status = "active"
		}
		if err := tx.Create(tenant).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.TenantMember{
			TenantID: tenant.ID,
			UserID:   user.ID,
			Status:   "active",
		}).Error; err != nil {
			return err
		}
		if roleCode != "" {
			if err := bindRoleInTx(tx, user.ID, roleCode, tenant.ID, 0, user.ID); err != nil {
				return err
			}
		}
		if project != nil {
			project.TenantID = tenant.ID
			project.CreatedBy = user.ID
			if project.Code == "" {
				project.Code = "default"
			}
			if project.Status == "" {
				project.Status = "active"
			}
			if err := tx.Create(project).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.ProjectMember{
				TenantID:  tenant.ID,
				ProjectID: project.ID,
				UserID:    user.ID,
				Status:    "active",
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormUserRepository) CreateTenantAccount(ctx context.Context, user *model.User, member *model.TenantMember, roleCode string, createdBy uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if member == nil || member.TenantID == 0 {
			return fmt.Errorf("tenant member is required")
		}
		member.UserID = user.ID
		if member.Status == "" {
			member.Status = "active"
		}
		if err := tx.Create(member).Error; err != nil {
			return err
		}
		if roleCode != "" {
			if err := bindRoleInTx(tx, user.ID, roleCode, member.TenantID, 0, createdBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func bindRoleInTx(tx *gorm.DB, userID uint64, roleCode string, tenantID uint64, projectID uint64, createdBy uint64) error {
	var role model.Role
	if err := tx.First(&role, "code = ?", roleCode).Error; err != nil {
		return err
	}
	return tx.Create(&model.RoleBinding{
		UserID:    userID,
		RoleID:    role.ID,
		TenantID:  tenantID,
		ProjectID: projectID,
		CreatedBy: createdBy,
	}).Error
}

func (r *GormUserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "username = ?", username).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) HasActiveWorkspace(ctx context.Context, userID uint64) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseDisabled
	}
	var count int64
	err := r.db.WithContext(ctx).Table("users").
		Where("users.id = ? AND users.status = ? AND users.deleted_at IS NULL", userID, "active").
		Where(
			"(EXISTS (SELECT 1 FROM tenant_members tm WHERE tm.user_id = users.id AND tm.status = 'active' AND tm.deleted_at IS NULL) OR EXISTS (SELECT 1 FROM role_bindings rb JOIN roles r ON r.id = rb.role_id WHERE rb.user_id = users.id AND r.code = 'super_admin'))",
		).
		Count(&count).
		Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormUserRepository) List(ctx context.Context, page Page) ([]model.User, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.User{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []model.User
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *GormUserRepository) UpdateLastLogin(ctx context.Context, id uint64, at time.Time) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("last_login_at", at).Error
}

func (r *GormUserRepository) AddTenantMember(ctx context.Context, member *model.TenantMember) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	var existing model.TenantMember
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
		First(&existing).
		Error
	if err == nil {
		status := member.Status
		if status == "" {
			status = "active"
		}
		if err := r.db.WithContext(ctx).
			Unscoped().
			Model(&model.TenantMember{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"status": status, "deleted_at": nil}).
			Error; err != nil {
			return err
		}
		existing.Status = status
		existing.DeletedAt = gorm.DeletedAt{}
		*member = existing
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *GormUserRepository) ListTenantMembers(ctx context.Context, tenantID uint64, page Page) ([]MemberRow, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Table("tenant_members").
		Joins("JOIN users ON users.id = tenant_members.user_id").
		Where("tenant_members.tenant_id = ?", tenantID).
		Where("tenant_members.deleted_at IS NULL").
		Where("users.deleted_at IS NULL")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []MemberRow
	if err := query.Select("tenant_members.id, tenant_members.tenant_id, tenant_members.user_id, users.username, users.display_name, users.email, users.mobile, CASE WHEN users.status <> 'active' THEN users.status ELSE tenant_members.status END AS status").
		Order("tenant_members.id DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormUserRepository) IsTenantPrimaryAdminMember(ctx context.Context, tenantID uint64, memberID uint64) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseDisabled
	}
	var count int64
	err := r.db.WithContext(ctx).Table("tenant_members").
		Joins("JOIN role_bindings ON role_bindings.user_id = tenant_members.user_id AND role_bindings.tenant_id = tenant_members.tenant_id").
		Joins("JOIN roles ON roles.id = role_bindings.role_id").
		Where("tenant_members.tenant_id = ? AND tenant_members.id = ?", tenantID, memberID).
		Where("roles.code = ? AND (role_bindings.created_by = tenant_members.user_id OR tenant_members.id = (SELECT MIN(tm.id) FROM tenant_members tm WHERE tm.tenant_id = tenant_members.tenant_id AND tm.deleted_at IS NULL))", "tenant_admin").
		Count(&count).
		Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormUserRepository) UpdateTenantMemberStatus(ctx context.Context, tenantID uint64, memberID uint64, status string) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	var member model.TenantMember
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, memberID).First(&member).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&member).Update("status", status).Error
}

func (r *GormUserRepository) DeleteTenantMember(ctx context.Context, tenantID uint64, memberID uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var member model.TenantMember
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, memberID).First(&member).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, member.UserID).Delete(&model.ProjectMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&member).Error
	})
}

func (r *GormUserRepository) AddProjectMember(ctx context.Context, member *model.ProjectMember) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	var existing model.ProjectMember
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("tenant_id = ? AND project_id = ? AND user_id = ?", member.TenantID, member.ProjectID, member.UserID).
		First(&existing).
		Error
	if err == nil {
		status := member.Status
		if status == "" {
			status = "active"
		}
		if err := r.db.WithContext(ctx).
			Unscoped().
			Model(&model.ProjectMember{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"status": status, "deleted_at": nil}).
			Error; err != nil {
			return err
		}
		existing.Status = status
		existing.DeletedAt = gorm.DeletedAt{}
		*member = existing
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *GormUserRepository) ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]MemberRow, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Table("project_members").
		Joins("JOIN users ON users.id = project_members.user_id").
		Joins("JOIN tenant_members ON tenant_members.tenant_id = project_members.tenant_id AND tenant_members.user_id = project_members.user_id").
		Where("project_members.tenant_id = ? AND project_members.project_id = ?", tenantID, projectID).
		Where("project_members.deleted_at IS NULL").
		Where("tenant_members.deleted_at IS NULL").
		Where("users.deleted_at IS NULL")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []MemberRow
	if err := query.Select("project_members.id, project_members.tenant_id, project_members.project_id, project_members.user_id, users.username, users.display_name, users.email, users.mobile, CASE WHEN users.status <> 'active' THEN users.status WHEN tenant_members.status <> 'active' THEN tenant_members.status ELSE project_members.status END AS status").
		Order("project_members.id DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormUserRepository) IsProjectPrimaryAdminMember(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseDisabled
	}
	var count int64
	err := r.db.WithContext(ctx).Table("project_members").
		Joins("JOIN role_bindings ON role_bindings.user_id = project_members.user_id AND role_bindings.tenant_id = project_members.tenant_id").
		Joins("JOIN roles ON roles.id = role_bindings.role_id").
		Where("project_members.tenant_id = ? AND project_members.project_id = ? AND project_members.id = ?", tenantID, projectID, memberID).
		Where("roles.code = ? AND (role_bindings.created_by = project_members.user_id OR EXISTS (SELECT 1 FROM tenant_members tm WHERE tm.tenant_id = project_members.tenant_id AND tm.user_id = project_members.user_id AND tm.id = (SELECT MIN(tm2.id) FROM tenant_members tm2 WHERE tm2.tenant_id = project_members.tenant_id AND tm2.deleted_at IS NULL)))", "tenant_admin").
		Count(&count).
		Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormUserRepository) UpdateProjectMemberStatus(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64, status string) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	var member model.ProjectMember
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND project_id = ? AND id = ?", tenantID, projectID, memberID).First(&member).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&member).Update("status", status).Error
}

func (r *GormUserRepository) DeleteProjectMember(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	var member model.ProjectMember
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND project_id = ? AND id = ?", tenantID, projectID, memberID).First(&member).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&member).Error
}
