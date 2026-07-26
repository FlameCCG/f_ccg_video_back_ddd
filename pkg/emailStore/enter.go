// Package emailStore provides package functionality.
package emailStore

import "time"

// EmailStoreInfo 类型.
type EmailStoreInfo struct {
	Email     string
	Code      string
	Timestamp time.Time
}
