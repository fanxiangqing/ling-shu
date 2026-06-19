CREATE DATABASE IF NOT EXISTS ling_shu DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
USE ling_shu;

CREATE TABLE tenants (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  name VARCHAR(128) NOT NULL COMMENT '租户名称',
  code VARCHAR(64) NOT NULL COMMENT '租户唯一编码',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '租户状态',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenants_code (code),
  KEY idx_tenants_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='租户表';

CREATE TABLE users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  username VARCHAR(64) NOT NULL COMMENT '用户名',
  email VARCHAR(191) DEFAULT NULL COMMENT '邮箱',
  mobile VARCHAR(32) DEFAULT NULL COMMENT '手机号',
  password_hash VARCHAR(255) NOT NULL COMMENT '密码哈希',
  display_name VARCHAR(128) NOT NULL COMMENT '显示名称',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '用户状态',
  last_login_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '最后登录时间',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_username (username),
  UNIQUE KEY uk_users_email (email),
  KEY idx_users_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户表';

CREATE TABLE tenant_members (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '租户成员状态',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_members_tenant_user (tenant_id, user_id),
  KEY idx_tenant_members_user (user_id),
  KEY idx_tenant_members_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='租户成员关系表';

CREATE TABLE projects (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  name VARCHAR(128) NOT NULL COMMENT '项目名称',
  code VARCHAR(64) NOT NULL COMMENT '租户内项目唯一编码',
  description VARCHAR(512) DEFAULT NULL COMMENT '描述',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '项目状态',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_projects_tenant_code (tenant_id, code),
  KEY idx_projects_tenant_status (tenant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目表';

CREATE TABLE project_members (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '项目成员状态',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_project_members_project_user (project_id, user_id),
  KEY idx_project_members_tenant_user (tenant_id, user_id),
  KEY idx_project_members_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目成员关系表';

CREATE TABLE project_datasources (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_project_datasources_project_ds (tenant_id, project_id, datasource_id),
  KEY idx_project_datasources_ds (datasource_id),
  KEY idx_project_datasources_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目数据源绑定表';

CREATE TABLE roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  code VARCHAR(64) NOT NULL COMMENT '角色编码',
  name VARCHAR(128) NOT NULL COMMENT '角色名称',
  scope_type VARCHAR(32) NOT NULL COMMENT '角色作用域类型',
  builtin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否内置角色',
  description VARCHAR(512) DEFAULT NULL COMMENT '描述',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_roles_code (code),
  KEY idx_roles_scope_type (scope_type),
  KEY idx_roles_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='角色表';

CREATE TABLE permissions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  code VARCHAR(128) NOT NULL COMMENT '权限编码',
  resource VARCHAR(64) NOT NULL COMMENT '权限资源',
  action VARCHAR(64) NOT NULL COMMENT '权限操作',
  description VARCHAR(512) DEFAULT NULL COMMENT '描述',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_permissions_code (code),
  KEY idx_permissions_resource_action (resource, action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='权限点表';

CREATE TABLE role_permissions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  permission_id BIGINT UNSIGNED NOT NULL COMMENT '权限ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_role_permissions_role_perm (role_id, permission_id),
  KEY idx_role_permissions_perm (permission_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='角色权限关系表';

CREATE TABLE role_bindings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  tenant_id BIGINT UNSIGNED DEFAULT NULL COMMENT '授权租户ID，全局角色可为空',
  project_id BIGINT UNSIGNED DEFAULT NULL COMMENT '授权项目ID，非项目级角色可为空',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_role_bindings_scope (user_id, role_id, tenant_id, project_id),
  KEY idx_role_bindings_user (user_id),
  KEY idx_role_bindings_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户角色绑定表';

CREATE TABLE datasources (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '项目ID，0表示租户级数据源，项目绑定见project_datasources',
  name VARCHAR(128) NOT NULL COMMENT '数据源名称',
  db_type VARCHAR(64) NOT NULL COMMENT '数据库类型',
  dsn_ciphertext TEXT NOT NULL COMMENT '数据源连接信息密文',
  config_json JSON DEFAULT NULL COMMENT '扩展配置JSON',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '数据源状态',
  last_sync_status VARCHAR(32) DEFAULT NULL COMMENT '最近同步状态',
  last_sync_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '最近同步时间',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_datasources_tenant_name (tenant_id, name),
  KEY idx_datasources_tenant_project (tenant_id, project_id),
  KEY idx_datasources_db_type (db_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='租户数据源表';

CREATE TABLE metadata_schemas (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  schema_name VARCHAR(191) NOT NULL COMMENT 'Schema名称',
  comment_text VARCHAR(1024) DEFAULT NULL COMMENT '有效注释，业务修订优先，否则使用数据库原始注释',
  original_comment_text VARCHAR(1024) DEFAULT NULL COMMENT '数据库原始表注释',
  business_comment_text VARCHAR(1024) DEFAULT NULL COMMENT 'Ling-Shu维护的业务表注释',
  synced_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '同步时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_metadata_schemas_ds_schema (datasource_id, schema_name),
  KEY idx_metadata_schemas_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据源Schema元数据表';

CREATE TABLE metadata_tables (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  schema_name VARCHAR(191) NOT NULL COMMENT 'Schema名称',
  table_name VARCHAR(191) NOT NULL COMMENT '表名',
  table_type VARCHAR(32) NOT NULL DEFAULT 'table' COMMENT '表类型',
  comment_text VARCHAR(1024) DEFAULT NULL COMMENT '有效注释，业务修订优先，否则使用数据库原始注释',
  original_comment_text VARCHAR(1024) DEFAULT NULL COMMENT '数据库原始表注释',
  business_comment_text VARCHAR(1024) DEFAULT NULL COMMENT 'Ling-Shu维护的业务表注释',
  row_count BIGINT DEFAULT NULL COMMENT '行数',
  synced_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '同步时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_metadata_tables_ds_schema_table (datasource_id, schema_name, table_name),
  KEY idx_metadata_tables_project (tenant_id, project_id),
  KEY idx_metadata_tables_table_name (table_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据源表元数据表';

CREATE TABLE metadata_columns (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  table_id BIGINT UNSIGNED NOT NULL COMMENT '元数据表ID',
  column_name VARCHAR(191) NOT NULL COMMENT '字段名',
  ordinal_position INT NOT NULL COMMENT '字段顺序',
  data_type VARCHAR(128) NOT NULL COMMENT '数据类型',
  column_type VARCHAR(255) DEFAULT NULL COMMENT '字段完整类型',
  nullable TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否允许为空',
  default_value VARCHAR(512) DEFAULT NULL COMMENT '默认值',
  is_primary_key TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否主键字段',
  is_foreign_key TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否外键字段',
  comment_text VARCHAR(1024) DEFAULT NULL COMMENT '有效注释，业务修订优先，否则使用数据库原始注释',
  original_comment_text VARCHAR(1024) DEFAULT NULL COMMENT '数据库原始字段注释',
  business_comment_text VARCHAR(1024) DEFAULT NULL COMMENT 'Ling-Shu维护的业务字段注释',
  synced_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '同步时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_metadata_columns_table_column (table_id, column_name),
  KEY idx_metadata_columns_project (tenant_id, project_id),
  KEY idx_metadata_columns_column_name (column_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据源字段元数据表';

CREATE TABLE metadata_indexes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  table_id BIGINT UNSIGNED NOT NULL COMMENT '元数据表ID',
  index_name VARCHAR(191) NOT NULL COMMENT '索引名',
  index_type VARCHAR(64) DEFAULT NULL COMMENT '索引类型',
  unique_index TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否唯一索引',
  columns_json JSON NOT NULL COMMENT '索引字段JSON',
  synced_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '同步时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_metadata_indexes_table_index (table_id, index_name),
  KEY idx_metadata_indexes_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据源索引元数据表';

CREATE TABLE metadata_foreign_keys (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  table_id BIGINT UNSIGNED NOT NULL COMMENT '元数据表ID',
  constraint_name VARCHAR(191) NOT NULL COMMENT '外键约束名',
  column_name VARCHAR(191) NOT NULL COMMENT '字段名',
  referenced_schema VARCHAR(191) DEFAULT NULL COMMENT '关联Schema',
  referenced_table VARCHAR(191) NOT NULL COMMENT '关联表',
  referenced_column VARCHAR(191) NOT NULL COMMENT '关联字段',
  synced_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '同步时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_metadata_fks_table_constraint_column (table_id, constraint_name, column_name),
  KEY idx_metadata_fks_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据源外键元数据表';

CREATE TABLE metadata_sync_jobs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
  trigger_type VARCHAR(32) NOT NULL COMMENT '同步触发方式',
  status VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT '同步任务状态',
  error_message TEXT DEFAULT NULL COMMENT '错误信息',
  started_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '开始时间',
  finished_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '完成时间',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  KEY idx_metadata_sync_jobs_ds_status (datasource_id, status),
  KEY idx_metadata_sync_jobs_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='元数据同步任务表';

CREATE TABLE kb_terms (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  term VARCHAR(191) NOT NULL COMMENT '业务术语',
  aliases_json JSON DEFAULT NULL COMMENT '术语别名JSON',
  definition TEXT NOT NULL COMMENT '术语定义',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_kb_terms_project_term (project_id, term),
  KEY idx_kb_terms_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='业务术语知识库表';

CREATE TABLE kb_metrics (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  name VARCHAR(191) NOT NULL COMMENT '指标名称',
  description TEXT NOT NULL COMMENT '描述',
  formula TEXT NOT NULL COMMENT '指标计算口径',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  default_time_column VARCHAR(191) DEFAULT NULL COMMENT '默认时间字段',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_kb_metrics_project_name (project_id, name),
  KEY idx_kb_metrics_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标定义知识库表';

CREATE TABLE kb_fewshot_sql (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  question TEXT NOT NULL COMMENT '用户问题',
  sql_text TEXT NOT NULL COMMENT 'SQL文本',
  explanation TEXT DEFAULT NULL COMMENT '解释说明',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  KEY idx_kb_fewshot_project_ds (project_id, datasource_id),
  KEY idx_kb_fewshot_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='FewShot SQL知识库表';

CREATE TABLE kb_documents (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  title VARCHAR(255) NOT NULL COMMENT '标题',
  source_type VARCHAR(64) NOT NULL DEFAULT 'manual' COMMENT '来源类型',
  source_uri VARCHAR(1024) DEFAULT NULL COMMENT '来源地址',
  content_hash VARCHAR(128) DEFAULT NULL COMMENT '内容哈希',
  status VARCHAR(32) NOT NULL DEFAULT 'ready' COMMENT '文档处理状态',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  KEY idx_kb_documents_project (tenant_id, project_id),
  KEY idx_kb_documents_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='知识库文档表';

CREATE TABLE kb_chunks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  kb_type VARCHAR(64) NOT NULL COMMENT '知识类型',
  ref_id BIGINT UNSIGNED NOT NULL COMMENT '引用业务ID',
  chunk_no INT NOT NULL DEFAULT 0 COMMENT '切片序号',
  chunk_text TEXT NOT NULL COMMENT '切片内容',
  embedding_provider VARCHAR(64) DEFAULT NULL COMMENT 'Embedding服务商',
  embedding_model VARCHAR(128) DEFAULT NULL COMMENT 'Embedding模型',
  vector_collection VARCHAR(128) DEFAULT NULL COMMENT '向量集合名',
  vector_id VARCHAR(128) DEFAULT NULL COMMENT '向量ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  KEY idx_kb_chunks_project_type (tenant_id, project_id, kb_type),
  KEY idx_kb_chunks_ref (kb_type, ref_id),
  KEY idx_kb_chunks_vector (vector_collection, vector_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='知识库向量切片表';

CREATE TABLE project_llm_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  provider VARCHAR(64) NOT NULL COMMENT '服务商',
  model VARCHAR(128) NOT NULL COMMENT '模型名称',
  api_base VARCHAR(512) DEFAULT NULL COMMENT 'API基础地址',
  api_key_ciphertext TEXT DEFAULT NULL COMMENT 'API密钥密文',
  config_json JSON DEFAULT NULL COMMENT '扩展配置JSON',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用LLM配置',
  is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否默认配置',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_project_llm_configs_project (tenant_id, project_id),
  KEY idx_project_llm_configs_provider (provider, model)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目LLM配置表';

CREATE TABLE project_asr_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  provider VARCHAR(64) NOT NULL COMMENT '服务商',
  model VARCHAR(128) NOT NULL COMMENT '模型名称',
  access_key_id_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey ID密文',
  access_key_secret_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey Secret密文',
  app_key VARCHAR(128) DEFAULT NULL COMMENT '阿里云NLS AppKey',
  token_endpoint VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS Token服务地址',
  token_region_id VARCHAR(64) DEFAULT NULL COMMENT '阿里云NLS Token区域ID',
  token_refresh_before_seconds INT UNSIGNED NOT NULL DEFAULT 600 COMMENT 'Token提前刷新秒数',
  websocket_url VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS WebSocket流式地址',
  audio_format VARCHAR(32) NOT NULL DEFAULT 'pcm' COMMENT '音频格式',
  sample_rate INT NOT NULL DEFAULT 16000 COMMENT '音频采样率',
  enable_intermediate_result TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否返回中间识别结果',
  enable_punctuation_prediction TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用标点预测',
  enable_inverse_text_normalization TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用数字等逆文本规范化',
  enable_words TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否返回词级时间戳',
  timeout_ms INT UNSIGNED NOT NULL DEFAULT 120000 COMMENT '流式识别超时时间毫秒',
  config_json JSON DEFAULT NULL COMMENT '扩展配置JSON',
  enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用ASR配置',
  is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否默认配置',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_project_asr_configs_project (tenant_id, project_id),
  KEY idx_project_asr_configs_provider (provider, model)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目ASR配置表';

CREATE TABLE project_tts_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  provider VARCHAR(64) NOT NULL COMMENT '服务商',
  model VARCHAR(128) NOT NULL COMMENT '模型名称',
  voice VARCHAR(128) DEFAULT NULL COMMENT '音色',
  access_key_id_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey ID密文',
  access_key_secret_ciphertext TEXT DEFAULT NULL COMMENT '阿里云AccessKey Secret密文',
  app_key VARCHAR(128) DEFAULT NULL COMMENT '阿里云NLS AppKey',
  token_endpoint VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS Token服务地址',
  token_region_id VARCHAR(64) DEFAULT NULL COMMENT '阿里云NLS Token区域ID',
  token_refresh_before_seconds INT UNSIGNED NOT NULL DEFAULT 600 COMMENT 'Token提前刷新秒数',
  websocket_url VARCHAR(512) DEFAULT NULL COMMENT '阿里云NLS WebSocket流式地址',
  audio_format VARCHAR(32) NOT NULL DEFAULT 'mp3' COMMENT '音频格式',
  sample_rate INT NOT NULL DEFAULT 16000 COMMENT '音频采样率',
  volume INT NOT NULL DEFAULT 50 COMMENT '音量，范围0到100',
  speech_rate INT NOT NULL DEFAULT 0 COMMENT '语速，范围-500到500',
  pitch_rate INT NOT NULL DEFAULT 0 COMMENT '语调，范围-500到500',
  enable_subtitle TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用字幕时间戳',
  timeout_ms INT UNSIGNED NOT NULL DEFAULT 60000 COMMENT '流式合成超时时间毫秒',
  config_json JSON DEFAULT NULL COMMENT '扩展配置JSON',
  enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用TTS配置',
  is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否默认配置',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_project_tts_configs_project (tenant_id, project_id),
  KEY idx_project_tts_configs_provider (provider, model)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目TTS配置表';

CREATE TABLE chat_sessions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  title VARCHAR(255) NOT NULL DEFAULT '新会话' COMMENT '标题',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '会话状态',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  KEY idx_chat_sessions_user_project (user_id, project_id),
  KEY idx_chat_sessions_tenant_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='问数会话表';

CREATE TABLE chat_messages (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  session_id BIGINT UNSIGNED NOT NULL COMMENT '会话ID',
  user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '用户ID',
  role VARCHAR(32) NOT NULL COMMENT '消息角色，user或assistant等',
  content MEDIUMTEXT NOT NULL COMMENT '消息内容',
  content_type VARCHAR(32) NOT NULL DEFAULT 'text' COMMENT '内容类型',
  query_execution_id BIGINT UNSIGNED DEFAULT NULL COMMENT '查询执行ID',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  KEY idx_chat_messages_session (session_id, id),
  KEY idx_chat_messages_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='问数消息表';

CREATE TABLE embed_apps (
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

CREATE TABLE embed_sessions (
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

CREATE TABLE query_executions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  session_id BIGINT UNSIGNED DEFAULT NULL COMMENT '会话ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  question TEXT NOT NULL COMMENT '用户问题',
  generated_sql MEDIUMTEXT DEFAULT NULL COMMENT 'LLM生成SQL',
  final_sql MEDIUMTEXT DEFAULT NULL COMMENT '最终执行SQL',
  sql_hash CHAR(64) DEFAULT NULL COMMENT 'SQL哈希',
  llm_provider VARCHAR(64) DEFAULT NULL COMMENT 'LLM服务商',
  llm_model VARCHAR(128) DEFAULT NULL COMMENT 'LLM模型',
  prompt_tokens INT DEFAULT NULL COMMENT 'Prompt Token数',
  completion_tokens INT DEFAULT NULL COMMENT 'Completion Token数',
  total_tokens INT DEFAULT NULL COMMENT '总Token数',
  status VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT '查询执行状态',
  row_count INT DEFAULT NULL COMMENT '结果行数',
  duration_ms INT DEFAULT NULL COMMENT '执行耗时毫秒',
  chart_type VARCHAR(64) DEFAULT NULL COMMENT '图表类型',
  result_preview_json JSON DEFAULT NULL COMMENT '结果预览JSON',
  error_message TEXT DEFAULT NULL COMMENT '错误信息',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  finished_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '完成时间',
  PRIMARY KEY (id),
  KEY idx_query_executions_project_user (tenant_id, project_id, user_id),
  KEY idx_query_executions_datasource (datasource_id),
  KEY idx_query_executions_sql_hash (sql_hash),
  KEY idx_query_executions_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='查询执行记录表';

CREATE TABLE sql_review_results (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  query_execution_id BIGINT UNSIGNED DEFAULT NULL COMMENT '查询执行ID',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  sql_text MEDIUMTEXT NOT NULL COMMENT 'SQL文本',
  passed TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'SQL是否通过安全审核',
  risk_level VARCHAR(32) NOT NULL DEFAULT 'low' COMMENT '风险等级',
  blocked_reason TEXT DEFAULT NULL COMMENT '拦截原因',
  rules_json JSON DEFAULT NULL COMMENT '命中规则JSON',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  KEY idx_sql_review_results_query (query_execution_id),
  KEY idx_sql_review_results_project (tenant_id, project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='SQL安全审核结果表';

CREATE TABLE sensitive_tables (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  schema_name VARCHAR(191) DEFAULT NULL COMMENT 'Schema名称',
  table_name VARCHAR(191) NOT NULL COMMENT '表名',
  risk_level VARCHAR(32) NOT NULL DEFAULT 'high' COMMENT '风险等级',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '敏感表规则是否启用',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sensitive_tables_scope (project_id, datasource_id, schema_name, table_name),
  KEY idx_sensitive_tables_project (tenant_id, project_id),
  KEY idx_sensitive_tables_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='敏感表规则表';

CREATE TABLE sensitive_columns (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  datasource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '数据源ID',
  schema_name VARCHAR(191) DEFAULT NULL COMMENT 'Schema名称',
  table_name VARCHAR(191) NOT NULL COMMENT '表名',
  column_name VARCHAR(191) NOT NULL COMMENT '字段名',
  mask_type VARCHAR(64) NOT NULL DEFAULT 'redact' COMMENT '脱敏方式',
  risk_level VARCHAR(32) NOT NULL DEFAULT 'medium' COMMENT '风险等级',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '敏感字段规则是否启用',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sensitive_columns_scope (project_id, datasource_id, schema_name, table_name, column_name),
  KEY idx_sensitive_columns_project (tenant_id, project_id),
  KEY idx_sensitive_columns_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='敏感字段规则表';

CREATE TABLE audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED DEFAULT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED DEFAULT NULL COMMENT '项目ID',
  user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '用户ID',
  event_type VARCHAR(128) NOT NULL COMMENT '审计事件类型',
  resource_type VARCHAR(64) DEFAULT NULL COMMENT '资源类型',
  resource_id BIGINT UNSIGNED DEFAULT NULL COMMENT '资源ID',
  request_id VARCHAR(128) DEFAULT NULL COMMENT '请求ID',
  ip VARCHAR(64) DEFAULT NULL COMMENT '客户端IP',
  user_agent VARCHAR(512) DEFAULT NULL COMMENT 'User-Agent',
  payload_json JSON DEFAULT NULL COMMENT '审计载荷JSON',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  KEY idx_audit_logs_scope_time (tenant_id, project_id, created_at),
  KEY idx_audit_logs_user_time (user_id, created_at),
  KEY idx_audit_logs_event_type (event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='审计日志表';

INSERT INTO roles (code, name, scope_type, builtin, description) VALUES
  ('super_admin', 'SuperAdmin', 'global', 1, '平台超级管理员'),
  ('tenant_admin', 'TenantAdmin', 'tenant', 1, '租户管理员'),
  ('project_admin', 'ProjectAdmin', 'project', 1, '项目管理员'),
  ('analyst', 'Analyst', 'project', 1, '分析师'),
  ('viewer', 'Viewer', 'project', 1, '只读查看者');

INSERT INTO permissions (code, resource, action, description) VALUES
  ('tenant.manage', 'tenant', 'manage', '管理租户'),
  ('project.manage', 'project', 'manage', '管理项目'),
  ('datasource.manage', 'datasource', 'manage', '管理数据源'),
  ('metadata.sync', 'metadata', 'sync', '同步元数据'),
  ('kb.manage', 'kb', 'manage', '管理知识库'),
  ('chat.use', 'chat', 'use', '使用问数'),
  ('query.execute', 'query', 'execute', '执行查询'),
  ('query.view_sql', 'query', 'view_sql', '查看 SQL'),
  ('audit.view', 'audit', 'view', '查看审计');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'super_admin'
UNION ALL
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code IN (
  'tenant.manage', 'project.manage', 'datasource.manage', 'metadata.sync',
  'kb.manage', 'chat.use', 'query.execute', 'query.view_sql', 'audit.view'
)
WHERE r.code = 'tenant_admin'
UNION ALL
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code IN (
  'project.manage', 'datasource.manage', 'metadata.sync',
  'kb.manage', 'chat.use', 'query.execute', 'query.view_sql', 'audit.view'
)
WHERE r.code = 'project_admin'
UNION ALL
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code IN (
  'chat.use', 'query.execute', 'query.view_sql'
)
WHERE r.code = 'analyst'
UNION ALL
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code IN (
  'chat.use'
)
WHERE r.code = 'viewer';
