package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ling-shu/internal/asr"
	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/config"
	"ling-shu/internal/handler"
	"ling-shu/internal/llm"
	"ling-shu/internal/query"
	"ling-shu/internal/repository"
	"ling-shu/internal/service"
	"ling-shu/internal/tts"
	"ling-shu/pkg/response"

	"go.uber.org/zap"
)

func TestHealthRoute(t *testing.T) {
	cfg := config.Default()
	tokenManager := authpkg.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL)
	healthService := service.NewHealthService(nil)
	authService := service.NewAuthService(repository.NewUserRepository(nil), tokenManager)
	permissionService := service.NewPermissionService(repository.NewPermissionRepository(nil))
	tenantService := service.NewTenantService(repository.NewTenantRepository(nil))
	projectService := service.NewProjectService(repository.NewProjectRepository(nil))
	datasourceService := service.NewDatasourceService(repository.NewDatasourceRepository(nil), nil)
	queryService := service.NewQueryService(repository.NewDatasourceRepository(nil), repository.NewQueryRepository(nil), nil, nil)
	providerService := service.NewProviderService(
		noopLLMProvider{},
		noopASRProvider{},
		noopTTSProvider{},
	)
	chatService := service.NewChatService(repository.NewChatRepository(nil), newNoopAgentRunner(), nil)
	voiceService := service.NewVoiceService(providerService, chatService)
	knowledgeService := service.NewKnowledgeService(repository.NewKnowledgeRepository(nil))
	ragService := service.NewRAGService(nil, nil)
	auditService := service.NewAuditService(repository.NewAuditRepository(nil))
	providerConfigService := service.NewProviderConfigService(repository.NewProviderConfigRepository(nil))
	healthHandler := handler.NewHealthHandler(healthService)
	authHandler := handler.NewAuthHandler(authService)
	permissionHandler := handler.NewPermissionHandler(permissionService)
	tenantHandler := handler.NewTenantHandler(tenantService)
	projectHandler := handler.NewProjectHandler(projectService)
	datasourceHandler := handler.NewDatasourceHandler(datasourceService)
	chatHandler := handler.NewChatHandler(chatService)
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeService)
	ragHandler := handler.NewRAGHandler(ragService)
	queryHandler := handler.NewQueryHandler(queryService)
	auditHandler := handler.NewAuditHandler(auditService, queryService)
	providerHandler := handler.NewProviderHandler(providerService)
	providerConfigHandler := handler.NewProviderConfigHandler(providerConfigService)
	engine := New(Dependencies{
		Config:                &cfg,
		TokenManager:          tokenManager,
		PermissionChecker:     permissionService,
		DatasourceScope:       datasourceService,
		HealthHandler:         healthHandler,
		AuthHandler:           authHandler,
		PermissionHandler:     permissionHandler,
		TenantHandler:         tenantHandler,
		ProjectHandler:        projectHandler,
		DatasourceHandler:     datasourceHandler,
		ProviderHandler:       providerHandler,
		ProviderConfigHandler: providerConfigHandler,
		ChatHandler:           chatHandler,
		VoiceHandler:          handler.NewVoiceHandler(voiceService),
		KnowledgeHandler:      knowledgeHandler,
		RAGHandler:            ragHandler,
		QueryHandler:          queryHandler,
		QueryAgentHandler:     newNoopQueryAgentHandler(),
		AuditHandler:          auditHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body response.Body
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != response.CodeOK {
		t.Fatalf("expected code %d, got %d", response.CodeOK, body.Code)
	}
	if body.RequestID == "" {
		t.Fatal("expected request_id to be set")
	}
}

func TestProviderSummaryRoute(t *testing.T) {
	cfg := config.Default()
	tokenManager := authpkg.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL)
	providerService := service.NewProviderService(
		noopLLMProvider{},
		noopASRProvider{},
		noopTTSProvider{},
	)
	chatService := service.NewChatService(
		repository.NewChatRepository(nil),
		newNoopAgentRunner(),
		nil,
	)
	engine := New(Dependencies{
		Config:       &cfg,
		TokenManager: tokenManager,
		PermissionChecker: service.NewPermissionService(
			repository.NewPermissionRepository(nil),
		),
		DatasourceScope: service.NewDatasourceService(repository.NewDatasourceRepository(nil), nil),
		HealthHandler:   handler.NewHealthHandler(service.NewHealthService(nil)),
		AuthHandler:     handler.NewAuthHandler(service.NewAuthService(repository.NewUserRepository(nil), tokenManager)),
		PermissionHandler: handler.NewPermissionHandler(service.NewPermissionService(
			repository.NewPermissionRepository(nil),
		)),
		TenantHandler:  handler.NewTenantHandler(service.NewTenantService(repository.NewTenantRepository(nil))),
		ProjectHandler: handler.NewProjectHandler(service.NewProjectService(repository.NewProjectRepository(nil))),
		DatasourceHandler: handler.NewDatasourceHandler(service.NewDatasourceService(
			repository.NewDatasourceRepository(nil),
			nil,
		)),
		ChatHandler:  handler.NewChatHandler(chatService),
		VoiceHandler: handler.NewVoiceHandler(service.NewVoiceService(providerService, chatService)),
		KnowledgeHandler: handler.NewKnowledgeHandler(service.NewKnowledgeService(
			repository.NewKnowledgeRepository(nil),
		)),
		RAGHandler: handler.NewRAGHandler(service.NewRAGService(nil, nil)),
		QueryHandler: handler.NewQueryHandler(service.NewQueryService(
			repository.NewDatasourceRepository(nil),
			repository.NewQueryRepository(nil),
			nil,
			nil,
		)),
		AuditHandler: handler.NewAuditHandler(
			service.NewAuditService(repository.NewAuditRepository(nil)),
			service.NewQueryService(repository.NewDatasourceRepository(nil), repository.NewQueryRepository(nil), nil, nil),
		),
		ProviderHandler: handler.NewProviderHandler(providerService),
		ProviderConfigHandler: handler.NewProviderConfigHandler(service.NewProviderConfigService(
			repository.NewProviderConfigRepository(nil),
		)),
		QueryAgentHandler: newNoopQueryAgentHandler(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func newNoopQueryAgentHandler() *handler.QueryAgentHandler {
	renderer, _ := query.NewPromptRendererFromTemplates(map[string]string{
		"agent/planner_system.tmpl":           "planner project={{.ProjectID}}",
		"agent/datasource_router_system.tmpl": "router project={{.ProjectID}}",
		"agent/text2sql_system.tmpl":          "project={{.ProjectID}}",
		"dialect/mysql.tmpl":                  "mysql rules",
	})
	agent := query.NewReactAgent(noopLLMProvider{}, query.NewSQLReviewer(200, 1000), renderer, zap.NewNop())
	return handler.NewQueryAgentHandler(service.NewQueryAgentService(agent))
}

func newNoopAgentRunner() service.AgentRunner {
	return service.NewQueryAgentService(query.NewReactAgent(noopLLMProvider{}, query.NewSQLReviewer(200, 1000), mustRouterPromptRenderer(), zap.NewNop()))
}

func mustRouterPromptRenderer() *query.PromptRenderer {
	renderer, _ := query.NewPromptRendererFromTemplates(map[string]string{
		"agent/planner_system.tmpl":           "planner project={{.ProjectID}}",
		"agent/datasource_router_system.tmpl": "router project={{.ProjectID}}",
		"agent/text2sql_system.tmpl":          "project={{.ProjectID}}",
		"dialect/mysql.tmpl":                  "mysql rules",
	})
	return renderer
}

type noopLLMProvider struct{}

func (noopLLMProvider) Name() string                  { return "aliyun" }
func (noopLLMProvider) Configured() bool              { return false }
func (noopLLMProvider) DefaultChatModel() string      { return "qwen-plus" }
func (noopLLMProvider) DefaultEmbeddingModel() string { return "text-embedding-v4" }
func (noopLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, nil
}
func (noopLLMProvider) StreamChat(ctx context.Context, req llm.ChatRequest, onEvent func(llm.ChatStreamEvent) error) error {
	return nil
}
func (noopLLMProvider) Embeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, nil
}

type noopASRProvider struct{}

func (noopASRProvider) Name() string         { return "aliyun" }
func (noopASRProvider) Configured() bool     { return false }
func (noopASRProvider) DefaultModel() string { return "nls-realtime-asr" }
func (noopASRProvider) Transcribe(ctx context.Context, req asr.TranscribeRequest) (*asr.TranscribeResponse, error) {
	return nil, nil
}
func (noopASRProvider) StreamTranscribe(ctx context.Context, req asr.TranscribeRequest, onEvent func(asr.TranscribeStreamEvent) error) error {
	return nil
}
func (noopASRProvider) GetTask(ctx context.Context, taskID string) (*asr.TranscribeResponse, error) {
	return nil, nil
}

type noopTTSProvider struct{}

func (noopTTSProvider) Name() string         { return "aliyun" }
func (noopTTSProvider) Configured() bool     { return false }
func (noopTTSProvider) DefaultModel() string { return "nls-tts" }
func (noopTTSProvider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (*tts.SynthesizeResponse, error) {
	return nil, nil
}
func (noopTTSProvider) StreamSynthesize(ctx context.Context, req tts.SynthesizeRequest, onEvent func(tts.SynthesizeStreamEvent) error) error {
	return nil
}
