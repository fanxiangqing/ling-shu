package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dsdriver "ling-shu/internal/datasource"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/pkg/secret"
)

func TestDatasourceServiceSyncMetadata(t *testing.T) {
	registry := dsdriver.NewRegistry()
	if err := registry.Register(syncFakeDriver{}); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	repo := &fakeDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel:     model.BaseModel{ID: 7},
			TenantID:      1,
			ProjectID:     2,
			Name:          "local",
			DBType:        "fake",
			DSNCiphertext: "dsn",
		},
	}
	service := NewDatasourceService(repo, registry)

	result, err := service.SyncMetadata(context.Background(), SyncMetadataInput{
		DatasourceID: 7,
		UserID:       9,
	})
	if err != nil {
		t.Fatalf("sync metadata: %v", err)
	}
	if result.Status != "success" || result.SchemaCount != 1 || result.TableCount != 1 || result.ColumnCount != 2 {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if repo.finishedStatus != "success" {
		t.Fatalf("expected success job, got %s", repo.finishedStatus)
	}
	if repo.syncStatus != "success" {
		t.Fatalf("expected datasource sync status success, got %s", repo.syncStatus)
	}
	if len(repo.schemas) != 1 || repo.schemas[0].SchemaName != "sales" {
		t.Fatalf("expected schema metadata")
	}
	if len(repo.tables) != 1 || repo.tables[0].Name != "orders" {
		t.Fatalf("expected table metadata")
	}
	if len(repo.tables[0].Columns) != 2 || !repo.tables[0].Columns[0].IsPrimaryKey {
		t.Fatalf("expected column metadata with primary key")
	}
	if repo.configJSON == nil || *repo.configJSON != `{"version":"fake-1.0"}` {
		t.Fatalf("expected detected version to be persisted, got %v", repo.configJSON)
	}
}

func TestDatasourceServiceDeleteReferencedDatasource(t *testing.T) {
	repo := &fakeDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel: model.BaseModel{ID: 7},
			TenantID:  1,
			Name:      "local",
			DBType:    "fake",
		},
		projectReferences: 1,
	}
	service := NewDatasourceService(repo, nil)

	err := service.Delete(context.Background(), DeleteDatasourceInput{TenantID: 1, DatasourceID: 7})
	if !errors.Is(err, ErrDatasourceInUse) {
		t.Fatalf("expected datasource in use error, got %v", err)
	}
	if repo.datasource == nil {
		t.Fatalf("referenced datasource should not be deleted")
	}
}

func TestDatasourceServiceCreateStoresEncryptedDSN(t *testing.T) {
	registry := dsdriver.NewRegistry()
	if err := registry.Register(syncFakeDriver{}); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	codec, err := secret.NewAESGCMCodec("test-dsn-secret")
	if err != nil {
		t.Fatalf("new dsn codec: %v", err)
	}
	repo := &fakeDatasourceRepository{}
	service := NewDatasourceService(repo, registry)
	service.SetDSNCodec(codec)

	datasource, err := service.Create(context.Background(), CreateDatasourceInput{
		TenantID:  1,
		Name:      "local",
		DBType:    "fake",
		DSN:       "plain-dsn",
		CreatedBy: 9,
	})
	if err != nil {
		t.Fatalf("create datasource: %v", err)
	}
	if datasource.DSNCiphertext == "plain-dsn" {
		t.Fatal("expected dsn to be stored encrypted")
	}
	plain, err := codec.Decrypt(datasource.DSNCiphertext)
	if err != nil {
		t.Fatalf("decrypt stored dsn: %v", err)
	}
	if plain != "plain-dsn" {
		t.Fatalf("expected decrypted dsn, got %s", plain)
	}
}

type syncFakeDriver struct{}

func (syncFakeDriver) Type() string {
	return "fake"
}

func (syncFakeDriver) Ping(ctx context.Context, cfg dsdriver.Config) error {
	return nil
}

func (syncFakeDriver) Introspect(ctx context.Context, cfg dsdriver.Config) (*dsdriver.Metadata, error) {
	rowCount := int64(10)
	return &dsdriver.Metadata{
		Version: "fake-1.0",
		Schemas: []dsdriver.Schema{{Name: "sales"}},
		Tables: []dsdriver.Table{{
			Schema:   "sales",
			Name:     "orders",
			Type:     "table",
			Comment:  "订单表",
			RowCount: &rowCount,
			Columns: []dsdriver.Column{
				{Name: "id", OrdinalPosition: 1, DataType: "bigint", ColumnType: "bigint unsigned", IsPrimaryKey: true},
				{Name: "amount", OrdinalPosition: 2, DataType: "decimal", ColumnType: "decimal(18,2)", Nullable: false},
			},
		}},
	}, nil
}

