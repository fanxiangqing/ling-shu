package service

import (
	"context"
	"errors"
	"strings"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrDatasourceInUse     = errors.New("datasource is referenced by project")
	ErrQueryAlreadyRunning = errors.New("same query is already running")
	ErrSecretEncryptFailed = errors.New("secret encrypt failed")
	ErrSecretDecryptFailed = errors.New("secret decrypt failed")
)

type TenantService struct {
	tenantRepo repository.TenantRepository
	logger     *zap.Logger
}

type CreateTenantInput struct {
	Name string
	Code string
}

func NewTenantService(tenantRepo repository.TenantRepository) *TenantService {
	return &TenantService{tenantRepo: tenantRepo, logger: zap.NewNop()}
}

func (s *TenantService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *TenantService) Create(ctx context.Context, input CreateTenantInput) (*model.Tenant, error) {
	name := strings.TrimSpace(input.Name)
	code := strings.TrimSpace(input.Code)
	if name == "" || code == "" {
		return nil, ErrInvalidInput
	}

	tenant := &model.Tenant{
		Name:   name,
		Code:   code,
		Status: "active",
	}
	if err := s.tenantRepo.Create(ctx, tenant); err != nil {
		s.logger.Error("tenant create failed",
			zap.String("tenant_code", code),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("tenant created",
		zap.Uint64("tenant_id", tenant.ID),
		zap.String("tenant_code", code),
	)
	return tenant, nil
}

func (s *TenantService) List(ctx context.Context, page int, pageSize int) (PageResult[model.Tenant], error) {
	p := NewPage(page, pageSize)
	items, total, err := s.tenantRepo.List(ctx, p)
	if err != nil {
		s.logger.Error("tenant list failed",
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[model.Tenant]{}, err
	}
	return PageResult[model.Tenant]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}
