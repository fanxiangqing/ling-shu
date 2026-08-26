package memory

import (
	"context"
	"time"
)

const (
	UserMemoryKindPreference  = "preference"
	UserMemoryKindProfile     = "profile"
	UserMemoryKindConvention  = "convention"
	UserMemoryKindInstruction = "instruction"
	UserMemoryKindCorrection  = "correction"

	UserMemoryStatusCandidate   = "candidate"
	UserMemoryStatusActive      = "active"
	UserMemoryStatusSuperseded  = "superseded"
	UserMemoryStatusRevoked     = "revoked"
	UserMemoryStatusExpired     = "expired"
	UserMemoryStatusQuarantined = "quarantined"

	UserMemorySourceExplicit  = "explicit"
	UserMemorySourceConfirmed = "confirmed"
	UserMemorySourceInferred  = "inferred"
	UserMemorySourceImported  = "imported"

	UserMemorySensitivityNormal     = "normal"
	UserMemorySensitivitySensitive  = "sensitive"
	UserMemorySensitivityRestricted = "restricted"

	UserMemoryScopeProject = "project"
	UserMemoryScopeTenant  = "tenant"

	UserMemoryActionNone     = "none"
	UserMemoryActionRemember = "remember"
	UserMemoryActionForget   = "forget"
	UserMemoryActionList     = "list"
	UserMemoryActionClear    = "clear"
	UserMemoryActionConfirm  = "confirm"
	UserMemoryActionReject   = "reject"
)

type UserScope struct {
	TenantID  uint64 `json:"tenant_id"`
	ProjectID uint64 `json:"project_id"`
	UserID    uint64 `json:"user_id"`
}

func (s UserScope) Valid() bool {
	return s.TenantID > 0 && s.UserID > 0
}

