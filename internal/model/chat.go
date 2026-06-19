package model

import "time"

type ChatSession struct {
	BaseModel
	TenantID  uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID uint64 `gorm:"column:project_id;not null" json:"project_id"`
	UserID    uint64 `gorm:"column:user_id;not null" json:"user_id"`
	Title     string `gorm:"column:title;size:255;not null;default:新会话" json:"title"`
	Status    string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (ChatSession) TableName() string {
	return "chat_sessions"
}

type ChatMessage struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID         uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID        uint64    `gorm:"column:project_id;not null" json:"project_id"`
	SessionID        uint64    `gorm:"column:session_id;not null" json:"session_id"`
	UserID           uint64    `gorm:"column:user_id" json:"user_id,omitempty"`
	Role             string    `gorm:"column:role;size:32;not null" json:"role"`
	Content          string    `gorm:"column:content;type:mediumtext;not null" json:"content"`
	ContentType      string    `gorm:"column:content_type;size:32;not null;default:text" json:"content_type"`
	QueryExecutionID uint64    `gorm:"column:query_execution_id" json:"query_execution_id,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}
