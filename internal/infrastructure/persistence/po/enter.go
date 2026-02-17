// Package po provides package functionality.
package po

import "time"

// Model 类型.
type Model struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}
