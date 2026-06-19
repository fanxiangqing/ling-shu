package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"ling-shu/internal/asr"
	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/cache"
	"ling-shu/internal/config"
	"ling-shu/internal/database"
	"ling-shu/internal/datasource"
	"ling-shu/internal/handler"
	"ling-shu/internal/llm"
	"ling-shu/internal/middleware"
	"ling-shu/internal/query"
	"ling-shu/internal/rag"
	"ling-shu/internal/repository"
	"ling-shu/internal/router"
	"ling-shu/internal/service"
	"ling-shu/internal/tts"
	"ling-shu/pkg/secret"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type App struct {
	server      *http.Server
	cacheStore  cache.Store
	db          *gorm.DB
	vectorStore rag.VectorStore
}

func BuildApplication(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*App, error) {
	db, err := database.OpenMySQL(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	repos := newRepositories(db)
	providers := newProviders(cfg)
	cacheStore := newCacheStore(cfg, logger)

	services, err := newServices(ctx, cfg, logger, db, repos, providers, cacheStore)
	if err != nil {
		_ = closeStore(cacheStore)
		_ = database.Close(db)
		return nil, err
	}

	engine := router.New(newRouterDependencies(cfg, logger, services, newHandlers(services), cacheStore))
	return &App{
		server: &http.Server{
			Addr:              cfg.Server.Addr(),
			Handler:           engine,
			ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
			ReadTimeout:       cfg.Server.ReadTimeout,
			WriteTimeout:      cfg.Server.WriteTimeout,
			IdleTimeout:       cfg.Server.IdleTimeout,
		},
		cacheStore:  cacheStore,
		db:          db,
		vectorStore: services.vectorStore,
	}, nil
}

func (a *App) Server() *http.Server {
	if a == nil {
		return nil
	}
	return a.server
}

func (a *App) Close(logger *zap.Logger) {
	if a == nil {
		return
	}
	if a.vectorStore != nil {
		if err := a.vectorStore.Close(); err != nil {
			logger.Warn("close milvus rag store failed", zap.Error(err))
		}
	}
	if err := closeStore(a.cacheStore); err != nil {
		logger.Warn("close redis cache store failed", zap.Error(err))
	}
	if a.db != nil {
		if err := database.Close(a.db); err != nil {
			logger.Warn("close database failed", zap.Error(err))
		}
	}
}

type repositories struct {
	tenant         repository.TenantRepository
	user           repository.UserRepository
	permission     repository.PermissionRepository
	project        repository.ProjectRepository
	datasource     repository.DatasourceRepository
	providerConfig repository.ProviderConfigRepository
	query          repository.QueryRepository
	security       repository.SecurityRepository
	chat           repository.ChatRepository
	knowledge      repository.KnowledgeRepository
	rag            repository.RAGRepository
	audit          repository.AuditRepository
}

func newRepositories(db *gorm.DB) repositories {
	return repositories{
		tenant:         repository.NewTenantRepository(db),
		user:           repository.NewUserRepository(db),
		permission:     repository.NewPermissionRepository(db),
		project:        repository.NewProjectRepository(db),
		datasource:     repository.NewDatasourceRepository(db),
		providerConfig: repository.NewProviderConfigRepository(db),
		query:          repository.NewQueryRepository(db),
		security:       repository.NewSecurityRepository(db),
		chat:           repository.NewChatRepository(db),
		knowledge:      repository.NewKnowledgeRepository(db),
		rag:            repository.NewRAGRepository(db),
		audit:          repository.NewAuditRepository(db),
	}
}

type providers struct {
	llm llm.Provider
	asr asr.Provider
	tts tts.Provider
}

func newProviders(cfg *config.Config) providers {
	return providers{
		llm: llm.NewAliyunProvider(llm.AliyunConfig{
			APIKey:         cfg.Providers.LLM.APIKey,
			BaseURL:        cfg.Providers.LLM.BaseURL,
			ChatModel:      cfg.Providers.LLM.ChatModel,
			EmbeddingModel: cfg.Providers.LLM.EmbeddingModel,
			Timeout:        cfg.Providers.LLM.Timeout,
		}),
		asr: buildASRProvider(cfg),
		tts: buildTTSProvider(cfg),
	}
}

type services struct {
	tokenManager   *authpkg.TokenManager
	permission     *service.PermissionService
	datasource     *service.DatasourceService
	provider       *service.ProviderService
	providerConfig *service.ProviderConfigService
	query          *service.QueryService
	queryAgent     *service.QueryAgentService
	rag            *service.RAGService
	audit          *service.AuditService
	health         *service.HealthService
	auth           *service.AuthService
	tenant         *service.TenantService
	project        *service.ProjectService
	chat           *service.ChatService
	voice          *service.VoiceService
	knowledge      *service.KnowledgeService
	vectorStore    rag.VectorStore
}

func newServices(ctx context.Context, cfg *config.Config, logger *zap.Logger, db *gorm.DB, repos repositories, providers providers, cacheStore cache.Store) (services, error) {
	datasourceRegistry := datasource.DefaultRegistry()
	dsnCodec, err := secret.NewAESGCMCodec(cfg.Security.DSNSecret)
	if err != nil {
		return services{}, fmt.Errorf("init datasource secret codec: %w", err)
	}
	promptRenderer, err := query.NewPromptRendererFromDir(cfg.Prompts.Dir)
	if err != nil {
		return services{}, fmt.Errorf("load prompts from %s: %w", cfg.Prompts.Dir, err)
	}
	vectorStore, err := newVectorStore(ctx, cfg)
	if err != nil {
		return services{}, err
	}

	tokenManager := authpkg.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL)
	healthOptions := []service.HealthOption{}
	if cacheStore != nil {
		healthOptions = append(healthOptions, service.WithRedisPinger(cacheStore))
	}
	healthService := service.NewHealthService(db, healthOptions...)
	authService := service.NewAuthService(repos.user, tokenManager, service.WithSignupWorkspace("tenant_admin"))
	permissionService := service.NewPermissionService(repos.permission)
	tenantService := service.NewTenantService(repos.tenant)
	projectService := service.NewProjectService(repos.project)

	datasourceService := service.NewDatasourceService(repos.datasource, datasourceRegistry)
	datasourceService.SetDSNCodec(dsnCodec)

	agentContextBuilder := service.NewProjectAgentContextBuilder(repos.datasource)
	agentContextBuilder.SetLogger(logger)
	if cacheStore != nil && cfg.Redis.MetadataCache.Enabled {
		agentContextBuilder.SetCache(cacheStore, cfg.Redis.MetadataCache.Prefix, cfg.Redis.MetadataCache.TTL)
	}
	providerConfigService := service.NewProviderConfigService(repos.providerConfig)
	providerService := service.NewProviderService(providers.llm, providers.asr, providers.tts, providerConfigService)

	sqlReviewer := query.NewSQLReviewer(200, 1000)
	auditService := service.NewAuditService(repos.audit)
	datasourceService.SetAuditRecorder(auditService)
	queryService := service.NewQueryService(repos.datasource, repos.query, datasourceRegistry, sqlReviewer, auditService)
	queryService.SetDSNCodec(dsnCodec)
	queryService.SetSecurityRepository(repos.security)
	if cacheStore != nil && cfg.Redis.QueryLock.Enabled {
		queryService.SetQueryLock(cacheStore, cfg.Redis.QueryLock.Prefix, cfg.Redis.QueryLock.TTL)
	}

	queryAgent := query.NewReactAgent(providers.llm, sqlReviewer, promptRenderer, logger)
	queryAgent.SetLLMProviderResolver(func(ctx context.Context, req query.AgentRequest) (llm.Provider, error) {
		provider, err := providerService.ResolveLLMProvider(ctx, service.ProviderScopeInput{
			TenantID:  req.TenantID,
			ProjectID: req.ProjectID,
		})
		if errors.Is(err, service.ErrProviderNotConfigured) {
			return nil, query.ErrLLMNotConfigured
		}
		return provider, err
	})
	queryAgentService := service.NewQueryAgentService(queryAgent)
	queryAgentService.SetAgentContextBuilder(agentContextBuilder)

	knowledgeService := service.NewKnowledgeService(repos.knowledge)
	ragRetriever := rag.NewRetriever(
		repos.knowledge,
		rag.WithEmbedder(providers.llm),
		rag.WithVectorStore(vectorStore),
		rag.WithTopK(cfg.RAG.Milvus.TopK),
	)
	ragIndexer := rag.NewIndexer(repos.knowledge, repos.rag, providers.llm, vectorStore, cfg.RAG.Milvus.Collection)
	ragService := service.NewRAGService(ragIndexer, ragRetriever)
	knowledgeService.SetIndexRefresher(ragService)

	chatService := service.NewChatService(repos.chat, queryAgentService, queryService, ragRetriever)
	chatService.SetAgentContextBuilder(agentContextBuilder)
	chatService.SetAuditRecorder(auditService)
	voiceService := service.NewVoiceService(providerService, chatService)

	out := services{
		tokenManager:   tokenManager,
		permission:     permissionService,
		datasource:     datasourceService,
		provider:       providerService,
		providerConfig: providerConfigService,
		query:          queryService,
		queryAgent:     queryAgentService,
		rag:            ragService,
		audit:          auditService,
		health:         healthService,
		auth:           authService,
		tenant:         tenantService,
		project:        projectService,
		chat:           chatService,
		voice:          voiceService,
		knowledge:      knowledgeService,
		vectorStore:    vectorStore,
	}
	out.attachLogger(logger)
	return out, nil
}

