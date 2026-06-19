package repository

import (
	"context"
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
	List(ctx context.Context, page Page) ([]model.User, int64, error)
	UpdateLastLogin(ctx context.Context, id uint64, at time.Time) error
	AddTenantMember(ctx context.Context, member *model.TenantMember) error
	ListTenantMembers(ctx context.Context, tenantID uint64, page Page) ([]MemberRow, int64, error)
	AddProjectMember(ctx context.Context, member *model.ProjectMember) error
	ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]MemberRow, int64, error)
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
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *GormUserRepository) ListTenantMembers(ctx context.Context, tenantID uint64, page Page) ([]MemberRow, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Table("tenant_members").
		Joins("JOIN users ON users.id = tenant_members.user_id").
		Where("tenant_members.tenant_id = ?", tenantID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []MemberRow
	if err := query.Select("tenant_members.id, tenant_members.tenant_id, tenant_members.user_id, users.username, users.display_name, users.email, users.mobile, tenant_members.status").
		Order("tenant_members.id DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormUserRepository) AddProjectMember(ctx context.Context, member *model.ProjectMember) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *GormUserRepository) ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]MemberRow, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Table("project_members").
		Joins("JOIN users ON users.id = project_members.user_id").
		Where("project_members.tenant_id = ? AND project_members.project_id = ?", tenantID, projectID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []MemberRow
	if err := query.Select("project_members.id, project_members.tenant_id, project_members.project_id, project_members.user_id, users.username, users.display_name, users.email, users.mobile, project_members.status").
		Order("project_members.id DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
