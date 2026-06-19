package model

type SensitiveTable struct {
	BaseModel
	TenantID     uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID    uint64 `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID uint64 `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	SchemaName   string `gorm:"column:schema_name;size:191" json:"schema_name,omitempty"`
	Table        string `gorm:"column:table_name;size:191;not null" json:"table_name"`
	RiskLevel    string `gorm:"column:risk_level;size:32;not null;default:high" json:"risk_level"`
	Enabled      bool   `gorm:"column:enabled;not null;default:true" json:"enabled"`
}

func (SensitiveTable) TableName() string {
	return "sensitive_tables"
}

type SensitiveColumn struct {
	BaseModel
	TenantID     uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID    uint64 `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID uint64 `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	SchemaName   string `gorm:"column:schema_name;size:191" json:"schema_name,omitempty"`
	Table        string `gorm:"column:table_name;size:191;not null" json:"table_name"`
	ColumnName   string `gorm:"column:column_name;size:191;not null" json:"column_name"`
	MaskType     string `gorm:"column:mask_type;size:64;not null;default:redact" json:"mask_type"`
	RiskLevel    string `gorm:"column:risk_level;size:32;not null;default:medium" json:"risk_level"`
	Enabled      bool   `gorm:"column:enabled;not null;default:true" json:"enabled"`
}

func (SensitiveColumn) TableName() string {
	return "sensitive_columns"
}
