package service

import (
	"context"
	"strings"
	"time"

	"ling-shu/internal/memory"
)

type UserMemoryService struct {
	manager *memory.UserManager
}

type ListUserMemoriesInput struct {
	TenantID  uint64
	ProjectID uint64
	UserID    uint64
	Statuses  []string
	Kinds     []string
	Page      int
	PageSize  int
}

type SaveUserMemoryInput struct {
	TenantID   uint64
	ProjectID  uint64
	UserID     uint64
	Kind       string
	Key        string
	Content    string
	Value      map[string]any
	ScopeLevel string
	ExpiresAt  *time.Time
}

type UpdateUserMemoryInput struct {
	SaveUserMemoryInput
	ID uint64
}

type UserMemoryItemInput struct {
	TenantID  uint64
	ProjectID uint64
	UserID    uint64
	ID        uint64
}

func NewUserMemoryService(manager *memory.UserManager) *UserMemoryService {
	return &UserMemoryService{manager: manager}
}

func (s *UserMemoryService) List(ctx context.Context, input ListUserMemoriesInput) (PageResult[memory.UserMemory], error) {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.UserID == 0 {
		return PageResult[memory.UserMemory]{}, ErrInvalidInput
	}
	pageInput := NewPage(input.Page, input.PageSize)
	page, pageSize := pageInput.Page, pageInput.PageSize
	items, total, err := s.manager.List(ctx, memory.UserMemoryListFilter{
		Scope:    memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID},
		Statuses: input.Statuses, Kinds: input.Kinds, Limit: pageSize, Offset: (page - 1) * pageSize,
	})
	if err != nil {
		return PageResult[memory.UserMemory]{}, err
	}
	return PageResult[memory.UserMemory]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *UserMemoryService) Save(ctx context.Context, input SaveUserMemoryInput) (memory.UserMemory, error) {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.UserID == 0 || strings.TrimSpace(input.Content) == "" ||
		(input.ProjectID == 0 && input.ScopeLevel != memory.UserMemoryScopeTenant) {
		return memory.UserMemory{}, ErrInvalidInput
	}
	operation := memory.UserMemoryOperation{
		Action: memory.UserMemoryActionRemember, Kind: input.Kind, Key: input.Key,
		Content: input.Content, Value: input.Value, ScopeLevel: input.ScopeLevel, Confidence: 1, ExpiresAt: input.ExpiresAt,
	}
	item, err := s.manager.Save(ctx, memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID}, operation, time.Now())
	if err != nil {
		return memory.UserMemory{}, err
	}
	return item, nil
}

func (s *UserMemoryService) Update(ctx context.Context, input UpdateUserMemoryInput) (memory.UserMemory, error) {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.UserID == 0 ||
		input.ID == 0 || strings.TrimSpace(input.Content) == "" {
		return memory.UserMemory{}, ErrInvalidInput
	}
	operation := memory.UserMemoryOperation{
		Action: memory.UserMemoryActionRemember, Kind: input.Kind, Key: input.Key,
		Content: input.Content, Value: input.Value, ScopeLevel: input.ScopeLevel, Confidence: 1, ExpiresAt: input.ExpiresAt,
	}
	return s.manager.Update(ctx, memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID}, input.ID, operation, time.Now())
}

func (s *UserMemoryService) Delete(ctx context.Context, input UserMemoryItemInput) error {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.UserID == 0 || input.ID == 0 {
		return ErrInvalidInput
	}
	return s.manager.Delete(ctx, memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID}, input.ID, time.Now())
}

func (s *UserMemoryService) SetCandidateStatus(ctx context.Context, input UserMemoryItemInput, confirm bool) (memory.UserMemory, error) {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.UserID == 0 || input.ID == 0 {
		return memory.UserMemory{}, ErrInvalidInput
	}
	return s.manager.SetCandidateStatus(ctx, memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID}, input.ID, confirm, time.Now())
}

func (s *UserMemoryService) Clear(ctx context.Context, input UserMemoryItemInput, includeTenant bool) (int64, error) {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.UserID == 0 {
		return 0, ErrInvalidInput
	}
	return s.manager.Clear(ctx, memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID}, includeTenant, time.Now())
}

func (s *UserMemoryService) ListEpisodes(ctx context.Context, input UserMemoryItemInput, limit int) ([]memory.SessionEpisode, error) {
	if s == nil || s.manager == nil || input.TenantID == 0 || input.ProjectID == 0 || input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	return s.manager.ListEpisodes(ctx, memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID}, limit)
}
