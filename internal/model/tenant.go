package model

type Tenant struct {
	BaseModel
	Name   string `gorm:"column:name;size:128;not null" json:"name"`
	Code   string `gorm:"column:code;size:64;not null;uniqueIndex:uk_tenants_code" json:"code"`
	Status string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (Tenant) TableName() string {
	return "tenants"
}
