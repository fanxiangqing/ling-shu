USE ling_shu;

CREATE TABLE IF NOT EXISTS embed_apps (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  app_id VARCHAR(64) NOT NULL COMMENT '内嵌应用公开ID',
  name VARCHAR(128) NOT NULL COMMENT '内嵌应用名称',
  secret_hash VARCHAR(128) NOT NULL COMMENT '内嵌应用密钥哈希',
  secret_ciphertext TEXT DEFAULT NULL COMMENT '内嵌应用密钥密文',
  allowed_origins_json JSON DEFAULT NULL COMMENT '允许嵌入来源JSON',
  session_policy VARCHAR(32) NOT NULL DEFAULT 'context' COMMENT '默认会话策略：user/context/new',
  launcher_title VARCHAR(64) NOT NULL DEFAULT '智能问数' COMMENT '悬浮入口标题',
  welcome_message VARCHAR(255) DEFAULT NULL COMMENT '欢迎语',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '状态',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_embed_apps_app_id (app_id),
  KEY idx_embed_apps_project (tenant_id, project_id),
  KEY idx_embed_apps_status (status),
  KEY idx_embed_apps_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='第三方内嵌应用表';

CREATE TABLE IF NOT EXISTS embed_sessions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  embed_app_id BIGINT UNSIGNED NOT NULL COMMENT '内嵌应用ID',
  app_id VARCHAR(64) NOT NULL COMMENT '内嵌应用公开ID',
  external_user_id VARCHAR(191) NOT NULL COMMENT '第三方用户ID',
  external_user_name VARCHAR(128) DEFAULT NULL COMMENT '第三方用户名称',
  session_key VARCHAR(191) NOT NULL COMMENT '第三方业务会话隔离Key',
  chat_session_id BIGINT UNSIGNED NOT NULL COMMENT 'Ling-Shu会话ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '影子用户ID',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '状态',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  KEY idx_embed_sessions_lookup (embed_app_id, external_user_id, session_key, status),
  KEY idx_embed_sessions_chat (chat_session_id),
  KEY idx_embed_sessions_project (tenant_id, project_id),
  KEY idx_embed_sessions_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='第三方内嵌会话映射表';

SET @has_embed_secret_ciphertext := (
  SELECT COUNT(*)
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'embed_apps'
    AND COLUMN_NAME = 'secret_ciphertext'
);

SET @add_embed_secret_ciphertext_sql := IF(
  @has_embed_secret_ciphertext = 0,
  'ALTER TABLE embed_apps ADD COLUMN secret_ciphertext TEXT DEFAULT NULL COMMENT ''内嵌应用密钥密文'' AFTER secret_hash',
  'SELECT 1'
);
PREPARE add_embed_secret_ciphertext_stmt FROM @add_embed_secret_ciphertext_sql;
EXECUTE add_embed_secret_ciphertext_stmt;
DEALLOCATE PREPARE add_embed_secret_ciphertext_stmt;
