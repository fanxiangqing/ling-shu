package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	auditpkg "ling-shu/internal/audit"
	dsdriver "ling-shu/internal/datasource"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/pkg/secret"
)

func TestQueryServiceExecuteSQLBlocksUnsafeSQL(t *testing.T) {
	queryRepo := &queryFakeRepository{}
	driver := &queryFakeDriver{}
	registry := dsdriver.NewRegistry()
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	service := NewQueryService(&queryDatasourceRepository{}, queryRepo, registry, nil)

	result, err := service.ExecuteSQL(context.Background(), ExecuteSQLInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		UserID:       4,
		SQL:          "delete from orders",
	})
	if err != nil {
		t.Fatalf("execute sql: %v", err)
	}
	if result.Execution.Status != "blocked" {
		t.Fatalf("expected blocked execution, got %s", result.Execution.Status)
	}
	if result.Review.Passed {
		t.Fatal("expected review to block unsafe sql")
	}
	if len(driver.queries) != 0 {
		t.Fatalf("expected driver not to be called, got %d calls", len(driver.queries))
	}
	if len(queryRepo.reviews) != 1 || queryRepo.reviews[0].Passed {
		t.Fatalf("expected one failed review, got %+v", queryRepo.reviews)
	}
	if queryRepo.executions[0].Status != "blocked" {
		t.Fatalf("expected persisted execution blocked, got %s", queryRepo.executions[0].Status)
	}
}

func TestQueryServiceExecuteSQLRunsReviewedSelect(t *testing.T) {
	queryRepo := &queryFakeRepository{}
	codec, err := secret.NewAESGCMCodec("test-dsn-secret")
	if err != nil {
		t.Fatalf("new dsn codec: %v", err)
	}
	encryptedDSN, err := codec.Encrypt("dsn")
	if err != nil {
		t.Fatalf("encrypt dsn: %v", err)
	}
	datasourceRepo := &queryDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel:     model.BaseModel{ID: 3},
			TenantID:      1,
			ProjectID:     2,
			Name:          "orders",
			DBType:        "fake",
			DSNCiphertext: encryptedDSN,
		},
	}
	driver := &queryFakeDriver{
		result: &dsdriver.QueryResult{
			Columns: []string{"id", "amount"},
			Rows: []map[string]any{
				{"id": int64(1), "amount": "99.00"},
			},
		},
	}
	registry := dsdriver.NewRegistry()
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	auditRecorder := &recordingAuditRecorder{}
	service := NewQueryService(datasourceRepo, queryRepo, registry, nil, auditRecorder)
	service.SetDSNCodec(codec)

	result, err := service.ExecuteSQL(context.Background(), ExecuteSQLInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		UserID:       4,
		Question:     "订单金额",
		SQL:          "select id, amount from orders",
		MaxRows:      1,
		RequestID:    "rid-1",
		AuditOrigin: AuditOrigin{
			Source:         AuditSourceEmbed,
			EmbedAppID:     "emb_test",
			EmbedSessionID: 11,
			ExternalUserID: "u-1",
			SessionKey:     "dashboard:123",
		},
	})
	if err != nil {
		t.Fatalf("execute sql: %v", err)
	}
	if result.Execution.Status != "success" {
		t.Fatalf("expected success execution, got %s", result.Execution.Status)
	}
	if !result.Review.Passed {
		t.Fatalf("expected review passed, got %+v", result.Review)
	}
	if len(driver.queries) != 1 {
		t.Fatalf("expected one query, got %d", len(driver.queries))
	}
	if len(driver.dsns) != 1 || driver.dsns[0] != "dsn" {
		t.Fatalf("expected decrypted dsn, got %+v", driver.dsns)
	}
	if driver.queries[0] != "select id, amount from orders LIMIT 1" {
		t.Fatalf("unexpected reviewed sql: %s", driver.queries[0])
	}
	if result.Execution.RowCount == nil || *result.Execution.RowCount != 1 {
		t.Fatalf("expected row count 1, got %+v", result.Execution.RowCount)
	}
	if result.Answer == "" {
		t.Fatal("expected natural language answer")
	}
	if result.SpeechSummary == "" || strings.Contains(result.SpeechSummary, "订单金额") {
		t.Fatalf("expected speech summary without question replay, got %q", result.SpeechSummary)
	}
	if strings.Contains(result.Answer, "推荐使用") {
		t.Fatalf("answer should describe rendered chart instead of recommending one, got %q", result.Answer)
	}
	if queryRepo.lastFinish.ResultPreviewJSON == nil || *queryRepo.lastFinish.ResultPreviewJSON == "" {
		t.Fatal("expected result preview json to be persisted")
	}
	if len(result.Rows) != 1 || result.Rows[0]["amount"] != "99.00" {
		t.Fatalf("unexpected result rows: %+v", result.Rows)
	}
	if result.Chart.Type == "" || result.Execution.ChartType != result.Chart.Type {
		t.Fatalf("expected chart suggestion to be persisted, result=%+v execution=%+v", result.Chart, result.Execution.ChartType)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(auditRecorder.events))
	}
	if auditRecorder.events[0].EventType != auditpkg.EventQueryExecute || auditRecorder.events[0].ResourceID != result.Execution.ID || auditRecorder.events[0].RequestID != "rid-1" {
		t.Fatalf("unexpected audit event: %+v", auditRecorder.events[0])
	}
	payload := auditRecorder.events[0].Payload
	if payload["source"] != AuditSourceEmbed || payload["app_id"] != "emb_test" || payload["embed_app_id"] != "emb_test" || payload["embed_session_id"] != uint64(11) || payload["external_user_id"] != "u-1" || payload["session_key"] != "dashboard:123" {
		t.Fatalf("unexpected embed audit payload: %+v", payload)
	}
}

