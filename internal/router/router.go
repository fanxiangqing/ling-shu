package router

import (
	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/config"
	"ling-shu/internal/handler"
	"ling-shu/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Dependencies struct {
	Config                *config.Config
	Logger                *zap.Logger
	TokenManager          *authpkg.TokenManager
	PermissionChecker     middleware.PermissionChecker
	DatasourceScope       middleware.DatasourceScopeResolver
	RateLimitStore        middleware.RateLimitStore
	RateLimitConfig       middleware.RateLimitConfig
	HealthHandler         *handler.HealthHandler
	AuthHandler           *handler.AuthHandler
	PermissionHandler     *handler.PermissionHandler
	TenantHandler         *handler.TenantHandler
	ProjectHandler        *handler.ProjectHandler
	DatasourceHandler     *handler.DatasourceHandler
	ProviderHandler       *handler.ProviderHandler
	ProviderConfigHandler *handler.ProviderConfigHandler
	ChatHandler           *handler.ChatHandler
	VoiceHandler          *handler.VoiceHandler
	KnowledgeHandler      *handler.KnowledgeHandler
	RAGHandler            *handler.RAGHandler
	QueryHandler          *handler.QueryHandler
	QueryAgentHandler     *handler.QueryAgentHandler
	AuditHandler          *handler.AuditHandler
}

