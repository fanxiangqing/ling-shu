package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig      `yaml:"app"`
	Server    ServerConfig   `yaml:"server"`
	Log       LogConfig      `yaml:"log"`
	Auth      AuthConfig     `yaml:"auth"`
	Security  SecurityConfig `yaml:"security"`
	Prompts   PromptConfig   `yaml:"prompts"`
	Database  DatabaseConfig `yaml:"database"`
	Redis     RedisConfig    `yaml:"redis"`
	RAG       RAGConfig      `yaml:"rag"`
	Providers ProviderConfig `yaml:"providers"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type ServerConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	Mode              string        `yaml:"mode"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
}

type LogConfig struct {
	Level          string `yaml:"level"`
	Encoding       string `yaml:"encoding"`
	ConsoleEnabled bool   `yaml:"console_enabled"`
	FileEnabled    bool   `yaml:"file_enabled"`
	FileDir        string `yaml:"file_dir"`
	FileName       string `yaml:"file_name"`
}

type AuthConfig struct {
	JWTSecret      string        `yaml:"jwt_secret"`
	AccessTokenTTL time.Duration `yaml:"access_token_ttl"`
}

type SecurityConfig struct {
	DSNSecret string `yaml:"dsn_secret"`
}

type PromptConfig struct {
	Dir string `yaml:"dir"`
}

type DatabaseConfig struct {
	Enabled         bool          `yaml:"enabled"`
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Enabled       bool                     `yaml:"enabled"`
	Addr          string                   `yaml:"addr"`
	Password      string                   `yaml:"password"`
	DB            int                      `yaml:"db"`
	DialTimeout   time.Duration            `yaml:"dial_timeout"`
	ReadTimeout   time.Duration            `yaml:"read_timeout"`
	WriteTimeout  time.Duration            `yaml:"write_timeout"`
	RateLimit     RedisRateLimitConfig     `yaml:"rate_limit"`
	QueryLock     RedisQueryLockConfig     `yaml:"query_lock"`
	MetadataCache RedisMetadataCacheConfig `yaml:"metadata_cache"`
}

type RedisRateLimitConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
	Prefix   string        `yaml:"prefix"`
}

type RedisQueryLockConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
	Prefix  string        `yaml:"prefix"`
}

type RedisMetadataCacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
	Prefix  string        `yaml:"prefix"`
}

type RAGConfig struct {
	Milvus MilvusConfig `yaml:"milvus"`
}

type MilvusConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Address    string        `yaml:"address"`
	Collection string        `yaml:"collection"`
	Dimension  int           `yaml:"dimension"`
	TopK       int           `yaml:"top_k"`
	Timeout    time.Duration `yaml:"timeout"`
}

type ProviderConfig struct {
	LLM LLMProviderConfig `yaml:"llm"`
	ASR ASRProviderConfig `yaml:"asr"`
	TTS TTSProviderConfig `yaml:"tts"`
}

type LLMProviderConfig struct {
	Provider       string        `yaml:"provider"`
	APIKey         string        `yaml:"api_key"`
	BaseURL        string        `yaml:"base_url"`
	ChatModel      string        `yaml:"chat_model"`
	EmbeddingModel string        `yaml:"embedding_model"`
	Timeout        time.Duration `yaml:"timeout"`
}

type ASRProviderConfig struct {
	Enabled                        bool          `yaml:"enabled"`
	Provider                       string        `yaml:"provider"`
	Token                          string        `yaml:"token"`
	AccessKeyID                    string        `yaml:"access_key_id"`
	AccessKeySecret                string        `yaml:"access_key_secret"`
	TokenEndpoint                  string        `yaml:"token_endpoint"`
	TokenRegionID                  string        `yaml:"token_region_id"`
	TokenRefreshBefore             time.Duration `yaml:"token_refresh_before"`
	AppKey                         string        `yaml:"app_key"`
	WebsocketURL                   string        `yaml:"websocket_url"`
	Model                          string        `yaml:"model"`
	Format                         string        `yaml:"format"`
	SampleRate                     int           `yaml:"sample_rate"`
	EnableIntermediateResult       bool          `yaml:"enable_intermediate_result"`
	EnablePunctuationPrediction    bool          `yaml:"enable_punctuation_prediction"`
	EnableInverseTextNormalization bool          `yaml:"enable_inverse_text_normalization"`
	EnableWords                    bool          `yaml:"enable_words"`
	Timeout                        time.Duration `yaml:"timeout"`
}

