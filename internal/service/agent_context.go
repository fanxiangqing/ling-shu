package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ling-shu/internal/cache"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

const (
	agentContextDatasourceLimit = 50
	agentContextTableLimit      = 80
	agentContextColumnLimit     = 80
)

type AgentContextBuilder interface {
	BuildAgentContext(ctx context.Context, input AgentContextInput) (AgentContext, error)
}

type AgentContextInput struct {
	TenantID              uint64
	ProjectID             uint64
	DatasourceID          uint64
	SelectedDatasourceIDs []uint64
	Datasources           []query.AgentDatasource
	Permission            query.AgentPermission
}

type AgentContext struct {
	Datasources []query.AgentDatasource
	Permission  query.AgentPermission
}

type ProjectAgentContextBuilder struct {
	datasourceRepo repository.DatasourceRepository
	cacheStore     cache.Store
	cachePrefix    string
	cacheTTL       time.Duration
	logger         *zap.Logger
}

func NewProjectAgentContextBuilder(datasourceRepo repository.DatasourceRepository) *ProjectAgentContextBuilder {
	return &ProjectAgentContextBuilder{
		datasourceRepo: datasourceRepo,
		cachePrefix:    "ling-shu:project:meta",
		cacheTTL:       10 * time.Minute,
		logger:         zap.NewNop(),
	}
}

func (b *ProjectAgentContextBuilder) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	b.logger = logger
}

func (b *ProjectAgentContextBuilder) SetCache(store cache.Store, prefix string, ttl time.Duration) {
	b.cacheStore = store
	if strings.TrimSpace(prefix) != "" {
		b.cachePrefix = prefix
	}
	if ttl > 0 {
		b.cacheTTL = ttl
	}
}

func (b *ProjectAgentContextBuilder) BuildAgentContext(ctx context.Context, input AgentContextInput) (AgentContext, error) {
	datasources := append([]query.AgentDatasource(nil), input.Datasources...)
	permission := input.Permission
	if len(datasources) == 0 {
		if cached, ok := b.cachedAgentContext(ctx, input); ok {
			return cached, nil
		}
		loaded, err := b.loadProjectDatasources(ctx, input)
		if err != nil {
			return AgentContext{}, err
		}
		datasources = loaded
	}
	if len(permission.AllowedDatasourceIDs) == 0 {
		permission.AllowedDatasourceIDs = datasourceIDs(datasources)
	}
	out := AgentContext{
		Datasources: datasources,
		Permission:  permission,
	}
	if len(input.Datasources) == 0 {
		b.saveAgentContextCache(ctx, input, out)
	}
	return out, nil
}

