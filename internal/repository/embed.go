package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type EmbedRepository interface {
	CreateApp(ctx context.Context, app *model.EmbedApp) error
	ListApps(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]model.EmbedApp, int64, error)
	GetApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) (*model.EmbedApp, error)
	GetAppByAppID(ctx context.Context, appID string) (*model.EmbedApp, error)
	UpdateAppStatus(ctx context.Context, tenantID uint64, projectID uint64, id uint64, status string) (*model.EmbedApp, error)
	DeleteApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) error
	EnsureSession(ctx context.Context, input EnsureEmbedSessionInput) (*model.EmbedSession, *model.ChatSession, error)
	GetSessionByChatSessionID(ctx context.Context, chatSessionID uint64) (*model.EmbedSession, error)
}

type EnsureEmbedSessionInput struct {
	App              *model.EmbedApp
	ExternalUserID   string
	ExternalUserName string
	SessionKey       string
	SessionMode      string
}

type GormEmbedRepository struct {
	db *gorm.DB
}

func NewEmbedRepository(db *gorm.DB) EmbedRepository {
	return &GormEmbedRepository{db: db}
}

func (r *GormEmbedRepository) CreateApp(ctx context.Context, app *model.EmbedApp) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *GormEmbedRepository) ListApps(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]model.EmbedApp, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.EmbedApp{})
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var apps []model.EmbedApp
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&apps).Error; err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

