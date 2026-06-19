package service

import (
	"context"
	"encoding/json"
	"strings"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

type KnowledgeService struct {
	knowledgeRepo  repository.KnowledgeRepository
	logger         *zap.Logger
	indexRefresher KnowledgeIndexRefresher
}

type KnowledgeIndexRefresher interface {
	RefreshKnowledgeIndex(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) error
}

type CreateKBTermInput struct {
	TenantID   uint64
	ProjectID  uint64
	Term       string
	Aliases    []string
	Definition string
	Enabled    *bool
	CreatedBy  uint64
}

type CreateKBMetricInput struct {
	TenantID          uint64
	ProjectID         uint64
	Name              string
	Description       string
	Formula           string
	DatasourceID      uint64
	DefaultTimeColumn string
	Enabled           *bool
	CreatedBy         uint64
}

type CreateKBFewShotInput struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Question     string
	SQL          string
	Explanation  string
	Enabled      *bool
	CreatedBy    uint64
}

type ListKnowledgeInput struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Enabled      *bool
	Page         int
	PageSize     int
}

type KnowledgeItemInput struct {
	TenantID  uint64
	ProjectID uint64
	ID        uint64
}

type UpdateKnowledgeEnabledInput struct {
	KnowledgeItemInput
	Enabled bool
}

func NewKnowledgeService(knowledgeRepo repository.KnowledgeRepository) *KnowledgeService {
	return &KnowledgeService{knowledgeRepo: knowledgeRepo, logger: zap.NewNop()}
}

func (s *KnowledgeService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *KnowledgeService) SetIndexRefresher(refresher KnowledgeIndexRefresher) {
	s.indexRefresher = refresher
}

