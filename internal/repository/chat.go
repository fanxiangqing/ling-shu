package repository

import (
	"context"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type ChatRepository interface {
	CreateSession(ctx context.Context, session *model.ChatSession) error
	GetSession(ctx context.Context, id uint64) (*model.ChatSession, error)
	ListSessions(ctx context.Context, filter ChatSessionFilter, page Page) ([]model.ChatSession, int64, error)
	CreateMessage(ctx context.Context, message *model.ChatMessage) error
	ListMessages(ctx context.Context, filter ChatMessageFilter, page Page) ([]model.ChatMessage, int64, error)
	GetRecentMessages(ctx context.Context, sessionID uint64, limit int) ([]model.ChatMessage, error)
}

type ChatSessionFilter struct {
	TenantID  uint64
	ProjectID uint64
	UserID    uint64
	Status    string
}

type ChatMessageFilter struct {
	TenantID  uint64
	ProjectID uint64
	SessionID uint64
}

type GormChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &GormChatRepository{db: db}
}

func (r *GormChatRepository) CreateSession(ctx context.Context, session *model.ChatSession) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *GormChatRepository) GetSession(ctx context.Context, id uint64) (*model.ChatSession, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var session model.ChatSession
	if err := r.db.WithContext(ctx).First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *GormChatRepository) ListSessions(ctx context.Context, filter ChatSessionFilter, page Page) ([]model.ChatSession, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.ChatSession{})
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var sessions []model.ChatSession
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&sessions).Error; err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

func (r *GormChatRepository) CreateMessage(ctx context.Context, message *model.ChatMessage) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *GormChatRepository) ListMessages(ctx context.Context, filter ChatMessageFilter, page Page) ([]model.ChatMessage, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.ChatMessage{}).Where("session_id = ?", filter.SessionID)
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var messages []model.ChatMessage
	if err := query.Order("id ASC").Offset(page.Offset()).Limit(page.Limit()).Find(&messages).Error; err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

func (r *GormChatRepository) GetRecentMessages(ctx context.Context, sessionID uint64, limit int) ([]model.ChatMessage, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var messages []model.ChatMessage
	if err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}