func (r *GormEmbedRepository) GetAppByAppID(ctx context.Context, appID string) (*model.EmbedApp, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var app model.EmbedApp
	if err := r.db.WithContext(ctx).First(&app, "app_id = ?", strings.TrimSpace(appID)).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *GormEmbedRepository) GetApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) (*model.EmbedApp, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var app model.EmbedApp
	if err := r.db.WithContext(ctx).
		First(&app, "tenant_id = ? AND project_id = ? AND id = ?", tenantID, projectID, id).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *GormEmbedRepository) UpdateAppStatus(ctx context.Context, tenantID uint64, projectID uint64, id uint64, status string) (*model.EmbedApp, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var app model.EmbedApp
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&app, "tenant_id = ? AND project_id = ? AND id = ?", tenantID, projectID, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.EmbedApp{}).
			Where("id = ?", app.ID).
			Update("status", status).
			Error; err != nil {
			return err
		}
		app.Status = status
		if status != "active" {
			if err := tx.Model(&model.EmbedSession{}).
				Where("embed_app_id = ? AND status = ?", app.ID, "active").
				Update("status", status).
				Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *GormEmbedRepository) DeleteApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app model.EmbedApp
		if err := tx.First(&app, "tenant_id = ? AND project_id = ? AND id = ?", tenantID, projectID, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.EmbedSession{}).
			Where("embed_app_id = ? AND status = ?", app.ID, "active").
			Update("status", "deleted").
			Error; err != nil {
			return err
		}
		result := tx.Delete(&model.EmbedApp{}, "id = ?", app.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *GormEmbedRepository) EnsureSession(ctx context.Context, input EnsureEmbedSessionInput) (*model.EmbedSession, *model.ChatSession, error) {
	if r.db == nil {
		return nil, nil, ErrDatabaseDisabled
	}
	if input.App == nil {
		return nil, nil, gorm.ErrRecordNotFound
	}

	var embedSession *model.EmbedSession
	var chatSession *model.ChatSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := r.ensureEmbedUserInTx(tx, input)
		if err != nil {
			return err
		}

		if strings.TrimSpace(input.SessionMode) != "new" {
			existingEmbed, existingChat, err := r.findReusableSessionInTx(tx, input)
			if err == nil {
				embedSession = existingEmbed
				chatSession = existingChat
				return nil
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
		}

		title := embedSessionTitle(input.SessionKey)
		nextChat := &model.ChatSession{
			TenantID:  input.App.TenantID,
			ProjectID: input.App.ProjectID,
			UserID:    user.ID,
			Title:     title,
			Status:    "active",
		}
		if err := tx.Create(nextChat).Error; err != nil {
			return err
		}

		nextEmbed := &model.EmbedSession{
			TenantID:         input.App.TenantID,
			ProjectID:        input.App.ProjectID,
			EmbedAppID:       input.App.ID,
			AppID:            input.App.AppID,
			ExternalUserID:   input.ExternalUserID,
			ExternalUserName: input.ExternalUserName,
			SessionKey:       input.SessionKey,
			ChatSessionID:    nextChat.ID,
			UserID:           user.ID,
			Status:           "active",
		}
		if err := tx.Create(nextEmbed).Error; err != nil {
			return err
		}
		embedSession = nextEmbed
		chatSession = nextChat
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return embedSession, chatSession, nil
}

func (r *GormEmbedRepository) GetSessionByChatSessionID(ctx context.Context, chatSessionID uint64) (*model.EmbedSession, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var session model.EmbedSession
	if err := r.db.WithContext(ctx).First(&session, "chat_session_id = ? AND status = ?", chatSessionID, "active").Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *GormEmbedRepository) ensureEmbedUserInTx(tx *gorm.DB, input EnsureEmbedSessionInput) (*model.User, error) {
	username := embedUsername(input.App.AppID, input.ExternalUserID)
	var user model.User
	err := tx.First(&user, "username = ?", username).Error
	if err == nil {
		if err := ensureTenantMembershipInTx(tx, input.App.TenantID, user.ID); err != nil {
			return nil, err
		}
		if err := ensureProjectMembershipInTx(tx, input.App.TenantID, input.App.ProjectID, user.ID); err != nil {
			return nil, err
		}
		return &user, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	displayName := strings.TrimSpace(input.ExternalUserName)
	if displayName == "" {
		displayName = strings.TrimSpace(input.ExternalUserID)
	}
	if displayName == "" {
		displayName = "嵌入访客"
	}
	nowHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", username, time.Now().UnixNano())))
	user = model.User{
		Username:     username,
		PasswordHash: "embed-subject:" + hex.EncodeToString(nowHash[:]),
		DisplayName:  displayName,
		Status:       "active",
	}
	if err := tx.Create(&user).Error; err != nil {
		return nil, err
	}
	if err := ensureTenantMembershipInTx(tx, input.App.TenantID, user.ID); err != nil {
		return nil, err
	}
	if err := ensureProjectMembershipInTx(tx, input.App.TenantID, input.App.ProjectID, user.ID); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormEmbedRepository) findReusableSessionInTx(tx *gorm.DB, input EnsureEmbedSessionInput) (*model.EmbedSession, *model.ChatSession, error) {
	var embedSession model.EmbedSession
	err := tx.Where(
		"embed_app_id = ? AND external_user_id = ? AND session_key = ? AND status = ?",
		input.App.ID,
		input.ExternalUserID,
		input.SessionKey,
		"active",
	).Order("id DESC").First(&embedSession).Error
	if err != nil {
		return nil, nil, err
	}
	var chatSession model.ChatSession
	if err := tx.First(&chatSession, "id = ? AND status = ?", embedSession.ChatSessionID, "active").Error; err != nil {
		return nil, nil, err
	}
	return &embedSession, &chatSession, nil
}

func ensureTenantMembershipInTx(tx *gorm.DB, tenantID uint64, userID uint64) error {
	var existing model.TenantMember
	err := tx.Unscoped().Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&existing).Error
	if err == nil {
		return tx.Unscoped().
			Model(&model.TenantMember{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"status": "active", "deleted_at": nil}).
			Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return tx.Create(&model.TenantMember{TenantID: tenantID, UserID: userID, Status: "active"}).Error
}

func ensureProjectMembershipInTx(tx *gorm.DB, tenantID uint64, projectID uint64, userID uint64) error {
	var existing model.ProjectMember
	err := tx.Unscoped().Where("project_id = ? AND user_id = ?", projectID, userID).First(&existing).Error
	if err == nil {
		return tx.Unscoped().
			Model(&model.ProjectMember{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"tenant_id": tenantID, "status": "active", "deleted_at": nil}).
			Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return tx.Create(&model.ProjectMember{TenantID: tenantID, ProjectID: projectID, UserID: userID, Status: "active"}).Error
}

func embedUsername(appID string, externalUserID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(appID) + "\x00" + strings.TrimSpace(externalUserID)))
	return "embed_" + safeIdentifier(appID, 28) + "_" + hex.EncodeToString(sum[:])[:20]
}

func safeIdentifier(value string, limit int) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= limit {
			break
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

func embedSessionTitle(key string) string {
	key = strings.TrimSpace(key)
	if key == "" || key == "default" {
		return "内嵌会话"
	}
	if len(key) > 80 {
		key = key[:80]
	}
	return "内嵌会话 · " + key
}
