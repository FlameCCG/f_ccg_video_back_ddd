package po

import "gorm.io/datatypes"

// ApiResource 接口资源.
type ApiResource struct {
	Model
	Path    string         `gorm:"size:200;not null;uniqueIndex:idx_api_resource;comment:接口路径" json:"path"`
	Method  string         `gorm:"size:10;not null;uniqueIndex:idx_api_resource;comment:请求方法" json:"method"`
	Summary string         `gorm:"size:200;comment:接口说明" json:"summary"`
	Tags    datatypes.JSON `gorm:"type:json;comment:标签" json:"tags"`
}

// TableName 方法.
func (ApiResource) TableName() string {
	return "api_resources"
}
