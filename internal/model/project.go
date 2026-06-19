package model

type Project struct {
	BaseModel
	TenantID    uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_projects_tenant_code" json:"tenant_id"`
	Name        string `gorm:"column:name;size:128;not null" json:"name"`
	Code        string `gorm:"column:code;size:64;not null;uniqueIndex:uk_projects_tenant_code" json:"code"`
	Description string `gorm:"column:description;size:512" json:"description"`
	Status      string `gorm:"column:status;size:32;not null;default:active" json:"status"`
	CreatedBy   uint64 `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (Project) TableName() string {
	return "projects"
}
