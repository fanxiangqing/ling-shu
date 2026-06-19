package model

type EmbedApp struct {
	BaseModel
	TenantID           uint64  `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID          uint64  `gorm:"column:project_id;not null" json:"project_id"`
	AppID              string  `gorm:"column:app_id;size:64;not null;uniqueIndex:uk_embed_apps_app_id" json:"app_id"`
	Name               string  `gorm:"column:name;size:128;not null" json:"name"`
	SecretHash         string  `gorm:"column:secret_hash;size:128;not null" json:"-"`
	SecretCiphertext   string  `gorm:"column:secret_ciphertext;type:text" json:"-"`
	AllowedOriginsJSON *string `gorm:"column:allowed_origins_json;type:json" json:"allowed_origins_json,omitempty"`
	SessionPolicy      string  `gorm:"column:session_policy;size:32;not null;default:context" json:"session_policy"`
	LauncherTitle      string  `gorm:"column:launcher_title;size:64;not null;default:智能问数" json:"launcher_title"`
	WelcomeMessage     string  `gorm:"column:welcome_message;size:255" json:"welcome_message,omitempty"`
	Status             string  `gorm:"column:status;size:32;not null;default:active" json:"status"`
	CreatedBy          uint64  `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (EmbedApp) TableName() string {
	return "embed_apps"
}

type EmbedSession struct {
	BaseModel
	TenantID         uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID        uint64 `gorm:"column:project_id;not null" json:"project_id"`
	EmbedAppID       uint64 `gorm:"column:embed_app_id;not null" json:"embed_app_id"`
	AppID            string `gorm:"column:app_id;size:64;not null" json:"app_id"`
	ExternalUserID   string `gorm:"column:external_user_id;size:191;not null" json:"external_user_id"`
	ExternalUserName string `gorm:"column:external_user_name;size:128" json:"external_user_name,omitempty"`
	SessionKey       string `gorm:"column:session_key;size:191;not null" json:"session_key"`
	ChatSessionID    uint64 `gorm:"column:chat_session_id;not null" json:"chat_session_id"`
	UserID           uint64 `gorm:"column:user_id;not null" json:"user_id"`
	Status           string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (EmbedSession) TableName() string {
	return "embed_sessions"
}
