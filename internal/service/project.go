package service

import (
	"context"
	"strings"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ProjectService struct {
	projectRepo repository.ProjectRepository
	logger      *zap.Logger
}

type CreateProjectInput struct {
	TenantID      uint64
	Name          string
	Code          string
	Description   string
	DatasourceIDs []uint64
	CreatedBy     uint64
}

type DeleteProjectInput struct {
	TenantID  uint64
	ProjectID uint64
}

func NewProjectService(projectRepo repository.ProjectRepository) *ProjectService {
	return &ProjectService{projectRepo: projectRepo, logger: zap.NewNop()}
}

func (s *ProjectService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *ProjectService) Create(ctx context.Context, input CreateProjectInput) (*model.Project, error) {
	name := strings.TrimSpace(input.Name)
	code := strings.TrimSpace(input.Code)
	datasourceIDs := nonZeroIDs(input.DatasourceIDs)
	if input.TenantID == 0 || name == "" || code == "" || len(datasourceIDs) == 0 {
		return nil, ErrInvalidInput
	}

	project := &model.Project{
		TenantID:    input.TenantID,
		Name:        name,
		Code:        code,
		Description: strings.TrimSpace(input.Description),
		Status:      "active",
		CreatedBy:   input.CreatedBy,
	}
	if err := s.projectRepo.CreateWithBindings(ctx, project, datasourceIDs, input.CreatedBy); err != nil {
		s.logger.Error("project create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("created_by", input.CreatedBy),
			zap.String("project_code", code),
			zap.Uint64s("datasource_ids", datasourceIDs),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("project created",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", project.ID),
		zap.Uint64("created_by", input.CreatedBy),
		zap.String("project_code", code),
		zap.Uint64s("datasource_ids", datasourceIDs),
	)
	return project, nil
}

func (s *ProjectService) Delete(ctx context.Context, input DeleteProjectInput) error {
	if input.ProjectID == 0 {
		return ErrInvalidInput
	}
	if input.TenantID > 0 {
		project, err := s.projectRepo.GetByID(ctx, input.ProjectID)
		if err != nil {
			s.logger.Error("project delete scope lookup failed",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Error(err),
			)
			return err
		}
		if project.TenantID != input.TenantID {
			s.logger.Warn("project delete scope rejected",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Uint64("actual_tenant_id", project.TenantID),
			)
			return gorm.ErrRecordNotFound
		}
	}
	if err := s.projectRepo.Delete(ctx, input.TenantID, input.ProjectID); err != nil {
		s.logger.Error("project delete failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return err
	}
	s.logger.Info("project deleted",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
	)
	return nil
}

func (s *ProjectService) List(ctx context.Context, tenantID uint64, page int, pageSize int) (PageResult[model.Project], error) {
	p := NewPage(page, pageSize)
	items, total, err := s.projectRepo.List(ctx, tenantID, p)
	if err != nil {
		s.logger.Error("project list failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[model.Project]{}, err
	}
	return PageResult[model.Project]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func nonZeroIDs(values []uint64) []uint64 {
	out := make([]uint64, 0, len(values))
	seen := map[uint64]bool{}
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