func TestQueryServiceExecuteSQLUsesQueryLock(t *testing.T) {
	queryRepo := &queryFakeRepository{}
	datasourceRepo := &queryDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel:     model.BaseModel{ID: 3},
			TenantID:      1,
			ProjectID:     2,
			Name:          "orders",
			DBType:        "fake",
			DSNCiphertext: "dsn",
		},
	}
	driver := &queryFakeDriver{}
	registry := dsdriver.NewRegistry()
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	lockStore := newQueryFakeCacheStore()
	service := NewQueryService(datasourceRepo, queryRepo, registry, nil)
	service.SetQueryLock(lockStore, "test:query:lock", time.Minute)

	_, err := service.ExecuteSQL(context.Background(), ExecuteSQLInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		UserID:       4,
		SQL:          "select * from orders",
	})
	if err != nil {
		t.Fatalf("execute sql: %v", err)
	}
	if lockStore.setNXCalls != 1 {
		t.Fatalf("expected one lock acquire, got %d", lockStore.setNXCalls)
	}
	if lockStore.delCalls != 1 {
		t.Fatalf("expected one lock release, got %d", lockStore.delCalls)
	}
	if len(lockStore.values) != 0 {
		t.Fatalf("expected lock store to be empty after release, got %+v", lockStore.values)
	}
}

func TestQueryServiceExecuteSQLRejectsDuplicateRunningQuery(t *testing.T) {
	queryRepo := &queryFakeRepository{}
	datasourceRepo := &queryDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel:     model.BaseModel{ID: 3},
			TenantID:      1,
			ProjectID:     2,
			Name:          "orders",
			DBType:        "fake",
			DSNCiphertext: "dsn",
		},
	}
	driver := &queryFakeDriver{}
	registry := dsdriver.NewRegistry()
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	lockStore := newQueryFakeCacheStore()
	lockStore.busy = true
	service := NewQueryService(datasourceRepo, queryRepo, registry, nil)
	service.SetQueryLock(lockStore, "test:query:lock", time.Minute)

	result, err := service.ExecuteSQL(context.Background(), ExecuteSQLInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		UserID:       4,
		SQL:          "select * from orders",
	})
	if !errors.Is(err, ErrQueryAlreadyRunning) {
		t.Fatalf("expected ErrQueryAlreadyRunning, got %v", err)
	}
	if len(driver.queries) != 0 {
		t.Fatalf("expected driver not called, got %d", len(driver.queries))
	}
	if result == nil || result.Execution == nil || result.Execution.Status != "failed" {
		t.Fatalf("expected failed execution result, got %+v", result)
	}
	if queryRepo.executions[0].Status != "failed" {
		t.Fatalf("expected persisted execution failed, got %s", queryRepo.executions[0].Status)
	}
}

func TestQueryServiceExecuteSQLBlocksSensitiveColumn(t *testing.T) {
	queryRepo := &queryFakeRepository{}
	datasourceRepo := &queryDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel:     model.BaseModel{ID: 3},
			TenantID:      1,
			ProjectID:     2,
			Name:          "users",
			DBType:        "fake",
			DSNCiphertext: "dsn",
		},
	}
	driver := &queryFakeDriver{}
	registry := dsdriver.NewRegistry()
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	service := NewQueryService(datasourceRepo, queryRepo, registry, nil)
	service.SetSecurityRepository(&queryFakeSecurityRepository{
		columns: []model.SensitiveColumn{
			{TenantID: 1, ProjectID: 2, DatasourceID: 3, Table: "users", ColumnName: "mobile", RiskLevel: "high", Enabled: true},
		},
	})

	result, err := service.ExecuteSQL(context.Background(), ExecuteSQLInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		UserID:       4,
		Question:     "用户手机号",
		SQL:          "select id, mobile from users",
	})
	if err != nil {
		t.Fatalf("execute sql: %v", err)
	}
	if result.Execution.Status != "blocked" || result.Review.Passed {
		t.Fatalf("expected sensitive column blocked, got execution=%+v review=%+v", result.Execution, result.Review)
	}
	if !strings.Contains(result.Review.BlockedReason, "敏感字段") {
		t.Fatalf("expected sensitive field reason, got %s", result.Review.BlockedReason)
	}
	if len(driver.queries) != 0 {
		t.Fatalf("expected driver not called, got %d", len(driver.queries))
	}
}