type UserMemory struct {
	ID                uint64         `json:"id"`
	Scope             UserScope      `json:"scope"`
	Kind              string         `json:"kind"`
	Key               string         `json:"memory_key,omitempty"`
	CanonicalHash     string         `json:"-"`
	Content           string         `json:"content"`
	Value             map[string]any `json:"value,omitempty"`
	Applicability     map[string]any `json:"applicability,omitempty"`
	Status            string         `json:"status"`
	SourceType        string         `json:"source_type"`
	Confidence        float64        `json:"confidence"`
	Salience          float64        `json:"salience"`
	Sensitivity       string         `json:"sensitivity"`
	EvidenceCount     int            `json:"evidence_count"`
	Version           uint64         `json:"version"`
	ObservedAt        *time.Time     `json:"observed_at,omitempty"`
	ValidFrom         *time.Time     `json:"valid_from,omitempty"`
	ExpiresAt         *time.Time     `json:"expires_at,omitempty"`
	LastConfirmedAt   *time.Time     `json:"last_confirmed_at,omitempty"`
	LastRecalledAt    *time.Time     `json:"last_recalled_at,omitempty"`
	LastAppliedAt     *time.Time     `json:"last_applied_at,omitempty"`
	ApplyCount        int            `json:"apply_count"`
	SourceSessionID   uint64         `json:"source_session_id,omitempty"`
	SourceMessageID   uint64         `json:"source_message_id,omitempty"`
	EmbeddingProvider string         `json:"embedding_provider,omitempty"`
	EmbeddingModel    string         `json:"embedding_model,omitempty"`
	VectorID          string         `json:"vector_id,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (m UserMemory) ScopeLevel() string {
	if m.Scope.ProjectID == 0 {
		return UserMemoryScopeTenant
	}
	return UserMemoryScopeProject
}

func (m UserMemory) IsUsable(now time.Time) bool {
	if m.Status != UserMemoryStatusActive || m.Sensitivity == UserMemorySensitivityRestricted {
		return false
	}
	return m.ExpiresAt == nil || m.ExpiresAt.After(now)
}

type UserMemoryEvidence struct {
	ID              uint64    `json:"id"`
	MemoryID        uint64    `json:"memory_id"`
	Scope           UserScope `json:"scope"`
	SessionID       uint64    `json:"session_id,omitempty"`
	MessageID       uint64    `json:"message_id,omitempty"`
	EvidenceType    string    `json:"evidence_type"`
	EvidenceSummary string    `json:"evidence_summary,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type UserMemoryEvent struct {
	ID          uint64         `json:"id"`
	MemoryID    uint64         `json:"memory_id,omitempty"`
	Scope       UserScope      `json:"scope"`
	Operation   string         `json:"operation"`
	MemoryKey   string         `json:"memory_key,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	ActorUserID uint64         `json:"actor_user_id"`
	CreatedAt   time.Time      `json:"created_at"`
}

type SessionEpisode struct {
	ID                uint64         `json:"id"`
	Scope             UserScope      `json:"scope"`
	SessionID         uint64         `json:"session_id"`
	Summary           string         `json:"summary"`
	Topics            []string       `json:"topics,omitempty"`
	Decisions         []string       `json:"decisions,omitempty"`
	OpenLoops         []string       `json:"open_loops,omitempty"`
	QueryExecutionIDs []uint64       `json:"query_execution_ids,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	ExpiresAt         *time.Time     `json:"expires_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type UserMemoryJob struct {
	ID        uint64         `json:"id"`
	Scope     UserScope      `json:"scope"`
	SessionID uint64         `json:"session_id,omitempty"`
	MessageID uint64         `json:"message_id,omitempty"`
	JobType   string         `json:"job_type"`
	Payload   map[string]any `json:"payload,omitempty"`
	Status    string         `json:"status"`
	Attempts  int            `json:"attempts"`
	RunAfter  time.Time      `json:"run_after"`
	LastError string         `json:"last_error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type UserMemoryListFilter struct {
	Scope    UserScope
	Statuses []string
	Kinds    []string
	Limit    int
	Offset   int
}

type UserMemoryStore interface {
	List(ctx context.Context, filter UserMemoryListFilter) ([]UserMemory, int64, error)
	ListApplicable(ctx context.Context, scope UserScope, now time.Time, limit int) ([]UserMemory, error)
	Get(ctx context.Context, scope UserScope, id uint64) (UserMemory, error)
	Upsert(ctx context.Context, item UserMemory, evidence UserMemoryEvidence, event UserMemoryEvent) (UserMemory, error)
	Update(ctx context.Context, item UserMemory, event UserMemoryEvent) (UserMemory, error)
	SetStatus(ctx context.Context, scope UserScope, id uint64, status string, event UserMemoryEvent) (UserMemory, error)
	Delete(ctx context.Context, scope UserScope, id uint64, event UserMemoryEvent) error
	Clear(ctx context.Context, scope UserScope, includeTenant bool, event UserMemoryEvent) (int64, error)
	MarkRecalled(ctx context.Context, scope UserScope, ids []uint64, applied bool) error
	SetVectorMetadata(ctx context.Context, scope UserScope, id uint64, metadata UserMemoryVectorMetadata) error
	UpsertEpisode(ctx context.Context, episode SessionEpisode) (SessionEpisode, error)
	ListEpisodes(ctx context.Context, scope UserScope, limit int) ([]SessionEpisode, error)
	EnqueueJob(ctx context.Context, job UserMemoryJob) error
	ClaimJobs(ctx context.Context, now time.Time, limit int) ([]UserMemoryJob, error)
	CompleteJob(ctx context.Context, id uint64) error
	FailJob(ctx context.Context, id uint64, err error, retryAt time.Time) error
}

type UserMemoryOperation struct {
	Action     string         `json:"action"`
	TargetID   uint64         `json:"target_id,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Key        string         `json:"memory_key,omitempty"`
	Content    string         `json:"content,omitempty"`
	Value      map[string]any `json:"value,omitempty"`
	ScopeLevel string         `json:"scope_level,omitempty"`
	Confidence float64        `json:"confidence,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
}

func (o UserMemoryOperation) Handled() bool {
	return o.Action != "" && o.Action != UserMemoryActionNone
}

type UserMemoryPromptItem struct {
	ID         uint64 `json:"id"`
	Kind       string `json:"kind"`
	ScopeLevel string `json:"scope_level"`
	Content    string `json:"content"`
}

type UserMemoryContext struct {
	Memories     []UserMemory
	PromptItems  []UserMemoryPromptItem
	Episodes     []SessionEpisode
	Operation    UserMemoryOperation
	AppliedIDs   []uint64
	DefaultChart string
	DetailLevel  string
	Timezone     string
}

type UserMemoryOperationResult struct {
	Answer   string       `json:"answer"`
	Memory   *UserMemory  `json:"memory,omitempty"`
	Memories []UserMemory `json:"memories,omitempty"`
	Deleted  int64        `json:"deleted,omitempty"`
}

type UserTurnInput struct {
	Scope              UserScope
	SessionID          uint64
	UserMessageID      uint64
	AssistantMessageID uint64
	Question           string
	Answer             string
	Intent             string
	Timezone           string
	OccurredAt         time.Time
	QueryExecutionIDs  []uint64
}