func (syncFakeDriver) Query(ctx context.Context, cfg dsdriver.Config, sqlText string, maxRows int) (*dsdriver.QueryResult, error) {
	return &dsdriver.QueryResult{}, nil
}

type fakeDatasourceRepository struct {
	datasource        *model.Datasource
	schemas           []model.MetadataSchema
	tables            []model.MetadataTable
	finishedStatus    string
	syncStatus        string
	projectReferences int64
	configJSON        *string
}

func (r *fakeDatasourceRepository) Create(ctx context.Context, datasource *model.Datasource) error {
	r.datasource = datasource
	datasource.ID = 1
	return nil
}

func (r *fakeDatasourceRepository) ListByProject(ctx context.Context, tenantID uint64, projectID uint64, page repository.Page) ([]model.Datasource, int64, error) {
	if r.datasource == nil {
		return nil, 0, nil
	}
	return []model.Datasource{*r.datasource}, 1, nil
}

func (r *fakeDatasourceRepository) ListByTenant(ctx context.Context, tenantID uint64, page repository.Page) ([]model.Datasource, int64, error) {
	if r.datasource == nil {
		return nil, 0, nil
	}
	return []model.Datasource{*r.datasource}, 1, nil
}

func (r *fakeDatasourceRepository) GetByID(ctx context.Context, id uint64) (*model.Datasource, error) {
	if r.datasource == nil || r.datasource.ID != id {
		return nil, errors.New("not found")
	}
	return r.datasource, nil
}

func (r *fakeDatasourceRepository) BindToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceIDs []uint64, createdBy uint64) error {
	return nil
}

func (r *fakeDatasourceRepository) IsBoundToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) (bool, error) {
	return true, nil
}

func (r *fakeDatasourceRepository) CountProjectReferences(ctx context.Context, tenantID uint64, datasourceID uint64) (int64, error) {
	return r.projectReferences, nil
}

func (r *fakeDatasourceRepository) Delete(ctx context.Context, tenantID uint64, datasourceID uint64) error {
	r.datasource = nil
	return nil
}

func (r *fakeDatasourceRepository) UpdateConfigJSON(ctx context.Context, id uint64, configJSON *string) error {
	r.configJSON = configJSON
	if r.datasource != nil {
		r.datasource.ConfigJSON = configJSON
	}
	return nil
}

func (r *fakeDatasourceRepository) UpdateSyncStatus(ctx context.Context, id uint64, status string, syncedAt *time.Time) error {
	r.syncStatus = status
	return nil
}

func (r *fakeDatasourceRepository) CreateSyncJob(ctx context.Context, job *model.MetadataSyncJob) error {
	job.ID = 99
	return nil
}

func (r *fakeDatasourceRepository) FinishSyncJob(ctx context.Context, id uint64, status string, errorMessage string) error {
	r.finishedStatus = status
	return nil
}

func (r *fakeDatasourceRepository) ReplaceMetadata(ctx context.Context, datasource *model.Datasource, schemas []model.MetadataSchema, tables []model.MetadataTable) error {
	r.schemas = schemas
	r.tables = tables
	return nil
}

func (r *fakeDatasourceRepository) ListMetadataTables(ctx context.Context, datasourceID uint64, page repository.Page, withColumns bool) ([]model.MetadataTable, int64, error) {
	return r.tables, int64(len(r.tables)), nil
}

func (r *fakeDatasourceRepository) GetMetadataTableDetail(ctx context.Context, datasourceID uint64, tableID uint64) (*model.MetadataTable, error) {
	for _, table := range r.tables {
		if table.ID == tableID || tableID == 0 {
			return &table, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *fakeDatasourceRepository) GetMetadataColumn(ctx context.Context, datasourceID uint64, columnID uint64) (*model.MetadataColumn, error) {
	for i := range r.tables {
		for j := range r.tables[i].Columns {
			if r.tables[i].Columns[j].ID == columnID {
				return &r.tables[i].Columns[j], nil
			}
		}
	}
	return nil, errors.New("not found")
}

func (r *fakeDatasourceRepository) UpdateMetadataTableComment(ctx context.Context, datasourceID uint64, tableID uint64, comment string) (*model.MetadataTable, error) {
	for i := range r.tables {
		if r.tables[i].ID == tableID {
			r.tables[i].CommentText = comment
			return &r.tables[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (r *fakeDatasourceRepository) UpdateMetadataColumnComment(ctx context.Context, datasourceID uint64, columnID uint64, comment string) (*model.MetadataColumn, error) {
	for i := range r.tables {
		for j := range r.tables[i].Columns {
			if r.tables[i].Columns[j].ID == columnID {
				r.tables[i].Columns[j].CommentText = comment
				return &r.tables[i].Columns[j], nil
			}
		}
	}
	return nil, errors.New("not found")
}