func (b *ProjectAgentContextBuilder) cachedAgentContext(ctx context.Context, input AgentContextInput) (AgentContext, bool) {
	if b == nil || b.cacheStore == nil {
		return AgentContext{}, false
	}
	key := b.cacheKey(input)
	payload, ok, err := b.cacheStore.Get(ctx, key)
	if err != nil {
		b.logger.Warn("agent context cache read failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return AgentContext{}, false
	}
	if !ok || strings.TrimSpace(payload) == "" {
		return AgentContext{}, false
	}
	var out AgentContext
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		b.logger.Warn("agent context cache decode failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return AgentContext{}, false
	}
	return out, true
}

func (b *ProjectAgentContextBuilder) saveAgentContextCache(ctx context.Context, input AgentContextInput, value AgentContext) {
	if b == nil || b.cacheStore == nil {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		b.logger.Warn("agent context cache encode failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return
	}
	if err := b.cacheStore.Set(ctx, b.cacheKey(input), string(payload), b.cacheTTL); err != nil {
		b.logger.Warn("agent context cache write failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
	}
}

func (b *ProjectAgentContextBuilder) cacheKey(input AgentContextInput) string {
	raw := fmt.Sprintf("tenant=%d:project=%d:datasource=%d:selected=%s:allowed=%s",
		input.TenantID,
		input.ProjectID,
		input.DatasourceID,
		uint64ListKey(input.SelectedDatasourceIDs),
		uint64ListKey(input.Permission.AllowedDatasourceIDs),
	)
	return cache.BuildKey(b.cachePrefix, raw)
}

func (b *ProjectAgentContextBuilder) loadProjectDatasources(ctx context.Context, input AgentContextInput) ([]query.AgentDatasource, error) {
	if b == nil || b.datasourceRepo == nil {
		return nil, nil
	}
	if input.TenantID == 0 || input.ProjectID == 0 {
		return nil, ErrInvalidInput
	}
	items, _, err := b.datasourceRepo.ListByProject(ctx, input.TenantID, input.ProjectID, repository.Page{
		Page:     1,
		PageSize: agentContextDatasourceLimit,
	})
	if err != nil {
		return nil, err
	}

	allowedSet := uint64Set(input.Permission.AllowedDatasourceIDs)
	out := make([]query.AgentDatasource, 0, len(items))
	for _, item := range items {
		if item.Status != "" && item.Status != "active" {
			continue
		}
		if len(allowedSet) > 0 && !allowedSet[item.ID] {
			continue
		}
		agentDatasource, err := b.datasourceToAgent(ctx, item, input)
		if err != nil {
			return nil, err
		}
		out = append(out, agentDatasource)
	}
	return out, nil
}

func (b *ProjectAgentContextBuilder) datasourceToAgent(ctx context.Context, datasource model.Datasource, input AgentContextInput) (query.AgentDatasource, error) {
	tables, err := b.loadAgentTables(ctx, datasource.ID)
	if err != nil {
		return query.AgentDatasource{}, err
	}
	dialect := dialectFromDBType(datasource.DBType)
	return query.AgentDatasource{
		ID:        datasource.ID,
		Name:      datasource.Name,
		Type:      strings.ToLower(strings.TrimSpace(datasource.DBType)),
		Dialect:   dialect,
		Version:   datasourceVersion(datasource),
		Role:      datasourceRole(datasource.ID, input),
		IsDefault: datasource.ID == input.DatasourceID,
		Tables:    tables,
	}, nil
}

func datasourceVersion(datasource model.Datasource) string {
	if datasource.ConfigJSON == nil || strings.TrimSpace(*datasource.ConfigJSON) == "" {
		return ""
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(*datasource.ConfigJSON), &config); err != nil {
		return ""
	}
	value, ok := config["version"]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func (b *ProjectAgentContextBuilder) loadAgentTables(ctx context.Context, datasourceID uint64) ([]query.AgentTable, error) {
	tables, _, err := b.datasourceRepo.ListMetadataTables(ctx, datasourceID, repository.Page{
		Page:     1,
		PageSize: agentContextTableLimit,
	}, true)
	if err != nil {
		return nil, err
	}
	out := make([]query.AgentTable, 0, len(tables))
	for _, table := range tables {
		out = append(out, metadataTableToAgent(table))
	}
	return out, nil
}

func metadataTableToAgent(table model.MetadataTable) query.AgentTable {
	columns := make([]query.AgentColumn, 0, min(len(table.Columns), agentContextColumnLimit))
	primaryKeys := make([]string, 0, 4)
	for idx, column := range table.Columns {
		if idx < agentContextColumnLimit {
			columns = append(columns, query.AgentColumn{
				Name:    column.ColumnName,
				Type:    firstNonEmpty(column.ColumnType, column.DataType),
				Comment: column.CommentText,
			})
		}
		if column.IsPrimaryKey {
			primaryKeys = append(primaryKeys, column.ColumnName)
		}
	}
	indexes := make([]query.AgentIndex, 0, len(table.Indexes))
	for _, index := range table.Indexes {
		indexes = append(indexes, query.AgentIndex{
			Name:    index.IndexName,
			Type:    index.IndexType,
			Unique:  index.UniqueIndex,
			Columns: parseStringArrayJSON(index.ColumnsJSON),
		})
	}
	foreignKeys := make([]query.AgentForeignKey, 0, len(table.ForeignKeys))
	for _, foreignKey := range table.ForeignKeys {
		foreignKeys = append(foreignKeys, query.AgentForeignKey{
			Name:             foreignKey.ConstraintName,
			Column:           foreignKey.ColumnName,
			ReferencedSchema: foreignKey.ReferencedSchema,
			ReferencedTable:  foreignKey.ReferencedTable,
			ReferencedColumn: foreignKey.ReferencedColumn,
		})
	}
	return query.AgentTable{
		Schema:      table.SchemaName,
		Name:        table.Name,
		Comment:     table.CommentText,
		Columns:     columns,
		PrimaryKeys: primaryKeys,
		Indexes:     indexes,
		ForeignKeys: foreignKeys,
	}
}

func parseStringArrayJSON(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil
	}
	return out
}

func datasourceRole(datasourceID uint64, input AgentContextInput) string {
	if datasourceID == input.DatasourceID {
		return "selected"
	}
	for _, id := range input.SelectedDatasourceIDs {
		if id == datasourceID {
			return "selected"
		}
	}
	return "available"
}

func datasourceIDs(datasources []query.AgentDatasource) []uint64 {
	ids := make([]uint64, 0, len(datasources))
	seen := map[uint64]bool{}
	for _, datasource := range datasources {
		if datasource.ID == 0 || seen[datasource.ID] {
			continue
		}
		seen[datasource.ID] = true
		ids = append(ids, datasource.ID)
	}
	return ids
}

func uint64Set(values []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	for _, value := range values {
		if value > 0 {
			out[value] = true
		}
	}
	return out
}

func uint64ListKey(values []uint64) string {
	copied := append([]uint64(nil), values...)
	sort.Slice(copied, func(i int, j int) bool {
		return copied[i] < copied[j]
	})
	parts := make([]string, 0, len(copied))
	for _, value := range copied {
		if value == 0 {
			continue
		}
		parts = append(parts, fmt.Sprint(value))
	}
	return strings.Join(parts, ",")
}

func dialectFromDBType(dbType string) string {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "postgres", "postgresql":
		return "postgresql"
	case "oracle":
		return "oracle"
	case "sqlserver", "mssql":
		return "sqlserver"
	case "kingbase", "kingbasees":
		return "kingbase"
	case "dm", "dm8", "dameng":
		return "dm8"
	case "clickhouse":
		return "clickhouse"
	case "doris":
		return "doris"
	default:
		if strings.TrimSpace(dbType) == "" {
			return "mysql"
		}
		return strings.ToLower(strings.TrimSpace(dbType))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
