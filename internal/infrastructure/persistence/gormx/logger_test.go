package gormx

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"
)

func TestGORMDiagnosticsDoNotEvaluateSQLOrAlertTwice(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	lg := NewZapLogger(zap.New(core), 0)
	ctx := logmeta.With(context.Background(), logmeta.Values{RequestID: "request"})
	query := func() (string, int64) { t.Fatal("SQL and parameters evaluated for logging"); return "", 0 }
	lg.Trace(ctx, time.Now().Add(-250*time.Millisecond), query, nil)
	lg.Trace(ctx, time.Now(), query, errors.New("password=private-sql-value"))
	lg.Trace(ctx, time.Now(), query, gorm.ErrRecordNotFound)

	if logs.Len() != 2 || logs.FilterLevelExact(zap.ErrorLevel).Len() != 0 {
		t.Fatalf("unexpected diagnostics: %#v", logs.All())
	}

	for _, entry := range logs.All() {
		if entry.ContextMap()["request_id"] != "request" {
			t.Fatal("missing database correlation")
		}
	}
}
