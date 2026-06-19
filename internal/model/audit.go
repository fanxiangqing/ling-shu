package model

import "time"

type AuditLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id" json:"tenant_id,omitempty"`
	ProjectID    uint64    `gorm:"column:project_id" json:"project_id,omitempty"`
	UserID       uint64    `gorm:"column:user_id" json:"user_id,omitempty"`
	EventType    string    `gorm:"column:event_type;size:128;not null" json:"event_type"`
	ResourceType string    `gorm:"column:resource_type;size:64" json:"resource_type,omitempty"`
	ResourceID   uint64    `gorm:"column:resource_id" json:"resource_id,omitempty"`
	RequestID    string    `gorm:"column:request_id;size:128" json:"request_id,omitempty"`
	IP           string    `gorm:"column:ip;size:64" json:"ip,omitempty"`
	UserAgent    string    `gorm:"column:user_agent;size:512" json:"user_agent,omitempty"`
	PayloadJSON  *string   `gorm:"column:payload_json;type:json" json:"payload_json,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
