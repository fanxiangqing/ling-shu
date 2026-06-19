package model

import "time"

type ProjectLLMConfig struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID         uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID        uint64    `gorm:"column:project_id;not null" json:"project_id"`
	Provider         string    `gorm:"column:provider;size:64;not null" json:"provider"`
	Model            string    `gorm:"column:model;size:128;not null" json:"model"`
	APIBase          string    `gorm:"column:api_base;size:512" json:"api_base,omitempty"`
	APIKeyCiphertext string    `gorm:"column:api_key_ciphertext;type:text" json:"-"`
	ConfigJSON       *string   `gorm:"column:config_json;type:json" json:"config_json,omitempty"`
	Enabled          bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	IsDefault        bool      `gorm:"column:is_default;not null;default:false" json:"is_default"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ProjectLLMConfig) TableName() string {
	return "project_llm_configs"
}

type ProjectASRConfig struct {
	ID                             uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID                       uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID                      uint64    `gorm:"column:project_id;not null" json:"project_id"`
	Provider                       string    `gorm:"column:provider;size:64;not null" json:"provider"`
	Model                          string    `gorm:"column:model;size:128;not null" json:"model"`
	AccessKeyIDCiphertext          string    `gorm:"column:access_key_id_ciphertext;type:text" json:"-"`
	AccessKeySecretCiphertext      string    `gorm:"column:access_key_secret_ciphertext;type:text" json:"-"`
	AppKey                         string    `gorm:"column:app_key;size:128" json:"app_key,omitempty"`
	TokenEndpoint                  string    `gorm:"column:token_endpoint;size:512" json:"token_endpoint,omitempty"`
	TokenRegionID                  string    `gorm:"column:token_region_id;size:64" json:"token_region_id,omitempty"`
	TokenRefreshBeforeSeconds      int       `gorm:"column:token_refresh_before_seconds;not null;default:600" json:"token_refresh_before_seconds"`
	WebsocketURL                   string    `gorm:"column:websocket_url;size:512" json:"websocket_url,omitempty"`
	AudioFormat                    string    `gorm:"column:audio_format;size:32;not null;default:pcm" json:"format"`
	SampleRate                     int       `gorm:"column:sample_rate;not null;default:16000" json:"sample_rate"`
	EnableIntermediateResult       bool      `gorm:"column:enable_intermediate_result;not null;default:true" json:"enable_intermediate_result"`
	EnablePunctuationPrediction    bool      `gorm:"column:enable_punctuation_prediction;not null;default:true" json:"enable_punctuation_prediction"`
	EnableInverseTextNormalization bool      `gorm:"column:enable_inverse_text_normalization;not null;default:true" json:"enable_inverse_text_normalization"`
	EnableWords                    bool      `gorm:"column:enable_words;not null;default:false" json:"enable_words"`
	TimeoutMS                      int       `gorm:"column:timeout_ms;not null;default:120000" json:"timeout_ms"`
	ConfigJSON                     *string   `gorm:"column:config_json;type:json" json:"config_json,omitempty"`
	Enabled                        bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	IsDefault                      bool      `gorm:"column:is_default;not null;default:false" json:"is_default"`
	CreatedAt                      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ProjectASRConfig) TableName() string {
	return "project_asr_configs"
}

type ProjectTTSConfig struct {
	ID                        uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID                  uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID                 uint64    `gorm:"column:project_id;not null" json:"project_id"`
	Provider                  string    `gorm:"column:provider;size:64;not null" json:"provider"`
	Model                     string    `gorm:"column:model;size:128;not null" json:"model"`
	Voice                     string    `gorm:"column:voice;size:128" json:"voice,omitempty"`
	AccessKeyIDCiphertext     string    `gorm:"column:access_key_id_ciphertext;type:text" json:"-"`
	AccessKeySecretCiphertext string    `gorm:"column:access_key_secret_ciphertext;type:text" json:"-"`
	AppKey                    string    `gorm:"column:app_key;size:128" json:"app_key,omitempty"`
	TokenEndpoint             string    `gorm:"column:token_endpoint;size:512" json:"token_endpoint,omitempty"`
	TokenRegionID             string    `gorm:"column:token_region_id;size:64" json:"token_region_id,omitempty"`
	TokenRefreshBeforeSeconds int       `gorm:"column:token_refresh_before_seconds;not null;default:600" json:"token_refresh_before_seconds"`
	WebsocketURL              string    `gorm:"column:websocket_url;size:512" json:"websocket_url,omitempty"`
	AudioFormat               string    `gorm:"column:audio_format;size:32;not null;default:mp3" json:"format"`
	SampleRate                int       `gorm:"column:sample_rate;not null;default:16000" json:"sample_rate"`
	Volume                    int       `gorm:"column:volume;not null;default:50" json:"volume"`
	SpeechRate                int       `gorm:"column:speech_rate;not null;default:0" json:"speech_rate"`
	PitchRate                 int       `gorm:"column:pitch_rate;not null;default:0" json:"pitch_rate"`
	EnableSubtitle            bool      `gorm:"column:enable_subtitle;not null;default:false" json:"enable_subtitle"`
	TimeoutMS                 int       `gorm:"column:timeout_ms;not null;default:60000" json:"timeout_ms"`
	ConfigJSON                *string   `gorm:"column:config_json;type:json" json:"config_json,omitempty"`
	Enabled                   bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	IsDefault                 bool      `gorm:"column:is_default;not null;default:false" json:"is_default"`
	CreatedAt                 time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                 time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ProjectTTSConfig) TableName() string {
	return "project_tts_configs"
}