func TestQueryServiceExecuteSQLRejectsDatasourceOutsideProject(t *testing.T) {
	queryRepo := &queryFakeRepository{}
	datasourceRepo := &queryDatasourceRepository{
		datasource: &model.Datasource{
			BaseModel:     model.BaseModel{ID: 3},
			TenantID:      9,
			ProjectID:     9,
			Name:          "orders",
			DBType:        "fake",
			DSNCiphertext: "dsn",
		},
	}
	driver := &queryFakeDriver{}
	registry := dsdriver.NewRegistry()
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register fake driver: %v", err)
	}
	service := NewQueryService(datasourceRepo, queryRepo, registry, nil)

	_, err := service.ExecuteSQL(context.Background(), ExecuteSQLInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		UserID:       4,
		SQL:          "select * from orders",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if len(driver.queries) != 0 {
		t.Fatalf("expected driver not to be called, got %d calls", len(driver.queries))
	}
	if queryRepo.executions[0].Status != "failed" {
		t.Fatalf("expected persisted execution failed, got %s", queryRepo.executions[0].Status)
	}
}

type queryFakeRepository struct {
	nextID     uint64
	executions []*model.QueryExecution
	reviews    []*model.SQLReviewResult
	lastFinish repository.QueryExecutionFinish
}

func (r *queryFakeRepository) CreateExecution(ctx context.Context, execution *model.QueryExecution) error {
	r.nextID++
	execution.ID = r.nextID
	execution.CreatedAt = time.Now()
	copied := *execution
	r.executions = append(r.executions, &copied)
	return nil
}

func (r *queryFakeRepository) FinishExecution(ctx context.Context, id uint64, updates repository.QueryExecutionFinish) error {
	r.lastFinish = updates
	for _, execution := range r.executions {
		if execution.ID == id {
			execution.Status = updates.Status
			execution.FinalSQL = updates.FinalSQL
			execution.SQLHash = updates.SQLHash
			execution.RowCount = updates.RowCount
			execution.DurationMS = updates.DurationMS
			execution.ResultPreviewJSON = updates.ResultPreviewJSON
			execution.ErrorMessage = updates.ErrorMessage
			execution.FinishedAt = &updates.FinishedAt
			return nil
		}
	}
	return errors.New("execution not found")
}

func (r *queryFakeRepository) CreateReviewResult(ctx context.Context, result *model.SQLReviewResult) error {
	copied := *result
	copied.ID = uint64(len(r.reviews) + 1)
	r.reviews = append(r.reviews, &copied)
	return nil
}

func (r *queryFakeRepository) ListExecutions(ctx context.Context, filter repository.QueryExecutionFilter, page repository.Page) ([]model.QueryExecution, int64, error) {
	items := make([]model.QueryExecution, 0, len(r.executions))
	for _, execution := range r.executions {
		items = append(items, *execution)
	}
	return items, int64(len(items)), nil
}

type queryDatasourceRepository struct {
	datasource *model.Datasource
}

func (r *queryDatasourceRepository) Create(ctx context.Context, datasource *model.Datasource) error {
	r.datasource = datasource
	return nil
}

func (r *queryDatasourceRepository) ListByProject(ctx context.Context, tenantID uint64, projectID uint64, page repository.Page) ([]model.Datasource, int64, error) {
	if r.datasource == nil {
		return nil, 0, nil
	}
	return []model.Datasource{*r.datasource}, 1, nil
}

func (r *queryDatasourceRepository) ListByTenant(ctx context.Context, tenantID uint64, page repository.Page) ([]model.Datasource, int64, error) {
	if r.datasource == nil {
		return nil, 0, nil
	}
	return []model.Datasource{*r.datasource}, 1, nil
}

func (r *queryDatasourceRepository) GetByID(ctx context.Context, id uint64) (*model.Datasource, error) {
	if r.datasource == nil || r.datasource.ID != id {
		return nil, errors.New("datasource not found")
	}
	return r.datasource, nil
}

func (r *queryDatasourceRepository) BindToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceIDs []uint64, createdBy uint64) error {
	return nil
}