func New(deps Dependencies) *gin.Engine {
	gin.SetMode(deps.Config.Server.Mode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	if deps.Logger != nil {
		r.Use(middleware.AccessLog(deps.Logger))
	}

	r.GET("/healthz", deps.HealthHandler.Liveness)
	r.GET("/readyz", deps.HealthHandler.Readiness)

	v1 := r.Group("/api/v1")
	if deps.TokenManager != nil {
		v1.Use(middleware.OptionalAuth(deps.TokenManager))
	}
	if deps.RateLimitConfig.Enabled {
		v1.Use(middleware.RateLimit(deps.RateLimitStore, deps.RateLimitConfig, deps.Logger))
	}
	authz := func(code string, opts ...middleware.PermissionOption) []gin.HandlerFunc {
		return []gin.HandlerFunc{
			middleware.AuthRequired(deps.TokenManager),
			middleware.RequirePermission(deps.PermissionChecker, code, opts...),
		}
	}
	datasourceAuthz := func(code string) []gin.HandlerFunc {
		return authz(code, middleware.WithDatasourceScope(deps.DatasourceScope, "id"), middleware.RequireTenantScope())
	}
	{
		v1.GET("/health", deps.HealthHandler.Readiness)

		auth := v1.Group("/auth")
		{
			auth.POST("/users", deps.AuthHandler.CreateUser)
			auth.GET("/users", deps.AuthHandler.ListUsers)
			auth.POST("/login", deps.AuthHandler.Login)
		}

		permissions := v1.Group("/permissions")
		{
			permissions.GET("/roles", deps.PermissionHandler.ListRoles)
			permissions.GET("", deps.PermissionHandler.ListPermissions)
			permissions.POST("/role-bindings", append(authz("tenant.manage", middleware.RequireTenantScope()), deps.PermissionHandler.BindRole)...)
			permissions.GET("/role-bindings", deps.PermissionHandler.ListRoleBindings)
			permissions.POST("/check", deps.PermissionHandler.Check)
		}

		tenants := v1.Group("/tenants")
		{
			tenants.GET("", middleware.AuthRequired(deps.TokenManager), deps.TenantHandler.List)
			tenants.POST("", deps.TenantHandler.Create)
			tenants.POST("/:tenant_id/users", append(authz("tenant.manage", middleware.RequireTenantScope()), deps.AuthHandler.CreateTenantUser)...)
			tenants.GET("/:tenant_id/members", append(authz("tenant.manage", middleware.RequireTenantScope()), deps.AuthHandler.ListTenantMembers)...)
			tenants.POST("/:tenant_id/members", append(authz("tenant.manage", middleware.RequireTenantScope()), deps.AuthHandler.AddTenantMember)...)
			tenants.PATCH("/:tenant_id/members/:member_id/status", append(authz("tenant.manage", middleware.RequireTenantScope()), deps.AuthHandler.UpdateTenantMemberStatus)...)
			tenants.DELETE("/:tenant_id/members/:member_id", append(authz("tenant.manage", middleware.RequireTenantScope()), deps.AuthHandler.DeleteTenantMember)...)
			tenants.GET("/:tenant_id/datasources", append(authz("datasource.manage", middleware.RequireTenantScope()), deps.DatasourceHandler.List)...)
			tenants.POST("/:tenant_id/datasources", append(authz("datasource.manage", middleware.RequireTenantScope()), deps.DatasourceHandler.Create)...)
			tenants.GET("/:tenant_id/chat/sessions", append(authz("chat.use", middleware.RequireTenantScope()), deps.ChatHandler.ListSessions)...)
			tenants.POST("/:tenant_id/chat/sessions", append(authz("chat.use", middleware.RequireTenantScope()), deps.ChatHandler.CreateSession)...)
		}

		projects := v1.Group("/projects")
		{
			projects.GET("", middleware.AuthRequired(deps.TokenManager), deps.ProjectHandler.List)
			projects.POST("", append(authz("project.manage", middleware.RequireTenantScope()), deps.ProjectHandler.Create)...)
			projects.DELETE("/:project_id", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProjectHandler.Delete)...)
			projects.GET("/:project_id/members", append(authz("project.manage", middleware.RequireProjectScope()), deps.AuthHandler.ListProjectMembers)...)
			projects.POST("/:project_id/members", append(authz("project.manage", middleware.RequireProjectScope()), deps.AuthHandler.AddProjectMember)...)
			projects.PATCH("/:project_id/members/:member_id/status", append(authz("project.manage", middleware.RequireProjectScope()), deps.AuthHandler.UpdateProjectMemberStatus)...)
			projects.DELETE("/:project_id/members/:member_id", append(authz("project.manage", middleware.RequireProjectScope()), deps.AuthHandler.DeleteProjectMember)...)
			projects.GET("/:project_id/llm-config", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProviderConfigHandler.GetLLM)...)
			projects.PUT("/:project_id/llm-config", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProviderConfigHandler.UpsertLLM)...)
			projects.GET("/:project_id/asr-config", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProviderConfigHandler.GetASR)...)
			projects.PUT("/:project_id/asr-config", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProviderConfigHandler.UpsertASR)...)
			projects.GET("/:project_id/tts-config", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProviderConfigHandler.GetTTS)...)
			projects.PUT("/:project_id/tts-config", append(authz("project.manage", middleware.RequireProjectScope()), deps.ProviderConfigHandler.UpsertTTS)...)
			projects.GET("/:project_id/datasources", deps.DatasourceHandler.List)
			projects.POST("/:project_id/datasources", append(authz("datasource.manage", middleware.RequireProjectScope()), deps.DatasourceHandler.Create)...)
			projects.GET("/:project_id/chat/sessions", deps.ChatHandler.ListSessions)
			projects.POST("/:project_id/chat/sessions", deps.ChatHandler.CreateSession)
			projects.GET("/:project_id/kb/terms", deps.KnowledgeHandler.ListTerms)
			projects.POST("/:project_id/kb/terms", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.CreateTerm)...)
			projects.PATCH("/:project_id/kb/terms/:id/enabled", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.UpdateTermEnabled)...)
			projects.DELETE("/:project_id/kb/terms/:id", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.DeleteTerm)...)
			projects.GET("/:project_id/kb/metrics", deps.KnowledgeHandler.ListMetrics)
			projects.POST("/:project_id/kb/metrics", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.CreateMetric)...)
			projects.PATCH("/:project_id/kb/metrics/:id/enabled", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.UpdateMetricEnabled)...)
			projects.DELETE("/:project_id/kb/metrics/:id", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.DeleteMetric)...)
			projects.GET("/:project_id/kb/fewshots", deps.KnowledgeHandler.ListFewShots)
			projects.POST("/:project_id/kb/fewshots", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.CreateFewShot)...)
			projects.PATCH("/:project_id/kb/fewshots/:id/enabled", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.UpdateFewShotEnabled)...)
			projects.DELETE("/:project_id/kb/fewshots/:id", append(authz("kb.manage", middleware.RequireProjectScope()), deps.KnowledgeHandler.DeleteFewShot)...)
			projects.POST("/:project_id/rag/rebuild", append(authz("kb.manage", middleware.RequireProjectScope()), deps.RAGHandler.Rebuild)...)
			projects.POST("/:project_id/rag/search", append(authz("chat.use", middleware.RequireProjectScope()), deps.RAGHandler.Search)...)
		}

		datasources := v1.Group("/datasources")
		{
			datasources.POST("/test-connection", append(authz("datasource.manage", middleware.RequireTenantScope()), deps.DatasourceHandler.TestConnectionWithConfig)...)
			datasources.POST("/:id/test", append(datasourceAuthz("datasource.manage"), deps.DatasourceHandler.TestConnection)...)
			datasources.POST("/:id/sync", append(datasourceAuthz("metadata.sync"), deps.DatasourceHandler.SyncMetadata)...)
			datasources.GET("/:id/metadata/tables", append(datasourceAuthz("query.view_sql"), deps.DatasourceHandler.ListMetadataTables)...)
			datasources.GET("/:id/metadata/tables/:table_id", append(datasourceAuthz("query.view_sql"), deps.DatasourceHandler.GetMetadataTableDetail)...)
			datasources.PATCH("/:id/metadata/tables/:table_id/comment", append(datasourceAuthz("datasource.manage"), deps.DatasourceHandler.UpdateMetadataTableComment)...)
			datasources.PATCH("/:id/metadata/columns/:column_id/comment", append(datasourceAuthz("datasource.manage"), deps.DatasourceHandler.UpdateMetadataColumnComment)...)
			datasources.DELETE("/:id", append(datasourceAuthz("datasource.manage"), deps.DatasourceHandler.Delete)...)
		}

		providers := v1.Group("/providers")
		{
			providers.GET("", deps.ProviderHandler.Summary)
			providers.POST("/llm/chat", deps.ProviderHandler.Chat)
			providers.POST("/llm/chat/stream", deps.ProviderHandler.StreamChat)
			providers.POST("/asr/transcribe", deps.ProviderHandler.Transcribe)
			providers.POST("/asr/transcribe/stream", deps.ProviderHandler.StreamTranscribe)
			providers.GET("/asr/tasks/:task_id", deps.ProviderHandler.GetTranscribeTask)
			providers.POST("/tts/synthesize", deps.ProviderHandler.Synthesize)
			providers.POST("/tts/synthesize/stream", deps.ProviderHandler.StreamSynthesize)
		}

		chat := v1.Group("/chat")
		{
			chat.GET("/sessions/:session_id/messages", deps.ChatHandler.ListMessages)
			chat.POST("/sessions/:session_id/messages", deps.ChatHandler.SendMessage)
			chat.POST("/sessions/:session_id/messages/stream", deps.ChatHandler.StreamMessage)
			chat.POST("/sessions/:session_id/voice", deps.VoiceHandler.Chat)
			chat.POST("/sessions/:session_id/voice/stream", deps.VoiceHandler.StreamChat)
			chat.GET("/sessions/:session_id/voice/realtime", deps.VoiceHandler.RealtimeChat)
		}

		queryGroup := v1.Group("/query")
		{
			queryGroup.POST("/review", append(authz("query.view_sql", middleware.RequireProjectScope()), deps.QueryHandler.Review)...)
			queryGroup.POST("/execute", append(authz("query.execute", middleware.RequireProjectScope()), deps.QueryHandler.Execute)...)
			queryGroup.GET("/history", append(authz("query.view_sql", middleware.RequireTenantScope()), deps.QueryHandler.History)...)
		}

		queryAgent := queryGroup.Group("/agent")
		{
			queryAgent.POST("/ask", append(authz("chat.use", middleware.RequireProjectScope()), deps.QueryAgentHandler.Ask)...)
			queryAgent.POST("/ask/stream", append(authz("chat.use", middleware.RequireProjectScope()), deps.QueryAgentHandler.StreamAsk)...)
		}

		audit := v1.Group("/audit")
		{
			audit.GET("/logs", append(authz("audit.view", middleware.RequireTenantScope()), deps.AuditHandler.ListLogs)...)
			audit.GET("/query-executions", append(authz("audit.view", middleware.RequireTenantScope()), deps.AuditHandler.QueryExecutions)...)
		}
	}

	return r
}
