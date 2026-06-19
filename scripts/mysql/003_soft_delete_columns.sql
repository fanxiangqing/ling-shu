-- 仅用于已经使用旧版 001_init_schema.sql 初始化过的数据库。
-- 修复模型嵌入 BaseModel，但表结构缺少 deleted_at 导致 GORM 写入/查询失败的问题。
-- 新库请直接使用当前 001_init_schema.sql，不需要执行本文件。

ALTER TABLE tenant_members
  ADD COLUMN deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间' AFTER updated_at,
  ADD KEY idx_tenant_members_deleted_at (deleted_at);

ALTER TABLE project_members
  ADD COLUMN deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间' AFTER updated_at,
  ADD KEY idx_project_members_deleted_at (deleted_at);

ALTER TABLE roles
  ADD COLUMN deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间' AFTER updated_at,
  ADD KEY idx_roles_deleted_at (deleted_at);

ALTER TABLE sensitive_tables
  ADD COLUMN deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间' AFTER updated_at,
  ADD KEY idx_sensitive_tables_deleted_at (deleted_at);

ALTER TABLE sensitive_columns
  ADD COLUMN deleted_at TIMESTAMP(3) NULL DEFAULT NULL COMMENT '软删除时间' AFTER updated_at,
  ADD KEY idx_sensitive_columns_deleted_at (deleted_at);
