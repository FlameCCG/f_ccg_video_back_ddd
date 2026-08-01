package media

import (
	"context"
	"errors"
	"testing"
)

func TestTaskRegistryCancelVideosCancelsOnlyMatchingRuns(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()

	videoCtx, unregisterVideo := registry.Register(context.Background(), TaskKey{VideoID: 10})
	defer unregisterVideo()

	partCtx, unregisterPart := registry.Register(context.Background(), TaskKey{VideoID: 10, PartID: 2})
	defer unregisterPart()

	otherCtx, unregisterOther := registry.Register(context.Background(), TaskKey{VideoID: 11})
	defer unregisterOther()

	if got := registry.ActiveCount(); got != 3 {
		t.Fatalf("ActiveCount() = %d, want 3", got)
	}

	if got := registry.CancelVideos([]uint{10, 10, 0}); got != 2 {
		t.Fatalf("CancelVideos() = %d, want 2", got)
	}

	for name, ctx := range map[string]context.Context{"video": videoCtx, "part": partCtx} {
		if !errors.Is(context.Cause(ctx), ErrVideoDeleted) {
			t.Fatalf("%s cause = %v, want ErrVideoDeleted", name, context.Cause(ctx))
		}
	}

	select {
	case <-otherCtx.Done():
		t.Fatalf("unrelated task canceled with cause %v", context.Cause(otherCtx))
	default:
	}
}

// TestTaskRegistryUnregisterIsIdempotent 覆盖注销函数的幂等性与登记表清理。
func TestTaskRegistryUnregisterIsIdempotent(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()
	ctx, unregister := registry.Register(context.Background(), TaskKey{VideoID: 5})

	unregister()
	unregister()

	if got := registry.ActiveCount(); got != 0 {
		t.Fatalf("ActiveCount() after unregister = %d, want 0", got)
	}

	if !errors.Is(context.Cause(ctx), context.Canceled) {
		t.Fatalf("cause after unregister = %v, want context.Canceled", context.Cause(ctx))
	}
}

// TestTaskRegisterWithoutVideoIDIsNotTracked videoID 为 0 时不进登记表，但仍返回可取消 ctx。
func TestTaskRegisterWithoutVideoIDIsNotTracked(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()

	ctx, unregister := registry.Register(context.Background(), TaskKey{})
	defer unregister()

	if got := registry.ActiveCount(); got != 0 {
		t.Fatalf("ActiveCount() = %d, want 0 for zero videoID", got)
	}

	if registry.CancelVideos([]uint{0}) != 0 {
		t.Fatal("CancelVideos() must ignore zero video IDs")
	}

	unregister()

	if !errors.Is(context.Cause(ctx), context.Canceled) {
		t.Fatalf("cause = %v, want context.Canceled", context.Cause(ctx))
	}
}

func TestNormalizeVideoIDs(t *testing.T) {
	t.Parallel()

	got := NormalizeVideoIDs([]uint{7, 0, 3, 7, 3, 0})
	want := []uint{3, 7}

	if len(got) != len(want) {
		t.Fatalf("NormalizeVideoIDs() = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeVideoIDs() = %v, want %v", got, want)
		}
	}

	if ids := NormalizeVideoIDs(nil); len(ids) != 0 {
		t.Fatalf("NormalizeVideoIDs(nil) = %v, want empty", ids)
	}
}

// TestClassifyInterruptReason 覆盖中断原因分类（原 classifyTranscodeInterruptReason）。
func TestClassifyInterruptReason(t *testing.T) {
	t.Parallel()

	if got := ClassifyInterruptReason(context.Background(), ErrTranscodeProgressStalled, false); got != "progress_stalled" {
		t.Fatalf("stalled reason = %q", got)
	}

	if got := ClassifyInterruptReason(context.Background(), ErrTranscodeEncodeTimeout, false); got != "encode_timeout" {
		t.Fatalf("encode timeout reason = %q", got)
	}

	if got := ClassifyInterruptReason(context.Background(), ErrTranscodeUploadTimeout, false); got != "upload_timeout" {
		t.Fatalf("upload timeout reason = %q", got)
	}

	if got := ClassifyInterruptReason(context.Background(), ErrVideoDeleted, false); got != "video_deleted" {
		t.Fatalf("video deleted reason = %q", got)
	}

	// Kafka 会话已取消优先于普通 context.Canceled。
	if got := ClassifyInterruptReason(context.Background(), context.Canceled, true); got != "kafka_session_canceled" {
		t.Fatalf("session cancel reason = %q", got)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	if got := ClassifyInterruptReason(canceled, context.Canceled, false); got != "context_canceled" {
		t.Fatalf("context cancel reason = %q", got)
	}

	if got := ClassifyInterruptReason(context.Background(), errors.New("boom"), false); got != "process_error" {
		t.Fatalf("generic reason = %q", got)
	}
}

// TestTaskWasDeleted 覆盖「任务因视频删除而作废」的两条判定路径（err 与 ctx cause）。
func TestTaskWasDeleted(t *testing.T) {
	t.Parallel()

	if !TaskWasDeleted(context.Background(), ErrVideoDeleted) {
		t.Fatal("explicit ErrVideoDeleted must be detected")
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrVideoDeleted)

	if !TaskWasDeleted(ctx, nil) {
		t.Fatal("ctx cause ErrVideoDeleted must be detected")
	}

	if TaskWasDeleted(context.Background(), errors.New("boom")) {
		t.Fatal("unrelated error must not be treated as deletion")
	}
}