func (r *queryDatasourceRepository) IsBoundToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) (bool, error) {
	return true, nil
}

func (r *queryDatasourceRepository) CountProjectReferences(ctx context.Context, tenantID uint64, datasourceID uint64) (int64, error) {
	return 0, nil
}

func (r *queryDatasourceRepository) Delete(ctx context.Context, tenantID uint64, datasourceID uint64) error {
	return nil
}

func (r *queryDatasourceRepository) UpdateConfigJSON(ctx context.Context, id uint64, configJSON *string) error {
	return nil
}

func (r *queryDatasourceRepository) UpdateSyncStatus(ctx context.Context, id uint64, status string, syncedAt *time.Time) error {
	return nil
}

func (r *queryDatasourceRepository) CreateSyncJob(ctx context.Context, job *model.MetadataSyncJob) error {
	return nil
}

func (r *queryDatasourceRepository) FinishSyncJob(ctx context.Context, id uint64, status string, errorMessage string) error {
	return nil
}

func (r *queryDatasourceRepository) ReplaceMetadata(ctx context.Context, datasource *model.Datasource, schemas []model.MetadataSchema, tables []model.MetadataTable) error {
	return nil
}

func (r *queryDatasourceRepository) ListMetadataTables(ctx context.Context, datasourceID uint64, page repository.Page, withColumns bool) ([]model.MetadataTable, int64, error) {
	return nil, 0, nil
}

func (r *queryDatasourceRepository) GetMetadataTableDetail(ctx context.Context, datasourceID uint64, tableID uint64) (*model.MetadataTable, error) {
	return nil, errors.New("not found")
}

func (r *queryDatasourceRepository) GetMetadataColumn(ctx context.Context, datasourceID uint64, columnID uint64) (*model.MetadataColumn, error) {
	return nil, errors.New("not found")
}

func (r *queryDatasourceRepository) UpdateMetadataTableComment(ctx context.Context, datasourceID uint64, tableID uint64, comment string) (*model.MetadataTable, error) {
	return nil, errors.New("not found")
}

func (r *queryDatasourceRepository) UpdateMetadataColumnComment(ctx context.Context, datasourceID uint64, columnID uint64, comment string) (*model.MetadataColumn, error) {
	return nil, errors.New("not found")
}

type queryFakeDriver struct {
	queries []string
	dsns    []string
	result  *dsdriver.QueryResult
}

type queryFakeSecurityRepository struct {
	tables  []model.SensitiveTable
	columns []model.SensitiveColumn
}

func (r *queryFakeSecurityRepository) ListSensitiveTables(ctx context.Context, filter repository.SensitiveRuleFilter) ([]model.SensitiveTable, error) {
	return r.tables, nil
}

func (r *queryFakeSecurityRepository) ListSensitiveColumns(ctx context.Context, filter repository.SensitiveRuleFilter) ([]model.SensitiveColumn, error) {
	return r.columns, nil
}

type queryFakeCacheStore struct {
	values     map[string]string
	busy       bool
	setNXCalls int
	delCalls   int
}

func newQueryFakeCacheStore() *queryFakeCacheStore {
	return &queryFakeCacheStore{values: map[string]string{}}
}

func (s *queryFakeCacheStore) Ping(ctx context.Context) error {
	return nil
}

func (s *queryFakeCacheStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return 1, nil
}

func (s *queryFakeCacheStore) Get(ctx context.Context, key string) (string, bool, error) {
	value, ok := s.values[key]
	return value, ok, nil
}

func (s *queryFakeCacheStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	s.values[key] = value
	return nil
}

func (s *queryFakeCacheStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	s.setNXCalls++
	if s.busy {
		return false, nil
	}
	if _, ok := s.values[key]; ok {
		return false, nil
	}
	s.values[key] = value
	return true, nil
}

func (s *queryFakeCacheStore) Del(ctx context.Context, key string) error {
	s.delCalls++
	delete(s.values, key)
	return nil
}

func (s *queryFakeCacheStore) Close() error {
	return nil
}

func (d *queryFakeDriver) Type() string {
	return "fake"
}

func (d *queryFakeDriver) Ping(ctx context.Context, cfg dsdriver.Config) error {
	return nil
}

func (d *queryFakeDriver) Introspect(ctx context.Context, cfg dsdriver.Config) (*dsdriver.Metadata, error) {
	return &dsdriver.Metadata{}, nil
}

func (d *queryFakeDriver) Query(ctx context.Context, cfg dsdriver.Config, sqlText string, maxRows int) (*dsdriver.QueryResult, error) {
	d.queries = append(d.queries, sqlText)
	d.dsns = append(d.dsns, cfg.DSN)
	if d.result == nil {
		return &dsdriver.QueryResult{}, nil
	}
	return d.result, nil
}
