package repository

import (
	"context"
	"errors"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type ProviderConfigRepository interface {
	GetDefaultLLM(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectLLMConfig, error)
	UpsertDefaultLLM(ctx context.Context, config *model.ProjectLLMConfig) error
	GetDefaultASR(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectASRConfig, error)
	UpsertDefaultASR(ctx context.Context, config *model.ProjectASRConfig) error
	GetDefaultTTS(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectTTSConfig, error)
	UpsertDefaultTTS(ctx context.Context, config *model.ProjectTTSConfig) error
}

type GormProviderConfigRepository struct {
	db *gorm.DB
}

func NewProviderConfigRepository(db *gorm.DB) ProviderConfigRepository {
	return &GormProviderConfigRepository{db: db}
}

func (r *GormProviderConfigRepository) GetDefaultLLM(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectLLMConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var config model.ProjectLLMConfig
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND project_id = ? AND is_default = ?", tenantID, projectID, true).
		Limit(1).
		Find(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &config, nil
}

func (r *GormProviderConfigRepository) UpsertDefaultLLM(ctx context.Context, config *model.ProjectLLMConfig) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.ProjectLLMConfig
		err := tx.First(&existing, "tenant_id = ? AND project_id = ? AND is_default = ?", config.TenantID, config.ProjectID, true).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.IsDefault = true
			return tx.Create(config).Error
		}
		if err != nil {
			return err
		}
		config.ID = existing.ID
		return tx.Model(&model.ProjectLLMConfig{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"provider":           config.Provider,
			"model":              config.Model,
			"api_base":           config.APIBase,
			"api_key_ciphertext": config.APIKeyCiphertext,
			"config_json":        config.ConfigJSON,
			"enabled":            config.Enabled,
			"is_default":         true,
		}).Error
	})
}

func (r *GormProviderConfigRepository) GetDefaultASR(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectASRConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var config model.ProjectASRConfig
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND project_id = ? AND is_default = ?", tenantID, projectID, true).
		Limit(1).
		Find(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &config, nil
}

func (r *GormProviderConfigRepository) UpsertDefaultASR(ctx context.Context, config *model.ProjectASRConfig) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.ProjectASRConfig
		err := tx.First(&existing, "tenant_id = ? AND project_id = ? AND is_default = ?", config.TenantID, config.ProjectID, true).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.IsDefault = true
			return tx.Create(config).Error
		}
		if err != nil {
			return err
		}
		config.ID = existing.ID
		return tx.Model(&model.ProjectASRConfig{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"provider":                          config.Provider,
			"model":                             config.Model,
			"access_key_id_ciphertext":          config.AccessKeyIDCiphertext,
			"access_key_secret_ciphertext":      config.AccessKeySecretCiphertext,
			"app_key":                           config.AppKey,
			"token_endpoint":                    config.TokenEndpoint,
			"token_region_id":                   config.TokenRegionID,
			"token_refresh_before_seconds":      config.TokenRefreshBeforeSeconds,
			"websocket_url":                     config.WebsocketURL,
			"audio_format":                      config.AudioFormat,
			"sample_rate":                       config.SampleRate,
			"enable_intermediate_result":        config.EnableIntermediateResult,
			"enable_punctuation_prediction":     config.EnablePunctuationPrediction,
			"enable_inverse_text_normalization": config.EnableInverseTextNormalization,
			"enable_words":                      config.EnableWords,
			"timeout_ms":                        config.TimeoutMS,
			"config_json":                       config.ConfigJSON,
			"enabled":                           config.Enabled,
			"is_default":                        true,
		}).Error
	})
}

func (r *GormProviderConfigRepository) GetDefaultTTS(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectTTSConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var config model.ProjectTTSConfig
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND project_id = ? AND is_default = ?", tenantID, projectID, true).
		Limit(1).
		Find(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &config, nil
}

func (r *GormProviderConfigRepository) UpsertDefaultTTS(ctx context.Context, config *model.ProjectTTSConfig) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.ProjectTTSConfig
		err := tx.First(&existing, "tenant_id = ? AND project_id = ? AND is_default = ?", config.TenantID, config.ProjectID, true).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.IsDefault = true
			return tx.Create(config).Error
		}
		if err != nil {
			return err
		}
		config.ID = existing.ID
		return tx.Model(&model.ProjectTTSConfig{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"provider":                     config.Provider,
			"model":                        config.Model,
			"voice":                        config.Voice,
			"access_key_id_ciphertext":     config.AccessKeyIDCiphertext,
			"access_key_secret_ciphertext": config.AccessKeySecretCiphertext,
			"app_key":                      config.AppKey,
			"token_endpoint":               config.TokenEndpoint,
			"token_region_id":              config.TokenRegionID,
			"token_refresh_before_seconds": config.TokenRefreshBeforeSeconds,
			"websocket_url":                config.WebsocketURL,
			"audio_format":                 config.AudioFormat,
			"sample_rate":                  config.SampleRate,
			"volume":                       config.Volume,
			"speech_rate":                  config.SpeechRate,
			"pitch_rate":                   config.PitchRate,
			"enable_subtitle":              config.EnableSubtitle,
			"timeout_ms":                   config.TimeoutMS,
			"config_json":                  config.ConfigJSON,
			"enabled":                      config.Enabled,
			"is_default":                   true,
		}).Error
	})
}