func (s *KnowledgeService) CreateTerm(ctx context.Context, input CreateKBTermInput) (*model.KBTerm, error) {
	termText := strings.TrimSpace(input.Term)
	definition := strings.TrimSpace(input.Definition)
	if input.TenantID == 0 || input.ProjectID == 0 || termText == "" || definition == "" {
		return nil, ErrInvalidInput
	}
	aliasesJSON, err := marshalOptionalStringSlice(input.Aliases)
	if err != nil {
		s.logger.Error("kb term aliases marshal failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Int("alias_count", len(input.Aliases)),
			zap.Error(err),
		)
		return nil, err
	}
	term := &model.KBTerm{
		TenantID:    input.TenantID,
		ProjectID:   input.ProjectID,
		Term:        termText,
		AliasesJSON: aliasesJSON,
		Definition:  definition,
		Enabled:     defaultEnabled(input.Enabled),
		CreatedBy:   input.CreatedBy,
	}
	if err := s.knowledgeRepo.CreateTerm(ctx, term); err != nil {
		s.logger.Error("kb term create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("term_hash", sqlHash(termText)),
			zap.Int("alias_count", len(input.Aliases)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("kb term created",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("term_id", term.ID),
		zap.String("term_hash", sqlHash(termText)),
		zap.Int("alias_count", len(input.Aliases)),
	)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "term.create")
	return term, nil
}

func (s *KnowledgeService) ListTerms(ctx context.Context, input ListKnowledgeInput) (PageResult[model.KBTerm], error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return PageResult[model.KBTerm]{}, ErrInvalidInput
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.knowledgeRepo.ListTerms(ctx, knowledgeFilter(input), p)
	if err != nil {
		s.logger.Error("kb term list failed",
			append(knowledgeListLogFields(input, p), zap.Error(err))...,
		)
		return PageResult[model.KBTerm]{}, err
	}
	return PageResult[model.KBTerm]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func (s *KnowledgeService) UpdateTermEnabled(ctx context.Context, input UpdateKnowledgeEnabledInput) error {
	if !validKnowledgeItemInput(input.KnowledgeItemInput) {
		return ErrInvalidInput
	}
	if err := s.knowledgeRepo.UpdateTermEnabled(ctx, knowledgeItemScope(input.KnowledgeItemInput), input.Enabled); err != nil {
		s.logger.Error("kb term enabled update failed",
			append(knowledgeItemLogFields(input.KnowledgeItemInput), zap.Bool("enabled", input.Enabled), zap.Error(err))...,
		)
		return err
	}
	s.logger.Info("kb term enabled updated",
		append(knowledgeItemLogFields(input.KnowledgeItemInput), zap.Bool("enabled", input.Enabled))...,
	)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "term.enabled")
	return nil
}

func (s *KnowledgeService) DeleteTerm(ctx context.Context, input KnowledgeItemInput) error {
	if !validKnowledgeItemInput(input) {
		return ErrInvalidInput
	}
	if err := s.knowledgeRepo.DeleteTerm(ctx, knowledgeItemScope(input)); err != nil {
		s.logger.Error("kb term delete failed",
			append(knowledgeItemLogFields(input), zap.Error(err))...,
		)
		return err
	}
	s.logger.Info("kb term deleted", knowledgeItemLogFields(input)...)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "term.delete")
	return nil
}

func (s *KnowledgeService) CreateMetric(ctx context.Context, input CreateKBMetricInput) (*model.KBMetric, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	formula := strings.TrimSpace(input.Formula)
	if input.TenantID == 0 || input.ProjectID == 0 || name == "" || description == "" || formula == "" {
		return nil, ErrInvalidInput
	}
	metric := &model.KBMetric{
		TenantID:          input.TenantID,
		ProjectID:         input.ProjectID,
		Name:              name,
		Description:       description,
		Formula:           formula,
		DatasourceID:      input.DatasourceID,
		DefaultTimeColumn: strings.TrimSpace(input.DefaultTimeColumn),
		Enabled:           defaultEnabled(input.Enabled),
		CreatedBy:         input.CreatedBy,
	}
	if err := s.knowledgeRepo.CreateMetric(ctx, metric); err != nil {
		s.logger.Error("kb metric create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.String("metric_hash", sqlHash(name)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("kb metric created",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Uint64("metric_id", metric.ID),
		zap.String("metric_hash", sqlHash(name)),
	)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, input.DatasourceID, "metric.create")
	return metric, nil
}

func (s *KnowledgeService) ListMetrics(ctx context.Context, input ListKnowledgeInput) (PageResult[model.KBMetric], error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return PageResult[model.KBMetric]{}, ErrInvalidInput
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.knowledgeRepo.ListMetrics(ctx, knowledgeFilter(input), p)
	if err != nil {
		s.logger.Error("kb metric list failed",
			append(knowledgeListLogFields(input, p), zap.Error(err))...,
		)
		return PageResult[model.KBMetric]{}, err
	}
	return PageResult[model.KBMetric]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func (s *KnowledgeService) UpdateMetricEnabled(ctx context.Context, input UpdateKnowledgeEnabledInput) error {
	if !validKnowledgeItemInput(input.KnowledgeItemInput) {
		return ErrInvalidInput
	}
	if err := s.knowledgeRepo.UpdateMetricEnabled(ctx, knowledgeItemScope(input.KnowledgeItemInput), input.Enabled); err != nil {
		s.logger.Error("kb metric enabled update failed",
			append(knowledgeItemLogFields(input.KnowledgeItemInput), zap.Bool("enabled", input.Enabled), zap.Error(err))...,
		)
		return err
	}
	s.logger.Info("kb metric enabled updated",
		append(knowledgeItemLogFields(input.KnowledgeItemInput), zap.Bool("enabled", input.Enabled))...,
	)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "metric.enabled")
	return nil
}

func (s *KnowledgeService) DeleteMetric(ctx context.Context, input KnowledgeItemInput) error {
	if !validKnowledgeItemInput(input) {
		return ErrInvalidInput
	}
	if err := s.knowledgeRepo.DeleteMetric(ctx, knowledgeItemScope(input)); err != nil {
		s.logger.Error("kb metric delete failed",
			append(knowledgeItemLogFields(input), zap.Error(err))...,
		)
		return err
	}
	s.logger.Info("kb metric deleted", knowledgeItemLogFields(input)...)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "metric.delete")
	return nil
}