func (s services) attachLogger(logger *zap.Logger) {
	s.auth.SetLogger(logger)
	s.permission.SetLogger(logger)
	s.tenant.SetLogger(logger)
	s.project.SetLogger(logger)
	s.datasource.SetLogger(logger)
	s.providerConfig.SetLogger(logger)
	s.provider.SetLogger(logger)
	s.audit.SetLogger(logger)
	s.query.SetLogger(logger)
	s.queryAgent.SetLogger(logger)
	s.knowledge.SetLogger(logger)
	s.rag.SetLogger(logger)
	s.chat.SetLogger(logger)
	s.voice.SetLogger(logger)
}

func newVectorStore(ctx context.Context, cfg *config.Config) (rag.VectorStore, error) {
	if !cfg.RAG.Milvus.Enabled {
		return nil, nil
	}
	store, err := rag.NewMilvusStore(ctx, rag.MilvusConfig{
		Address:    cfg.RAG.Milvus.Address,
		Collection: cfg.RAG.Milvus.Collection,
		Dimension:  cfg.RAG.Milvus.Dimension,
		TopK:       cfg.RAG.Milvus.TopK,
		Timeout:    cfg.RAG.Milvus.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("init milvus rag store addr=%s collection=%s: %w", cfg.RAG.Milvus.Address, cfg.RAG.Milvus.Collection, err)
	}
	return store, nil
}

type handlers struct {
	health         *handler.HealthHandler
	auth           *handler.AuthHandler
	permission     *handler.PermissionHandler
	tenant         *handler.TenantHandler
	project        *handler.ProjectHandler
	datasource     *handler.DatasourceHandler
	provider       *handler.ProviderHandler
	providerConfig *handler.ProviderConfigHandler
	chat           *handler.ChatHandler
	voice          *handler.VoiceHandler
	knowledge      *handler.KnowledgeHandler
	rag            *handler.RAGHandler
	query          *handler.QueryHandler
	queryAgent     *handler.QueryAgentHandler
	audit          *handler.AuditHandler
}

func newHandlers(services services) handlers {
	return handlers{
		health:         handler.NewHealthHandler(services.health),
		auth:           handler.NewAuthHandler(services.auth),
		permission:     handler.NewPermissionHandler(services.permission),
		tenant:         handler.NewTenantHandler(services.tenant),
		project:        handler.NewProjectHandler(services.project),
		datasource:     handler.NewDatasourceHandler(services.datasource),
		provider:       handler.NewProviderHandler(services.provider),
		providerConfig: handler.NewProviderConfigHandler(services.providerConfig),
		chat:           handler.NewChatHandler(services.chat),
		voice:          handler.NewVoiceHandler(services.voice),
		knowledge:      handler.NewKnowledgeHandler(services.knowledge),
		rag:            handler.NewRAGHandler(services.rag),
		query:          handler.NewQueryHandler(services.query),
		queryAgent:     handler.NewQueryAgentHandler(services.queryAgent),
		audit:          handler.NewAuditHandler(services.audit, services.query),
	}
}

func newRouterDependencies(cfg *config.Config, logger *zap.Logger, services services, handlers handlers, cacheStore cache.Store) router.Dependencies {
	return router.Dependencies{
		Config:                cfg,
		Logger:                logger,
		TokenManager:          services.tokenManager,
		PermissionChecker:     services.permission,
		DatasourceScope:       services.datasource,
		RateLimitStore:        cacheStore,
		RateLimitConfig:       redisRateLimitConfig(cfg),
		HealthHandler:         handlers.health,
		AuthHandler:           handlers.auth,
		PermissionHandler:     handlers.permission,
		TenantHandler:         handlers.tenant,
		ProjectHandler:        handlers.project,
		DatasourceHandler:     handlers.datasource,
		ProviderHandler:       handlers.provider,
		ProviderConfigHandler: handlers.providerConfig,
		ChatHandler:           handlers.chat,
		VoiceHandler:          handlers.voice,
		KnowledgeHandler:      handlers.knowledge,
		RAGHandler:            handlers.rag,
		QueryHandler:          handlers.query,
		QueryAgentHandler:     handlers.queryAgent,
		AuditHandler:          handlers.audit,
	}
}

func newCacheStore(cfg *config.Config, logger *zap.Logger) cache.Store {
	if cfg == nil || !cfg.Redis.Enabled {
		return nil
	}
	if logger != nil {
		logger.Info("redis cache configured",
			zap.String("addr", cfg.Redis.Addr),
			zap.Int("db", cfg.Redis.DB),
			zap.Bool("rate_limit_enabled", cfg.Redis.RateLimit.Enabled),
		)
	}
	return cache.NewRedisClient(cache.RedisOptions{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
}

func closeStore(store cache.Store) error {
	if store == nil {
		return nil
	}
	return store.Close()
}

func redisRateLimitConfig(cfg *config.Config) middleware.RateLimitConfig {
	if cfg == nil || !cfg.Redis.Enabled {
		return middleware.RateLimitConfig{}
	}
	return middleware.RateLimitConfig{
		Enabled:  cfg.Redis.RateLimit.Enabled,
		Requests: cfg.Redis.RateLimit.Requests,
		Window:   cfg.Redis.RateLimit.Window,
		Prefix:   cfg.Redis.RateLimit.Prefix,
	}
}

func buildASRProvider(cfg *config.Config) asr.Provider {
	if cfg == nil || !cfg.Providers.ASR.Enabled {
		return nil
	}
	return asr.NewAliyunProvider(asr.AliyunConfig{
		Token:                          cfg.Providers.ASR.Token,
		AccessKeyID:                    cfg.Providers.ASR.AccessKeyID,
		AccessKeySecret:                cfg.Providers.ASR.AccessKeySecret,
		TokenEndpoint:                  cfg.Providers.ASR.TokenEndpoint,
		TokenRegionID:                  cfg.Providers.ASR.TokenRegionID,
		TokenRefreshBefore:             cfg.Providers.ASR.TokenRefreshBefore,
		AppKey:                         cfg.Providers.ASR.AppKey,
		WebsocketURL:                   cfg.Providers.ASR.WebsocketURL,
		Model:                          cfg.Providers.ASR.Model,
		Format:                         cfg.Providers.ASR.Format,
		SampleRate:                     cfg.Providers.ASR.SampleRate,
		EnableIntermediateResult:       cfg.Providers.ASR.EnableIntermediateResult,
		EnablePunctuationPrediction:    cfg.Providers.ASR.EnablePunctuationPrediction,
		EnableInverseTextNormalization: cfg.Providers.ASR.EnableInverseTextNormalization,
		EnableWords:                    cfg.Providers.ASR.EnableWords,
		Timeout:                        cfg.Providers.ASR.Timeout,
	})
}

func buildTTSProvider(cfg *config.Config) tts.Provider {
	if cfg == nil || !cfg.Providers.TTS.Enabled {
		return nil
	}
	return tts.NewAliyunProvider(tts.AliyunConfig{
		Token:              cfg.Providers.TTS.Token,
		AccessKeyID:        cfg.Providers.TTS.AccessKeyID,
		AccessKeySecret:    cfg.Providers.TTS.AccessKeySecret,
		TokenEndpoint:      cfg.Providers.TTS.TokenEndpoint,
		TokenRegionID:      cfg.Providers.TTS.TokenRegionID,
		TokenRefreshBefore: cfg.Providers.TTS.TokenRefreshBefore,
		AppKey:             cfg.Providers.TTS.AppKey,
		WebsocketURL:       cfg.Providers.TTS.WebsocketURL,
		Model:              cfg.Providers.TTS.Model,
		Voice:              cfg.Providers.TTS.Voice,
		Format:             cfg.Providers.TTS.Format,
		SampleRate:         cfg.Providers.TTS.SampleRate,
		Volume:             cfg.Providers.TTS.Volume,
		SpeechRate:         cfg.Providers.TTS.SpeechRate,
		PitchRate:          cfg.Providers.TTS.PitchRate,
		EnableSubtitle:     cfg.Providers.TTS.EnableSubtitle,
		Timeout:            cfg.Providers.TTS.Timeout,
	})
}
