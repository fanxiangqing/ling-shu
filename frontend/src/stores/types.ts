export type ModuleKey = 'project' | 'datasource' | 'chat' | 'members' | 'knowledge' | 'audit'
export type MemberSubKey = 'invite' | 'projectAccess' | 'directory'
export type KnowledgeSubKey = 'terms' | 'metrics' | 'fewShots' | 'rag'
export type AuditSubKey = 'operationLogs' | 'queryExecutions'

export type ProviderConfigMode = 'global' | 'custom' | 'disabled'
export type ProjectProviderModes = { llm: ProviderConfigMode; asr: ProviderConfigMode; tts: ProviderConfigMode }

export type SavedNavState = {
  activeModule?: ModuleKey
  activeMemberSub?: MemberSubKey
  activeKnowledgeSub?: KnowledgeSubKey
  activeAuditSub?: AuditSubKey
}

export type PageKey =
  | 'tenants'
  | 'projects'
  | 'datasources'
  | 'projectDatasources'
  | 'metadataTables'
  | 'users'
  | 'tenantMembers'
  | 'projectMembers'
  | 'sessions'
  | 'terms'
  | 'metrics'
  | 'fewShots'
  | 'auditLogs'
  | 'auditQueries'

export type RefreshFn = (options?: { silent?: boolean }) => Promise<unknown>
