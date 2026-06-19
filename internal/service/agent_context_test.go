package service

import (
	"context"
	"testing"
	"time"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestProjectAgentContextBuilderLoadsProjectMetadata(t *testing.T) {
	repo := &agentContextFakeDatasourceRepository{
		datasources: []model.Datasource{
			{BaseModel: model.BaseModel{ID: 7}, TenantID: 1, ProjectID: 2, Name: "orders", DBType: "mysql", ConfigJSON: testStringPtr(`{"version":"5.7"}`), Status: "active"},
			{BaseModel: model.BaseModel{ID: 8}, TenantID: 1, ProjectID: 2, Name: "archive", DBType: "mysql", Status: "disabled"},
		},
		tables: map[uint64][]model.MetadataTable{
			7: {
				{
					ID:          11,
					SchemaName:  "shop",
					Name:        "orders",
					CommentText: "订单表",
					Columns: []model.MetadataColumn{
						{ColumnName: "id", ColumnType: "bigint unsigned", IsPrimaryKey: true, CommentText: "订单ID"},
						{ColumnName: "pay_amount", DataType: "decimal", CommentText: "支付金额"},
					},
				},
			},
		},
	}
	builder := NewProjectAgentContextBuilder(repo)

	context, err := builder.BuildAgentContext(context.Background(), AgentContextInput{
		TenantID:              1,
		ProjectID:             2,
		SelectedDatasourceIDs: []uint64{7},
	})
	if err != nil {
		t.Fatalf("build agent context: %v", err)
	}
	if len(context.Datasources) != 1 {
		t.Fatalf("expected one active datasource, got %+v", context.Datasources)
	}
	datasource := context.Datasources[0]
	if datasource.ID != 7 || datasource.Name != "orders" || datasource.Dialect != "mysql" || datasource.Version != "5.7" || datasource.Role != "selected" {
		t.Fatalf("unexpected datasource context: %+v", datasource)
	}
	if len(datasource.Tables) != 1 || datasource.Tables[0].Name != "orders" {
		t.Fatalf("expected orders table context, got %+v", datasource.Tables)
	}
	if len(datasource.Tables[0].Columns) != 2 || datasource.Tables[0].Columns[1].Name != "pay_amount" {
		t.Fatalf("expected table columns, got %+v", datasource.Tables[0].Columns)
	}
	if len(datasource.Tables[0].PrimaryKeys) != 1 || datasource.Tables[0].PrimaryKeys[0] != "id" {
		t.Fatalf("expected primary key id, got %+v", datasource.Tables[0].PrimaryKeys)
	}
	if len(context.Permission.AllowedDatasourceIDs) != 1 || context.Permission.AllowedDatasourceIDs[0] != 7 {
		t.Fatalf("expected allowed datasource ids from context, got %+v", context.Permission.AllowedDatasourceIDs)
	}
}

func TestProjectAgentContextBuilderUsesMetadataCache(t *testing.T) {
	repo := &agentContextFakeDatasourceRepository{
		datasources: []model.Datasource{
			{BaseModel: model.BaseModel{ID: 7}, TenantID: 1, ProjectID: 2, Name: "orders", DBType: "mysql", Status: "active"},
		},
		tables: map[uint64][]model.MetadataTable{
			7: {
				{
					ID:         11,
					SchemaName: "shop",
					Name:       "orders",
					Columns: []model.MetadataColumn{
						{ColumnName: "id", ColumnType: "bigint unsigned", IsPrimaryKey: true},
					},
				},
			},
		},
	}
	cacheStore := newQueryFakeCacheStore()
	builder := NewProjectAgentContextBuilder(repo)
	builder.SetCache(cacheStore, "test:project:meta", time.Minute)

	first, err := builder.BuildAgentContext(context.Background(), AgentContextInput{
		TenantID:  1,
		ProjectID: 2,
	})
	if err != nil {
		t.Fatalf("build first agent context: %v", err)
	}
	second, err := builder.BuildAgentContext(context.Background(), AgentContextInput{
		TenantID:  1,
		ProjectID: 2,
	})
	if err != nil {
		t.Fatalf("build second agent context: %v", err)
	}
	if len(first.Datasources) != 1 || len(second.Datasources) != 1 {
		t.Fatalf("expected cached datasource context, got first=%+v second=%+v", first.Datasources, second.Datasources)
	}
	if repo.listByProjectCalls != 1 {
		t.Fatalf("expected project datasource list to be loaded once, got %d", repo.listByProjectCalls)
	}
	if repo.listMetadataCalls != 1 {
		t.Fatalf("expected metadata tables to be loaded once, got %d", repo.listMetadataCalls)
	}
}

func testStringPtr(value string) *string {
	return &value
}

type agentContextFakeDatasourceRepository struct {
	datasources        []model.Datasource
	tables             map[uint64][]model.MetadataTable
	listByProjectCalls int
	listMetadataCalls  int
}

func (r *agentContextFakeDatasourceRepository) Create(ctx context.Context, datasource *model.Datasource) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) ListByProject(ctx context.Context, tenantID uint64, projectID uint64, page repository.Page) ([]model.Datasource, int64, error) {
	r.listByProjectCalls++
	out := make([]model.Datasource, 0, len(r.datasources))
	for _, datasource := range r.datasources {
		if datasource.TenantID == tenantID && datasource.ProjectID == projectID {
			out = append(out, datasource)
		}
	}
	return out, int64(len(out)), nil
}

