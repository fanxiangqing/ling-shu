-- 仅用于已经使用旧版 001_init_schema.sql 初始化过的数据库。
-- 元数据从“表/字段列表”升级为“表详情 + 索引/外键 + 原始备注/业务备注”。
-- 新库请直接使用当前 001_init_schema.sql，不需要执行本文件。

CREATE TABLE IF NOT EXISTS metadata_indexes (
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

CREATE TABLE IF NOT EXISTS metadata_foreign_keys (
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

ALTER TABLE metadata_tables
  MODIFY COLUMN comment_text VARCHAR(1024) DEFAULT NULL COMMENT '有效注释，业务修订优先，否则使用数据库原始注释',
  ADD COLUMN original_comment_text VARCHAR(1024) DEFAULT NULL COMMENT '数据库原始表注释' AFTER comment_text,
  ADD COLUMN business_comment_text VARCHAR(1024) DEFAULT NULL COMMENT 'Ling-Shu维护的业务表注释' AFTER original_comment_text;

UPDATE metadata_tables
SET original_comment_text = comment_text
WHERE original_comment_text IS NULL AND comment_text IS NOT NULL;

ALTER TABLE metadata_columns
  MODIFY COLUMN comment_text VARCHAR(1024) DEFAULT NULL COMMENT '有效注释，业务修订优先，否则使用数据库原始注释',
  ADD COLUMN original_comment_text VARCHAR(1024) DEFAULT NULL COMMENT '数据库原始字段注释' AFTER comment_text,
  ADD COLUMN business_comment_text VARCHAR(1024) DEFAULT NULL COMMENT 'Ling-Shu维护的业务字段注释' AFTER original_comment_text;

UPDATE metadata_columns
SET original_comment_text = comment_text
WHERE original_comment_text IS NULL AND comment_text IS NOT NULL;
