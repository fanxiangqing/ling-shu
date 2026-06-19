-- 仅用于已经使用旧版 001_init_schema.sql 初始化过的数据库。
-- 新库请直接使用当前 001_init_schema.sql，不需要执行本文件。

ALTER TABLE project_asr_configs
  ADD COLUMN access_key_id_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey ID密文' AFTER model,
  ADD COLUMN access_key_secret_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey Secret密文' AFTER access_key_id_ciphertext,
  ADD COLUMN app_key VARCHAR(128) DEFAULT NULL COMMENT '阿里云NLS AppKey' AFTER access_key_secret_ciphertext,
  ADD COLUMN token_endpoint VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS Token服务地址' AFTER app_key,
  ADD COLUMN token_region_id VARCHAR(64) DEFAULT NULL COMMENT '阿里云NLS Token区域ID' AFTER token_endpoint,
  ADD COLUMN token_refresh_before_seconds INT UNSIGNED NOT NULL DEFAULT 600 COMMENT 'Token提前刷新秒数' AFTER token_region_id,
  ADD COLUMN websocket_url VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS WebSocket流式地址' AFTER token_refresh_before_seconds,
  ADD COLUMN audio_format VARCHAR(32) NOT NULL DEFAULT 'pcm' COMMENT '音频格式' AFTER websocket_url,
  ADD COLUMN sample_rate INT NOT NULL DEFAULT 16000 COMMENT '音频采样率' AFTER audio_format,
  ADD COLUMN enable_intermediate_result TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否返回中间识别结果' AFTER sample_rate,
  ADD COLUMN enable_punctuation_prediction TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用标点预测' AFTER enable_intermediate_result,
  ADD COLUMN enable_inverse_text_normalization TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用数字等逆文本规范化' AFTER enable_punctuation_prediction,
  ADD COLUMN enable_words TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否返回词级时间戳' AFTER enable_inverse_text_normalization,
  ADD COLUMN timeout_ms INT UNSIGNED NOT NULL DEFAULT 120000 COMMENT '流式识别超时时间毫秒' AFTER enable_words;

ALTER TABLE project_tts_configs
  ADD COLUMN access_key_id_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey ID密文' AFTER voice,
  ADD COLUMN access_key_secret_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey Secret密文' AFTER access_key_id_ciphertext,
  ADD COLUMN app_key VARCHAR(128) DEFAULT NULL COMMENT '阿里云NLS AppKey' AFTER access_key_secret_ciphertext,
  ADD COLUMN token_endpoint VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS Token服务地址' AFTER app_key,
  ADD COLUMN token_region_id VARCHAR(64) DEFAULT NULL COMMENT '阿里云NLS Token区域ID' AFTER token_endpoint,
  ADD COLUMN token_refresh_before_seconds INT UNSIGNED NOT NULL DEFAULT 600 COMMENT 'Token提前刷新秒数' AFTER token_region_id,
  ADD COLUMN websocket_url VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS WebSocket流式地址' AFTER token_refresh_before_seconds,
  ADD COLUMN audio_format VARCHAR(32) NOT NULL DEFAULT 'mp3' COMMENT '音频格式' AFTER websocket_url,
  ADD COLUMN sample_rate INT NOT NULL DEFAULT 16000 COMMENT '音频采样率' AFTER audio_format,
  ADD COLUMN volume INT NOT NULL DEFAULT 50 COMMENT '音量，范围0到100' AFTER sample_rate,
  ADD COLUMN speech_rate INT NOT NULL DEFAULT 0 COMMENT '语速，范围-500到500' AFTER volume,
  ADD COLUMN pitch_rate INT NOT NULL DEFAULT 0 COMMENT '语调，范围-500到500' AFTER speech_rate,
  ADD COLUMN enable_subtitle TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用字幕时间戳' AFTER pitch_rate,
  ADD COLUMN timeout_ms INT UNSIGNED NOT NULL DEFAULT 60000 COMMENT '流式合成超时时间毫秒' AFTER enable_subtitle;