type TTSProviderConfig struct {
	Enabled            bool          `yaml:"enabled"`
	Provider           string        `yaml:"provider"`
	Token              string        `yaml:"token"`
	AccessKeyID        string        `yaml:"access_key_id"`
	AccessKeySecret    string        `yaml:"access_key_secret"`
	TokenEndpoint      string        `yaml:"token_endpoint"`
	TokenRegionID      string        `yaml:"token_region_id"`
	TokenRefreshBefore time.Duration `yaml:"token_refresh_before"`
	AppKey             string        `yaml:"app_key"`
	WebsocketURL       string        `yaml:"websocket_url"`
	Model              string        `yaml:"model"`
	Voice              string        `yaml:"voice"`
	Format             string        `yaml:"format"`
	SampleRate         int           `yaml:"sample_rate"`
	Volume             int           `yaml:"volume"`
	SpeechRate         int           `yaml:"speech_rate"`
	PitchRate          int           `yaml:"pitch_rate"`
	EnableSubtitle     bool          `yaml:"enable_subtitle"`
	Timeout            time.Duration `yaml:"timeout"`
}

func Default() Config {
	return Config{
		App: AppConfig{
			Name: "ling-shu",
			Env:  "local",
		},
		Server: ServerConfig{
			Host:              "0.0.0.0",
			Port:              8080,
			Mode:              "debug",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      0,
			IdleTimeout:       60 * time.Second,
		},
		Log: LogConfig{
			Level:          "info",
			Encoding:       "json",
			ConsoleEnabled: true,
			FileEnabled:    true,
			FileDir:        "logs",
			FileName:       "ling-shu",
		},
		Auth: AuthConfig{
			JWTSecret:      "ling-shu-local-dev-secret",
			AccessTokenTTL: 24 * time.Hour,
		},
		Security: SecurityConfig{
			DSNSecret: "ling-shu-local-dev-dsn-secret",
		},
		Prompts: PromptConfig{
			Dir: "prompts",
		},
		Database: DatabaseConfig{
			Enabled:         false,
			MaxOpenConns:    30,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
		},
		Redis: RedisConfig{
			Enabled:      false,
			Addr:         "127.0.0.1:6379",
			DB:           0,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
			RateLimit: RedisRateLimitConfig{
				Enabled:  false,
				Requests: 120,
				Window:   time.Minute,
				Prefix:   "ling-shu:rate",
			},
			QueryLock: RedisQueryLockConfig{
				Enabled: false,
				TTL:     30 * time.Second,
				Prefix:  "ling-shu:query:lock",
			},
			MetadataCache: RedisMetadataCacheConfig{
				Enabled: false,
				TTL:     10 * time.Minute,
				Prefix:  "ling-shu:project:meta",
			},
		},
		RAG: RAGConfig{
			Milvus: MilvusConfig{
				Enabled:    false,
				Address:    "127.0.0.1:19530",
				Collection: "ling_shu_kb_chunks",
				Dimension:  1024,
				TopK:       8,
				Timeout:    10 * time.Second,
			},
		},
		Providers: ProviderConfig{
			LLM: LLMProviderConfig{
				Provider:       "aliyun",
				BaseURL:        "https://dashscope.aliyuncs.com/compatible-mode/v1",
				ChatModel:      "qwen-plus",
				EmbeddingModel: "text-embedding-v4",
				Timeout:        180 * time.Second,
			},
			ASR: ASRProviderConfig{
				Enabled:                        false,
				Provider:                       "aliyun",
				TokenEndpoint:                  "https://nls-meta.cn-shanghai.aliyuncs.com/",
				TokenRegionID:                  "cn-shanghai",
				TokenRefreshBefore:             10 * time.Minute,
				WebsocketURL:                   "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1",
				Model:                          "nls-realtime-asr",
				Format:                         "pcm",
				SampleRate:                     16000,
				EnableIntermediateResult:       true,
				EnablePunctuationPrediction:    true,
				EnableInverseTextNormalization: true,
				EnableWords:                    false,
				Timeout:                        120 * time.Second,
			},
			TTS: TTSProviderConfig{
				Enabled:            false,
				Provider:           "aliyun",
				TokenEndpoint:      "https://nls-meta.cn-shanghai.aliyuncs.com/",
				TokenRegionID:      "cn-shanghai",
				TokenRefreshBefore: 10 * time.Minute,
				WebsocketURL:       "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1",
				Model:              "nls-tts",
				Voice:              "aixia",
				Format:             "mp3",
				SampleRate:         16000,
				Volume:             50,
				SpeechRate:         0,
				PitchRate:          0,
				Timeout:            60 * time.Second,
			},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		} else if err := yaml.Unmarshal(content, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c Config) Validate() error {
	if c.App.Name == "" {
		return errors.New("app.name is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535: %d", c.Server.Port)
	}
	if c.Log.Level == "" {
		return errors.New("log.level is required")
	}
	if c.Log.Encoding == "" {
		return errors.New("log.encoding is required")
	}
	if c.Log.FileEnabled {
		if c.Log.FileDir == "" {
			return errors.New("log.file_dir is required when log.file_enabled is true")
		}
		if c.Log.FileName == "" {
			return errors.New("log.file_name is required when log.file_enabled is true")
		}
	}
	if c.Auth.JWTSecret == "" {
		return errors.New("auth.jwt_secret is required")
	}
	if c.Auth.AccessTokenTTL <= 0 {
		return errors.New("auth.access_token_ttl must be positive")
	}
	if c.Prompts.Dir == "" {
		return errors.New("prompts.dir is required")
	}
	if c.Database.Enabled && c.Database.DSN == "" {
		return errors.New("database.dsn is required when database.enabled is true")
	}
	if c.Redis.Enabled {
		if c.Redis.Addr == "" {
			return errors.New("redis.addr is required when redis.enabled is true")
		}
		if c.Redis.DB < 0 {
			return fmt.Errorf("redis.db must be greater than or equal to 0: %d", c.Redis.DB)
		}
		if c.Redis.DialTimeout <= 0 {
			return errors.New("redis.dial_timeout must be positive")
		}
		if c.Redis.ReadTimeout <= 0 {
			return errors.New("redis.read_timeout must be positive")
		}
		if c.Redis.WriteTimeout <= 0 {
			return errors.New("redis.write_timeout must be positive")
		}
		if c.Redis.RateLimit.Enabled {
			if c.Redis.RateLimit.Requests <= 0 {
				return errors.New("redis.rate_limit.requests must be positive")
			}
			if c.Redis.RateLimit.Window <= 0 {
				return errors.New("redis.rate_limit.window must be positive")
			}
			if c.Redis.RateLimit.Prefix == "" {
				return errors.New("redis.rate_limit.prefix is required when redis.rate_limit.enabled is true")
			}
		}
		if c.Redis.QueryLock.Enabled {
			if c.Redis.QueryLock.TTL <= 0 {
				return errors.New("redis.query_lock.ttl must be positive")
			}
			if c.Redis.QueryLock.Prefix == "" {
				return errors.New("redis.query_lock.prefix is required when redis.query_lock.enabled is true")
			}
		}
		if c.Redis.MetadataCache.Enabled {
			if c.Redis.MetadataCache.TTL <= 0 {
				return errors.New("redis.metadata_cache.ttl must be positive")
			}
			if c.Redis.MetadataCache.Prefix == "" {
				return errors.New("redis.metadata_cache.prefix is required when redis.metadata_cache.enabled is true")
			}
		}
	}
	if c.RAG.Milvus.Enabled {
		if c.RAG.Milvus.Address == "" {
			return errors.New("rag.milvus.address is required when rag.milvus.enabled is true")
		}
		if c.RAG.Milvus.Collection == "" {
			return errors.New("rag.milvus.collection is required when rag.milvus.enabled is true")
		}
		if c.RAG.Milvus.Dimension <= 0 {
			return errors.New("rag.milvus.dimension must be positive")
		}
		if c.RAG.Milvus.TopK <= 0 {
			return errors.New("rag.milvus.top_k must be positive")
		}
	}
	if c.Providers.LLM.Provider != "aliyun" {
		return fmt.Errorf("providers.llm.provider only supports aliyun: %s", c.Providers.LLM.Provider)
	}
	if c.Providers.ASR.Enabled {
		if c.Providers.ASR.Provider != "aliyun" {
			return fmt.Errorf("providers.asr.provider only supports aliyun: %s", c.Providers.ASR.Provider)
		}
		if c.Providers.ASR.SampleRate != 8000 && c.Providers.ASR.SampleRate != 16000 {
			return fmt.Errorf("providers.asr.sample_rate only supports 8000 or 16000: %d", c.Providers.ASR.SampleRate)
		}
	}
	if c.Providers.TTS.Enabled {
		if c.Providers.TTS.Provider != "aliyun" {
			return fmt.Errorf("providers.tts.provider only supports aliyun: %s", c.Providers.TTS.Provider)
		}
		if c.Providers.TTS.SampleRate <= 0 {
			return errors.New("providers.tts.sample_rate must be positive")
		}
		if c.Providers.TTS.Volume < 0 || c.Providers.TTS.Volume > 100 {
			return fmt.Errorf("providers.tts.volume must be between 0 and 100: %d", c.Providers.TTS.Volume)
		}
	}
	return nil
}

func (c ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func applyEnv(cfg *Config) {
	setString(&cfg.App.Name, "LING_SHU_APP_NAME")
	setString(&cfg.App.Env, "LING_SHU_APP_ENV")
	setString(&cfg.Server.Host, "LING_SHU_SERVER_HOST")
	setInt(&cfg.Server.Port, "LING_SHU_SERVER_PORT")
	setString(&cfg.Server.Mode, "LING_SHU_SERVER_MODE")
	setString(&cfg.Log.Level, "LING_SHU_LOG_LEVEL")
	setString(&cfg.Log.Encoding, "LING_SHU_LOG_ENCODING")
	setBool(&cfg.Log.ConsoleEnabled, "LING_SHU_LOG_CONSOLE_ENABLED")
	setBool(&cfg.Log.FileEnabled, "LING_SHU_LOG_FILE_ENABLED")
	setString(&cfg.Log.FileDir, "LING_SHU_LOG_FILE_DIR")
	setString(&cfg.Log.FileName, "LING_SHU_LOG_FILE_NAME")
	setString(&cfg.Auth.JWTSecret, "LING_SHU_JWT_SECRET")
	setDuration(&cfg.Auth.AccessTokenTTL, "LING_SHU_ACCESS_TOKEN_TTL")
	setString(&cfg.Security.DSNSecret, "LING_SHU_DSN_SECRET")
	setString(&cfg.Prompts.Dir, "LING_SHU_PROMPTS_DIR")
	setBool(&cfg.Database.Enabled, "LING_SHU_DATABASE_ENABLED")
	setString(&cfg.Database.DSN, "LING_SHU_MYSQL_DSN")
	setInt(&cfg.Database.MaxOpenConns, "LING_SHU_DATABASE_MAX_OPEN_CONNS")
	setInt(&cfg.Database.MaxIdleConns, "LING_SHU_DATABASE_MAX_IDLE_CONNS")
	setBool(&cfg.Redis.Enabled, "LING_SHU_REDIS_ENABLED")
	setString(&cfg.Redis.Addr, "LING_SHU_REDIS_ADDR")
	setString(&cfg.Redis.Password, "LING_SHU_REDIS_PASSWORD")
	setInt(&cfg.Redis.DB, "LING_SHU_REDIS_DB")
	setDuration(&cfg.Redis.DialTimeout, "LING_SHU_REDIS_DIAL_TIMEOUT")
	setDuration(&cfg.Redis.ReadTimeout, "LING_SHU_REDIS_READ_TIMEOUT")
	setDuration(&cfg.Redis.WriteTimeout, "LING_SHU_REDIS_WRITE_TIMEOUT")
	setBool(&cfg.Redis.RateLimit.Enabled, "LING_SHU_REDIS_RATE_LIMIT_ENABLED")
	setInt(&cfg.Redis.RateLimit.Requests, "LING_SHU_REDIS_RATE_LIMIT_REQUESTS")
	setDuration(&cfg.Redis.RateLimit.Window, "LING_SHU_REDIS_RATE_LIMIT_WINDOW")
	setString(&cfg.Redis.RateLimit.Prefix, "LING_SHU_REDIS_RATE_LIMIT_PREFIX")
	setBool(&cfg.Redis.QueryLock.Enabled, "LING_SHU_REDIS_QUERY_LOCK_ENABLED")
	setDuration(&cfg.Redis.QueryLock.TTL, "LING_SHU_REDIS_QUERY_LOCK_TTL")
	setString(&cfg.Redis.QueryLock.Prefix, "LING_SHU_REDIS_QUERY_LOCK_PREFIX")
	setBool(&cfg.Redis.MetadataCache.Enabled, "LING_SHU_REDIS_METADATA_CACHE_ENABLED")
	setDuration(&cfg.Redis.MetadataCache.TTL, "LING_SHU_REDIS_METADATA_CACHE_TTL")
	setString(&cfg.Redis.MetadataCache.Prefix, "LING_SHU_REDIS_METADATA_CACHE_PREFIX")
	setBool(&cfg.RAG.Milvus.Enabled, "LING_SHU_MILVUS_ENABLED")
	setString(&cfg.RAG.Milvus.Address, "LING_SHU_MILVUS_ADDR")
	setString(&cfg.RAG.Milvus.Collection, "LING_SHU_MILVUS_COLLECTION")
	setInt(&cfg.RAG.Milvus.Dimension, "LING_SHU_MILVUS_DIMENSION")
	setInt(&cfg.RAG.Milvus.TopK, "LING_SHU_RAG_TOP_K")

	aliyunAPIKey := firstEnv("LING_SHU_ALIYUN_API_KEY", "DASHSCOPE_API_KEY")
	if aliyunAPIKey != "" {
		cfg.Providers.LLM.APIKey = aliyunAPIKey
	}
	aliyunNLSToken := firstEnv("LING_SHU_ALIYUN_NLS_TOKEN", "LING_SHU_NLS_TOKEN", "ALIYUN_NLS_TOKEN", "NLS_TOKEN")
	if aliyunNLSToken != "" {
		cfg.Providers.ASR.Token = aliyunNLSToken
		cfg.Providers.TTS.Token = aliyunNLSToken
	}
	aliyunNLSAppKey := firstEnv("LING_SHU_ALIYUN_NLS_APP_KEY", "LING_SHU_NLS_APP_KEY", "ALIYUN_NLS_APP_KEY", "NLS_APP_KEY")
	if aliyunNLSAppKey != "" {
		cfg.Providers.ASR.AppKey = aliyunNLSAppKey
		cfg.Providers.TTS.AppKey = aliyunNLSAppKey
	}
	aliyunAccessKeyID := firstEnv("LING_SHU_ALIYUN_ACCESS_KEY_ID", "LING_SHU_ALIYUN_AK_ID", "ALIYUN_AK_ID", "ALIBABA_CLOUD_ACCESS_KEY_ID")
	if aliyunAccessKeyID != "" {
		cfg.Providers.ASR.AccessKeyID = aliyunAccessKeyID
		cfg.Providers.TTS.AccessKeyID = aliyunAccessKeyID
	}
	aliyunAccessKeySecret := firstEnv("LING_SHU_ALIYUN_ACCESS_KEY_SECRET", "LING_SHU_ALIYUN_AK_SECRET", "ALIYUN_AK_SECRET", "ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	if aliyunAccessKeySecret != "" {
		cfg.Providers.ASR.AccessKeySecret = aliyunAccessKeySecret
		cfg.Providers.TTS.AccessKeySecret = aliyunAccessKeySecret
	}

	setString(&cfg.Providers.LLM.Provider, "LING_SHU_LLM_PROVIDER")
	setString(&cfg.Providers.LLM.APIKey, "LING_SHU_LLM_API_KEY")
	setString(&cfg.Providers.LLM.BaseURL, "LING_SHU_LLM_BASE_URL")
	setString(&cfg.Providers.LLM.ChatModel, "LING_SHU_LLM_CHAT_MODEL")
	setString(&cfg.Providers.LLM.EmbeddingModel, "LING_SHU_LLM_EMBEDDING_MODEL")

	setBool(&cfg.Providers.ASR.Enabled, "LING_SHU_ASR_ENABLED")
	setString(&cfg.Providers.ASR.Provider, "LING_SHU_ASR_PROVIDER")
	setString(&cfg.Providers.ASR.Token, "LING_SHU_ASR_TOKEN")
	setString(&cfg.Providers.ASR.AccessKeyID, "LING_SHU_ASR_ACCESS_KEY_ID")
	setString(&cfg.Providers.ASR.AccessKeySecret, "LING_SHU_ASR_ACCESS_KEY_SECRET")
	setString(&cfg.Providers.ASR.TokenEndpoint, "LING_SHU_ASR_TOKEN_ENDPOINT")
	setString(&cfg.Providers.ASR.TokenRegionID, "LING_SHU_ASR_TOKEN_REGION_ID")
	setString(&cfg.Providers.ASR.AppKey, "LING_SHU_ASR_APP_KEY")
	setString(&cfg.Providers.ASR.WebsocketURL, "LING_SHU_ASR_WEBSOCKET_URL")
	setString(&cfg.Providers.ASR.Model, "LING_SHU_ASR_MODEL")
	setString(&cfg.Providers.ASR.Format, "LING_SHU_ASR_FORMAT")
	setInt(&cfg.Providers.ASR.SampleRate, "LING_SHU_ASR_SAMPLE_RATE")
	setBool(&cfg.Providers.ASR.EnableIntermediateResult, "LING_SHU_ASR_ENABLE_INTERMEDIATE_RESULT")
	setBool(&cfg.Providers.ASR.EnablePunctuationPrediction, "LING_SHU_ASR_ENABLE_PUNCTUATION_PREDICTION")
	setBool(&cfg.Providers.ASR.EnableInverseTextNormalization, "LING_SHU_ASR_ENABLE_INVERSE_TEXT_NORMALIZATION")
	setBool(&cfg.Providers.ASR.EnableWords, "LING_SHU_ASR_ENABLE_WORDS")

	setBool(&cfg.Providers.TTS.Enabled, "LING_SHU_TTS_ENABLED")
	setString(&cfg.Providers.TTS.Provider, "LING_SHU_TTS_PROVIDER")
	setString(&cfg.Providers.TTS.Token, "LING_SHU_TTS_TOKEN")
	setString(&cfg.Providers.TTS.AccessKeyID, "LING_SHU_TTS_ACCESS_KEY_ID")
	setString(&cfg.Providers.TTS.AccessKeySecret, "LING_SHU_TTS_ACCESS_KEY_SECRET")
	setString(&cfg.Providers.TTS.TokenEndpoint, "LING_SHU_TTS_TOKEN_ENDPOINT")
	setString(&cfg.Providers.TTS.TokenRegionID, "LING_SHU_TTS_TOKEN_REGION_ID")
	setString(&cfg.Providers.TTS.AppKey, "LING_SHU_TTS_APP_KEY")
	setString(&cfg.Providers.TTS.WebsocketURL, "LING_SHU_TTS_WEBSOCKET_URL")
	setString(&cfg.Providers.TTS.Model, "LING_SHU_TTS_MODEL")
	setString(&cfg.Providers.TTS.Voice, "LING_SHU_TTS_VOICE")
	setString(&cfg.Providers.TTS.Format, "LING_SHU_TTS_FORMAT")
	setInt(&cfg.Providers.TTS.SampleRate, "LING_SHU_TTS_SAMPLE_RATE")
	setInt(&cfg.Providers.TTS.Volume, "LING_SHU_TTS_VOLUME")
	setInt(&cfg.Providers.TTS.SpeechRate, "LING_SHU_TTS_SPEECH_RATE")
	setInt(&cfg.Providers.TTS.PitchRate, "LING_SHU_TTS_PITCH_RATE")
	setBool(&cfg.Providers.TTS.EnableSubtitle, "LING_SHU_TTS_ENABLE_SUBTITLE")
}

func setString(target *string, key string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}

func setInt(target *int, key string) {
	value := os.Getenv(key)
	if value == "" {
		return
	}
	parsed, err := strconv.Atoi(value)
	if err == nil {
		*target = parsed
	}
}

func setBool(target *bool, key string) {
	value := os.Getenv(key)
	if value == "" {
		return
	}
	parsed, err := strconv.ParseBool(value)
	if err == nil {
		*target = parsed
	}
}

func setDuration(target *time.Duration, key string) {
	value := os.Getenv(key)
	if value == "" {
		return
	}
	parsed, err := time.ParseDuration(value)
	if err == nil {
		*target = parsed
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