func (s *KnowledgeService) CreateFewShot(ctx context.Context, input CreateKBFewShotInput) (*model.KBFewShotSQL, error) {
	question := strings.TrimSpace(input.Question)
	sqlText := strings.TrimSpace(input.SQL)
	if input.TenantID == 0 || input.ProjectID == 0 || question == "" || sqlText == "" {
		return nil, ErrInvalidInput
	}
	fewShot := &model.KBFewShotSQL{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		DatasourceID: input.DatasourceID,
		Question:     question,
		SQLText:      sqlText,
		Explanation:  strings.TrimSpace(input.Explanation),
		Enabled:      defaultEnabled(input.Enabled),
		CreatedBy:    input.CreatedBy,
	}
	if err := s.knowledgeRepo.CreateFewShot(ctx, fewShot); err != nil {
		s.logger.Error("kb fewshot create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.String("question_hash", sqlHash(question)),
			zap.String("sql_hash", sqlHash(sqlText)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("kb fewshot created",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Uint64("fewshot_id", fewShot.ID),
		zap.String("question_hash", sqlHash(question)),
		zap.String("sql_hash", sqlHash(sqlText)),
	)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, input.DatasourceID, "fewshot.create")
	return fewShot, nil
}

func (s *KnowledgeService) ListFewShots(ctx context.Context, input ListKnowledgeInput) (PageResult[model.KBFewShotSQL], error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return PageResult[model.KBFewShotSQL]{}, ErrInvalidInput
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.knowledgeRepo.ListFewShots(ctx, knowledgeFilter(input), p)
	if err != nil {
		s.logger.Error("kb fewshot list failed",
			append(knowledgeListLogFields(input, p), zap.Error(err))...,
		)
		return PageResult[model.KBFewShotSQL]{}, err
	}
	return PageResult[model.KBFewShotSQL]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func (s *KnowledgeService) UpdateFewShotEnabled(ctx context.Context, input UpdateKnowledgeEnabledInput) error {
	if !validKnowledgeItemInput(input.KnowledgeItemInput) {
		return ErrInvalidInput
	}
	if err := s.knowledgeRepo.UpdateFewShotEnabled(ctx, knowledgeItemScope(input.KnowledgeItemInput), input.Enabled); err != nil {
		s.logger.Error("kb fewshot enabled update failed",
			append(knowledgeItemLogFields(input.KnowledgeItemInput), zap.Bool("enabled", input.Enabled), zap.Error(err))...,
		)
		return err
	}
	s.logger.Info("kb fewshot enabled updated",
		append(knowledgeItemLogFields(input.KnowledgeItemInput), zap.Bool("enabled", input.Enabled))...,
	)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "fewshot.enabled")
	return nil
}

func (s *KnowledgeService) DeleteFewShot(ctx context.Context, input KnowledgeItemInput) error {
	if !validKnowledgeItemInput(input) {
		return ErrInvalidInput
	}
	if err := s.knowledgeRepo.DeleteFewShot(ctx, knowledgeItemScope(input)); err != nil {
		s.logger.Error("kb fewshot delete failed",
			append(knowledgeItemLogFields(input), zap.Error(err))...,
		)
		return err
	}
	s.logger.Info("kb fewshot deleted", knowledgeItemLogFields(input)...)
	s.refreshKnowledgeIndex(ctx, input.TenantID, input.ProjectID, 0, "fewshot.delete")
	return nil
}

func (s *KnowledgeService) refreshKnowledgeIndex(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64, reason string) {
	if s.indexRefresher == nil {
		return
	}
	if err := s.indexRefresher.RefreshKnowledgeIndex(ctx, tenantID, projectID, datasourceID); err != nil {
		s.logger.Warn("knowledge index refresh failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Uint64("datasource_id", datasourceID),
			zap.String("reason", reason),
			zap.Error(err),
		)
		return
	}
	s.logger.Info("knowledge index refreshed",
		zap.Uint64("tenant_id", tenantID),
		zap.Uint64("project_id", projectID),
		zap.Uint64("datasource_id", datasourceID),
		zap.String("reason", reason),
	)
}

func knowledgeListLogFields(input ListKnowledgeInput, page repository.Page) []zap.Field {
	fields := []zap.Field{
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Int("page", page.Page),
		zap.Int("page_size", page.Limit()),
	}
	if input.Enabled != nil {
		fields = append(fields, zap.Bool("enabled", *input.Enabled))
	}
	return fields
}

func knowledgeItemLogFields(input KnowledgeItemInput) []zap.Field {
	return []zap.Field{
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("knowledge_id", input.ID),
	}
}

func knowledgeFilter(input ListKnowledgeInput) repository.KnowledgeFilter {
	return repository.KnowledgeFilter{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		DatasourceID: input.DatasourceID,
		Enabled:      input.Enabled,
	}
}

func knowledgeItemScope(input KnowledgeItemInput) repository.KnowledgeItemScope {
	return repository.KnowledgeItemScope{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		ID:        input.ID,
	}
}

func validKnowledgeItemInput(input KnowledgeItemInput) bool {
	return input.TenantID > 0 && input.ProjectID > 0 && input.ID > 0
}

func marshalOptionalStringSlice(values []string) (*string, error) {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	content, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	out := string(content)
	return &out, nil
}

func defaultEnabled(enabled *bool) bool {
	if enabled == nil {
		return true
	}
	return *enabled
}
