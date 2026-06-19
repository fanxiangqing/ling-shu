package model

import "time"

type KBTerm struct {
	BaseModel
	TenantID    uint64  `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID   uint64  `gorm:"column:project_id;not null" json:"project_id"`
	Term        string  `gorm:"column:term;size:191;not null" json:"term"`
	AliasesJSON *string `gorm:"column:aliases_json;type:json" json:"aliases_json,omitempty"`
	Definition  string  `gorm:"column:definition;type:text;not null" json:"definition"`
	Enabled     bool    `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedBy   uint64  `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (KBTerm) TableName() string {
	return "kb_terms"
}

type KBMetric struct {
	BaseModel
	TenantID          uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID         uint64 `gorm:"column:project_id;not null" json:"project_id"`
	Name              string `gorm:"column:name;size:191;not null" json:"name"`
	Description       string `gorm:"column:description;type:text;not null" json:"description"`
	Formula           string `gorm:"column:formula;type:text;not null" json:"formula"`
	DatasourceID      uint64 `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	DefaultTimeColumn string `gorm:"column:default_time_column;size:191" json:"default_time_column,omitempty"`
	Enabled           bool   `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedBy         uint64 `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (KBMetric) TableName() string {
	return "kb_metrics"
}

type KBFewShotSQL struct {
	BaseModel
	TenantID     uint64 `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID    uint64 `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID uint64 `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	Question     string `gorm:"column:question;type:text;not null" json:"question"`
	SQLText      string `gorm:"column:sql_text;type:text;not null" json:"sql_text"`
	Explanation  string `gorm:"column:explanation;type:text" json:"explanation,omitempty"`
	Enabled      bool   `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedBy    uint64 `gorm:"column:created_by" json:"created_by,omitempty"`
}

func (KBFewShotSQL) TableName() string {
	return "kb_fewshot_sql"
}

type KBChunk struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID          uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID         uint64    `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID      uint64    `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	KBType            string    `gorm:"column:kb_type;size:64;not null" json:"kb_type"`
	RefID             uint64    `gorm:"column:ref_id;not null" json:"ref_id"`
	ChunkNo           int       `gorm:"column:chunk_no;not null;default:0" json:"chunk_no"`
	ChunkText         string    `gorm:"column:chunk_text;type:text;not null" json:"chunk_text"`
	EmbeddingProvider string    `gorm:"column:embedding_provider;size:64" json:"embedding_provider,omitempty"`
	EmbeddingModel    string    `gorm:"column:embedding_model;size:128" json:"embedding_model,omitempty"`
	VectorCollection  string    `gorm:"column:vector_collection;size:128" json:"vector_collection,omitempty"`
	VectorID          string    `gorm:"column:vector_id;size:128" json:"vector_id,omitempty"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"created_at"`
}

func (KBChunk) TableName() string {
	return "kb_chunks"
}
