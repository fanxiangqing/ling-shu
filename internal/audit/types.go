package audit

import "context"

const (
	EventSQLReview    = "sql.review"
	EventQueryExecute = "query.execute"
	EventChatMessage  = "chat.message"
	EventMetadataEdit = "metadata.comment.update"

	ResourceSQLReview      = "sql_review"
	ResourceQueryExecution = "query_execution"
	ResourceChatSession    = "chat_session"
	ResourceMetadataTable  = "metadata_table"
	ResourceMetadataColumn = "metadata_column"
)

type Event struct {
	TenantID     uint64
	ProjectID    uint64
	UserID       uint64
	EventType    string
	ResourceType string
	ResourceID   uint64
	RequestID    string
	IP           string
	UserAgent    string
	Payload      map[string]any
}

type Recorder interface {
	Record(ctx context.Context, event Event) error
}
