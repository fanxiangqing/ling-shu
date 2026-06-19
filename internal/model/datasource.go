package model

import "time"

type Datasource struct {
	BaseModel
	TenantID       uint64     `gorm:"column:tenant_id;not null;uniqueIndex:uk_datasources_tenant_name" json:"tenant_id"`
	ProjectID      uint64     `gorm:"column:project_id;not null;default:0" json:"project_id"`
	Name           string     `gorm:"column:name;size:128;not null;uniqueIndex:uk_datasources_tenant_name" json:"name"`
	DBType         string     `gorm:"column:db_type;size:64;not null" json:"db_type"`
	DSNCiphertext  string     `gorm:"column:dsn_ciphertext;type:text;not null" json:"-"`
	ConfigJSON     *string    `gorm:"column:config_json;type:json" json:"config_json,omitempty"`
	Status         string     `gorm:"column:status;size:32;not null;default:active" json:"status"`
	LastSyncStatus string     `gorm:"column:last_sync_status;size:32" json:"last_sync_status,omitempty"`
	LastSyncAt     *time.Time `gorm:"column:last_sync_at" json:"last_sync_at,omitempty"`
	CreatedBy      uint64     `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (Datasource) TableName() string {
	return "datasources"
}

type ProjectDatasource struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;not null;uniqueIndex:uk_project_datasources_project_ds" json:"tenant_id"`
	ProjectID    uint64    `gorm:"column:project_id;not null;uniqueIndex:uk_project_datasources_project_ds" json:"project_id"`
	DatasourceID uint64    `gorm:"column:datasource_id;not null;uniqueIndex:uk_project_datasources_project_ds" json:"datasource_id"`
	CreatedBy    uint64    `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ProjectDatasource) TableName() string {
	return "project_datasources"
}

type MetadataSchema struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID    uint64    `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID uint64    `gorm:"column:datasource_id;not null" json:"datasource_id"`
	SchemaName   string    `gorm:"column:schema_name;size:191;not null" json:"schema_name"`
	CommentText  string    `gorm:"column:comment_text;size:1024" json:"comment_text,omitempty"`
	SyncedAt     time.Time `gorm:"column:synced_at" json:"synced_at"`
}

func (MetadataSchema) TableName() string {
	return "metadata_schemas"
}

type MetadataTable struct {
	ID                  uint64               `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID            uint64               `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID           uint64               `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID        uint64               `gorm:"column:datasource_id;not null" json:"datasource_id"`
	SchemaName          string               `gorm:"column:schema_name;size:191;not null" json:"schema_name"`
	Name                string               `gorm:"column:table_name;size:191;not null" json:"table_name"`
	TableType           string               `gorm:"column:table_type;size:32;not null;default:table" json:"table_type"`
	CommentText         string               `gorm:"column:comment_text;size:1024" json:"comment_text,omitempty"`
	OriginalCommentText string               `gorm:"column:original_comment_text;size:1024" json:"original_comment_text,omitempty"`
	BusinessCommentText string               `gorm:"column:business_comment_text;size:1024" json:"business_comment_text,omitempty"`
	RowCount            *int64               `gorm:"column:row_count" json:"row_count,omitempty"`
	SyncedAt            time.Time            `gorm:"column:synced_at" json:"synced_at"`
	Columns             []MetadataColumn     `gorm:"foreignKey:TableID" json:"columns,omitempty"`
	Indexes             []MetadataIndex      `gorm:"foreignKey:TableID" json:"indexes,omitempty"`
	ForeignKeys         []MetadataForeignKey `gorm:"foreignKey:TableID" json:"foreign_keys,omitempty"`
}

func (MetadataTable) TableName() string {
	return "metadata_tables"
}

type MetadataColumn struct {
	ID                  uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID            uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID           uint64    `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID        uint64    `gorm:"column:datasource_id;not null" json:"datasource_id"`
	TableID             uint64    `gorm:"column:table_id;not null" json:"table_id"`
	ColumnName          string    `gorm:"column:column_name;size:191;not null" json:"column_name"`
	OrdinalPosition     int       `gorm:"column:ordinal_position;not null" json:"ordinal_position"`
	DataType            string    `gorm:"column:data_type;size:128;not null" json:"data_type"`
	ColumnType          string    `gorm:"column:column_type;size:255" json:"column_type,omitempty"`
	Nullable            bool      `gorm:"column:nullable;not null" json:"nullable"`
	DefaultValue        string    `gorm:"column:default_value;size:512" json:"default_value,omitempty"`
	IsPrimaryKey        bool      `gorm:"column:is_primary_key;not null;default:false" json:"is_primary_key"`
	IsForeignKey        bool      `gorm:"column:is_foreign_key;not null;default:false" json:"is_foreign_key"`
	CommentText         string    `gorm:"column:comment_text;size:1024" json:"comment_text,omitempty"`
	OriginalCommentText string    `gorm:"column:original_comment_text;size:1024" json:"original_comment_text,omitempty"`
	BusinessCommentText string    `gorm:"column:business_comment_text;size:1024" json:"business_comment_text,omitempty"`
	SyncedAt            time.Time `gorm:"column:synced_at" json:"synced_at"`
}

func (MetadataColumn) TableName() string {
	return "metadata_columns"
}

type MetadataIndex struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID    uint64    `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID uint64    `gorm:"column:datasource_id;not null" json:"datasource_id"`
	TableID      uint64    `gorm:"column:table_id;not null" json:"table_id"`
	IndexName    string    `gorm:"column:index_name;size:191;not null" json:"index_name"`
	IndexType    string    `gorm:"column:index_type;size:64" json:"index_type,omitempty"`
	UniqueIndex  bool      `gorm:"column:unique_index;not null;default:false" json:"unique_index"`
	ColumnsJSON  string    `gorm:"column:columns_json;type:json;not null" json:"columns_json"`
	SyncedAt     time.Time `gorm:"column:synced_at" json:"synced_at"`
}

func (MetadataIndex) TableName() string {
	return "metadata_indexes"
}

type MetadataForeignKey struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID         uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID        uint64    `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID     uint64    `gorm:"column:datasource_id;not null" json:"datasource_id"`
	TableID          uint64    `gorm:"column:table_id;not null" json:"table_id"`
	ConstraintName   string    `gorm:"column:constraint_name;size:191;not null" json:"constraint_name"`
	ColumnName       string    `gorm:"column:column_name;size:191;not null" json:"column_name"`
	ReferencedSchema string    `gorm:"column:referenced_schema;size:191" json:"referenced_schema,omitempty"`
	ReferencedTable  string    `gorm:"column:referenced_table;size:191;not null" json:"referenced_table"`
	ReferencedColumn string    `gorm:"column:referenced_column;size:191;not null" json:"referenced_column"`
	SyncedAt         time.Time `gorm:"column:synced_at" json:"synced_at"`
}

func (MetadataForeignKey) TableName() string {
	return "metadata_foreign_keys"
}

type MetadataSyncJob struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID     uint64     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID    uint64     `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID uint64     `gorm:"column:datasource_id;not null" json:"datasource_id"`
	TriggerType  string     `gorm:"column:trigger_type;size:32;not null" json:"trigger_type"`
	Status       string     `gorm:"column:status;size:32;not null;default:pending" json:"status"`
	ErrorMessage string     `gorm:"column:error_message;type:text" json:"error_message,omitempty"`
	StartedAt    *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	CreatedBy    uint64     `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (MetadataSyncJob) TableName() string {
	return "metadata_sync_jobs"
}
