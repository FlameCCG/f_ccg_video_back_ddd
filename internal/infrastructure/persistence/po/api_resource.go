package po

import "gorm.io/datatypes"

// APIResource 接口资源.
type APIResource struct {
	Model
	Path    string         `gorm:"size:200;not null;uniqueIndex:idx_api_resource;comment:接口路径" json:"path"`
	Method  string         `gorm:"size:10;not null;uniqueIndex:idx_api_resource;comment:请求方法" json:"method"`
	Summary string         `gorm:"size:200;comment:接口说明" json:"summary"`
	Tags    datatypes.JSON `gorm:"type:json;comment:标签" json:"tags"`
}

// TableName 显式绑定表名 api_resources，保持迁移与仓储 SQL 使用同一张表。
func (APIResource) TableName() string {
	return "api_resources"
}
