-- 仅用于已经使用旧版 001_init_schema.sql 初始化过的数据库。
-- 数据源从“项目独占”升级为“租户级资源池 + 项目绑定”。
-- 新库请直接使用当前 001_init_schema.sql，不需要执行本文件。

CREATE TABLE IF NOT EXISTS project_datasources (
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

INSERT IGNORE INTO project_datasources (tenant_id, project_id, datasource_id, created_by)
SELECT tenant_id, project_id, id, created_by
FROM datasources
WHERE project_id > 0;

ALTER TABLE datasources
  DROP INDEX uk_datasources_project_name,
  ADD UNIQUE KEY uk_datasources_tenant_name (tenant_id, name),
  MODIFY COLUMN project_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '项目ID，0表示租户级数据源，项目绑定见project_datasources';