func (r *agentContextFakeDatasourceRepository) ListByTenant(ctx context.Context, tenantID uint64, page repository.Page) ([]model.Datasource, int64, error) {
	out := make([]model.Datasource, 0, len(r.datasources))
	for _, datasource := range r.datasources {
		if datasource.TenantID == tenantID {
			out = append(out, datasource)
		}
	}
	return out, int64(len(out)), nil
}

func (r *agentContextFakeDatasourceRepository) GetByID(ctx context.Context, id uint64) (*model.Datasource, error) {
	return nil, nil
}

func (r *agentContextFakeDatasourceRepository) BindToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceIDs []uint64, createdBy uint64) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) IsBoundToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) (bool, error) {
	return true, nil
}

func (r *agentContextFakeDatasourceRepository) CountProjectReferences(ctx context.Context, tenantID uint64, datasourceID uint64) (int64, error) {
	return 0, nil
}

func (r *agentContextFakeDatasourceRepository) Delete(ctx context.Context, tenantID uint64, datasourceID uint64) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) UpdateConfigJSON(ctx context.Context, id uint64, configJSON *string) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) UpdateSyncStatus(ctx context.Context, id uint64, status string, syncedAt *time.Time) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) CreateSyncJob(ctx context.Context, job *model.MetadataSyncJob) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) FinishSyncJob(ctx context.Context, id uint64, status string, errorMessage string) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) ReplaceMetadata(ctx context.Context, datasource *model.Datasource, schemas []model.MetadataSchema, tables []model.MetadataTable) error {
	return nil
}

func (r *agentContextFakeDatasourceRepository) ListMetadataTables(ctx context.Context, datasourceID uint64, page repository.Page, withColumns bool) ([]model.MetadataTable, int64, error) {
	r.listMetadataCalls++
	tables := r.tables[datasourceID]
	return tables, int64(len(tables)), nil
}

func (r *agentContextFakeDatasourceRepository) GetMetadataTableDetail(ctx context.Context, datasourceID uint64, tableID uint64) (*model.MetadataTable, error) {
	return nil, nil
}

func (r *agentContextFakeDatasourceRepository) GetMetadataColumn(ctx context.Context, datasourceID uint64, columnID uint64) (*model.MetadataColumn, error) {
	return nil, nil
}

func (r *agentContextFakeDatasourceRepository) UpdateMetadataTableComment(ctx context.Context, datasourceID uint64, tableID uint64, comment string) (*model.MetadataTable, error) {
	return nil, nil
}

func (r *agentContextFakeDatasourceRepository) UpdateMetadataColumnComment(ctx context.Context, datasourceID uint64, columnID uint64, comment string) (*model.MetadataColumn, error) {
	return nil, nil
}
