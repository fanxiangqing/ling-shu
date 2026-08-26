-- 仅用于已经使用旧版 001_init_schema.sql 初始化过的数据库。
-- 新库请直接使用当前 001_init_schema.sql，不需要执行本文件。

CREATE TABLE IF NOT EXISTS chat_session_contexts (
  session_id BIGINT UNSIGNED NOT NULL COMMENT '问数会话ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  summary TEXT DEFAULT NULL COMMENT '最近一轮精简摘要',
  last_intent VARCHAR(64) DEFAULT NULL COMMENT '最近一轮意图',
  active_artifact_ids JSON NOT NULL COMMENT '当前有效产物ID列表',
  focus_artifact_id BIGINT UNSIGNED DEFAULT NULL COMMENT '当前焦点产物ID',
  version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '上下文版本',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (session_id),
  KEY idx_chat_session_contexts_scope (tenant_id, project_id, user_id),
  KEY idx_chat_session_contexts_focus (focus_artifact_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='问数会话结构化上下文表';

CREATE TABLE IF NOT EXISTS chat_artifacts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  session_id BIGINT UNSIGNED NOT NULL COMMENT '问数会话ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  source_message_id BIGINT UNSIGNED NOT NULL COMMENT '来源助手消息ID',
  source_query_execution_id BIGINT UNSIGNED DEFAULT NULL COMMENT '来源查询执行ID',
  kind VARCHAR(32) NOT NULL COMMENT '产物类型',
  purpose VARCHAR(512) DEFAULT NULL COMMENT '产物业务用途',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT '产物状态',
  completeness VARCHAR(32) NOT NULL DEFAULT 'bounded' COMMENT '数据完整性',
  payload_json JSON NOT NULL COMMENT '列、行、图表和答案快照',
  semantics_json JSON NOT NULL COMMENT '维度、指标和单位语义',
  lineage_json JSON NOT NULL COMMENT '查询执行、数据源和派生血缘',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  KEY idx_chat_artifacts_session (session_id, status, id),
  KEY idx_chat_artifacts_scope (tenant_id, project_id, user_id),
  KEY idx_chat_artifacts_message (source_message_id),
  KEY idx_chat_artifacts_execution (source_query_execution_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='问数会话可复用结果产物表';
