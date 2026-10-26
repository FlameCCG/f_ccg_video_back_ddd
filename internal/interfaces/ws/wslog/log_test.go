package wslog

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/chat"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMessageBoundaryPreservesConnectionAndDistinctMessageIDs(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	lg := zap.New(core)
	ctx := Connection(logmeta.With(context.Background(), logmeta.Values{RequestID: "request"}), "7")
	Run(ctx, lg, "chat.send", func(context.Context) error { return chat.ErrDailySendLimit })
	Run(ctx, lg, "chat.send", func(context.Context) error { return errors.New("database unavailable") })

	summaries := logs.FilterMessage("websocket.message").All()
	if len(summaries) != 2 || logs.FilterLevelExact(zap.ErrorLevel).Len() != 1 {
		t.Fatalf("wrong message boundaries: %#v", logs.All())
	}

	a, b := summaries[0].ContextMap(), summaries[1].ContextMap()
	if a["request_id"] != "request" || a["user_id"] != "7" || a["connection_id"] != b["connection_id"] || a["message_id"] == b["message_id"] || a["message_id"] == nil {
		t.Fatalf("bad association: %#v %#v", a, b)
	}

	Run(ctx, lg, "chat.send", func(context.Context) error { panic("boom") })

	if logs.FilterLevelExact(zap.ErrorLevel).Len() != 2 {
		t.Fatal("message panic was not recorded")
	}
}
