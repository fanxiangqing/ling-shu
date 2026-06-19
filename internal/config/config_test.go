package config

import (
	"os"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
	if cfg.Providers.ASR.Enabled || cfg.Providers.TTS.Enabled {
		t.Fatal("expected asr and tts providers to be disabled by default")
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("LING_SHU_SERVER_PORT", "18080")
	t.Setenv("LING_SHU_APP_ENV", "test")
	t.Setenv("LING_SHU_JWT_SECRET", "test-secret")
	t.Setenv("LING_SHU_DSN_SECRET", "test-dsn-secret")
	t.Setenv("LING_SHU_LOG_FILE_ENABLED", "true")
	t.Setenv("LING_SHU_LOG_FILE_DIR", "/tmp/ling-shu-test-logs")
	t.Setenv("LING_SHU_LOG_FILE_NAME", "api")
	t.Setenv("LING_SHU_ACCESS_TOKEN_TTL", "2h")
	t.Setenv("LING_SHU_REDIS_ENABLED", "true")
	t.Setenv("LING_SHU_REDIS_ADDR", "redis:6379")
	t.Setenv("LING_SHU_REDIS_DB", "2")
	t.Setenv("LING_SHU_REDIS_RATE_LIMIT_ENABLED", "true")
	t.Setenv("LING_SHU_REDIS_RATE_LIMIT_REQUESTS", "88")
	t.Setenv("LING_SHU_REDIS_RATE_LIMIT_WINDOW", "30s")
	t.Setenv("LING_SHU_REDIS_QUERY_LOCK_ENABLED", "true")
	t.Setenv("LING_SHU_REDIS_QUERY_LOCK_TTL", "45s")
	t.Setenv("LING_SHU_REDIS_METADATA_CACHE_ENABLED", "true")
	t.Setenv("LING_SHU_REDIS_METADATA_CACHE_TTL", "3m")
	t.Setenv("DASHSCOPE_API_KEY", "dashscope-key")
	t.Setenv("LING_SHU_MILVUS_ENABLED", "true")
	t.Setenv("LING_SHU_MILVUS_ADDR", "127.0.0.1:19530")
	t.Setenv("ALIYUN_AK_ID", "ak-id")
	t.Setenv("ALIYUN_AK_SECRET", "ak-secret")
	t.Setenv("LING_SHU_ALIYUN_NLS_APP_KEY", "nls-app-key")
	t.Setenv("LING_SHU_ASR_ENABLED", "true")
	t.Setenv("LING_SHU_TTS_ENABLED", "true")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Port != 18080 {
		t.Fatalf("expected server port 18080, got %d", cfg.Server.Port)
	}
	if cfg.App.Env != "test" {
		t.Fatalf("expected app env test, got %q", cfg.App.Env)
	}
	if cfg.Providers.LLM.APIKey != "dashscope-key" {
		t.Fatalf("expected llm api key from env")
	}
	if cfg.Auth.JWTSecret != "test-secret" || cfg.Auth.AccessTokenTTL.String() != "2h0m0s" {
		t.Fatalf("expected auth config from env")
	}
	if cfg.Security.DSNSecret != "test-dsn-secret" {
		t.Fatalf("expected dsn secret from env")
	}
	if !cfg.Log.FileEnabled || cfg.Log.FileDir != "/tmp/ling-shu-test-logs" || cfg.Log.FileName != "api" {
		t.Fatalf("expected log file config from env, got %+v", cfg.Log)
	}
	if !cfg.Redis.Enabled || cfg.Redis.Addr != "redis:6379" || cfg.Redis.DB != 2 {
		t.Fatalf("expected redis config from env, got %+v", cfg.Redis)
	}
	if !cfg.Redis.RateLimit.Enabled || cfg.Redis.RateLimit.Requests != 88 || cfg.Redis.RateLimit.Window.String() != "30s" {
		t.Fatalf("expected redis rate limit config from env, got %+v", cfg.Redis.RateLimit)
	}
	if !cfg.Redis.QueryLock.Enabled || cfg.Redis.QueryLock.TTL.String() != "45s" {
		t.Fatalf("expected redis query lock config from env, got %+v", cfg.Redis.QueryLock)
	}
	if !cfg.Redis.MetadataCache.Enabled || cfg.Redis.MetadataCache.TTL.String() != "3m0s" {
		t.Fatalf("expected redis metadata cache config from env, got %+v", cfg.Redis.MetadataCache)
	}
	if !cfg.RAG.Milvus.Enabled || cfg.RAG.Milvus.Address != "127.0.0.1:19530" {
		t.Fatalf("expected milvus config from env")
	}
	if cfg.Providers.ASR.AccessKeyID != "ak-id" {
		t.Fatalf("expected asr access key id from env")
	}
	if !cfg.Providers.ASR.Enabled || !cfg.Providers.TTS.Enabled {
		t.Fatalf("expected asr and tts enabled from env")
	}
	if cfg.Providers.TTS.AccessKeyID != "ak-id" {
		t.Fatalf("expected tts access key id from env")
	}
	if cfg.Providers.ASR.AccessKeySecret != "ak-secret" {
		t.Fatalf("expected asr access key secret from env")
	}
	if cfg.Providers.TTS.AccessKeySecret != "ak-secret" {
		t.Fatalf("expected tts access key secret from env")
	}
	if cfg.Providers.ASR.AppKey != "nls-app-key" {
		t.Fatalf("expected asr app key from env")
	}
	if cfg.Providers.TTS.AppKey != "nls-app-key" {
		t.Fatalf("expected tts app key from env")
	}
}

func TestDisabledVoiceProvidersDoNotBlockValidation(t *testing.T) {
	cfg := Default()
	cfg.Providers.ASR.Provider = "disabled-asr"
	cfg.Providers.ASR.SampleRate = 44100
	cfg.Providers.TTS.Provider = "disabled-tts"
	cfg.Providers.TTS.SampleRate = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled voice providers should not block validation: %v", err)
	}
}

func TestLoadParsesYAML(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, _ = file.WriteString("server:\n  port: 19090\n")
	_ = file.Close()

	cfg, err := Load(file.Name())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Port != 19090 {
		t.Fatalf("expected server port 19090, got %d", cfg.Server.Port)
	}
}
