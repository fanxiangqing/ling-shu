import type { Component } from 'vue'
import {
  BookOpen,
  Bot,
  Building2,
  ClipboardList,
  Database,
  UsersRound
} from '@lucide/vue'
import type { AuditSubKey, KnowledgeSubKey, MemberSubKey, ModuleKey } from '@/stores/types'

export const modules: Array<{ key: ModuleKey; label: string; hint: string; icon: Component }> = [
  { key: 'project', label: '项目管理', hint: '项目和数据源绑定', icon: Building2 },
  { key: 'datasource', label: '数据源管理', hint: '连接、同步和元数据', icon: Database },
  { key: 'chat', label: '对话管理', hint: '创建会话，进入自然语言问数', icon: Bot },
  { key: 'members', label: '成员管理', hint: '创建账号，加入项目', icon: UsersRound },
  { key: 'knowledge', label: '业务知识', hint: '术语、指标和示例问法', icon: BookOpen },
  { key: 'audit', label: '审计查询', hint: '操作日志和查询记录', icon: ClipboardList }
]

export const memberSubmenus: Array<{ key: MemberSubKey; label: string; hint: string }> = [
  { key: 'invite', label: '邀请成员', hint: '创建组织账号' },
  { key: 'projectAccess', label: '项目授权', hint: '加入项目和分配角色' },
  { key: 'directory', label: '成员目录', hint: '查看组织和项目成员' }
]

export const knowledgeSubmenus: Array<{ key: KnowledgeSubKey; label: string; hint: string }> = [
  { key: 'terms', label: '业务术语', hint: '简称、黑话和同义词' },
  { key: 'metrics', label: '指标口径', hint: '指标公式和时间字段' },
  { key: 'fewShots', label: '示例问法', hint: '标准问题和 SQL 示例' },
  { key: 'rag', label: '索引维护', hint: '检索和重建知识索引' }
]

export const auditSubmenus: Array<{ key: AuditSubKey; label: string; hint: string }> = [
  { key: 'operationLogs', label: '操作日志', hint: '资源和成员行为' },
  { key: 'queryExecutions', label: '查询记录', hint: '问数执行链路' }
]
