package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCalcPipelineEncodeTimeout(t *testing.T) {
	t.Parallel()

	// 未知时长 → 兜底时长 * 倍率，且不低于下限。
	if got := CalcPipelineEncodeTimeout(0, 4, false, 0); got < EncodeTimeoutMin {
		t.Fatalf("zero duration cpu: %v < min %v", got, EncodeTimeoutMin)
	}

	// 23 分钟片源、4 路、CPU：1380s * mult(~24) 约数小时，应在上下限内。
	got := CalcPipelineEncodeTimeout(23*60, 4, false, 1024*1024*1024)
	if got < EncodeTimeoutMin || got > EncodeTimeoutMax {
		t.Fatalf("23m 4k cpu: got %v, want in [%v,%v]", got, EncodeTimeoutMin, EncodeTimeoutMax)
	}

	gpu := CalcPipelineEncodeTimeout(23*60, 4, true, 1024*1024*1024)
	if gpu >= got {
		t.Fatalf("gpu timeout %v should be < cpu %v", gpu, got)
	}

	// 同时长下，更大源文件应给出更多预算。
	small := CalcPipelineEncodeTimeout(600, 2, false, 0)

	large := CalcPipelineEncodeTimeout(600, 2, false, 2*1024*1024*1024)
	if large <= small {
		t.Fatalf("larger source should increase timeout: small=%v large=%v", small, large)
	}
}

func TestCalcProgressStallTimeout(t *testing.T) {
	t.Parallel()

	if got := CalcProgressStallTimeout(0); got != ProgressStallMin {
		t.Fatalf("zero duration: got %v want %v", got, ProgressStallMin)
	}
	// 很长片：duration/3 很大 → 钳到上限。
	if got := CalcProgressStallTimeout(6 * 3600); got != ProgressStallMax {
		t.Fatalf("long media: got %v want %v", got, ProgressStallMax)
	}
	// 25 分钟片源 → duration/3 ≈ 8 分钟 → 抬到下限 20 分钟。
	if got := CalcProgressStallTimeout(25 * 60); got != ProgressStallMin {
		t.Fatalf("25m media stall: got %v want min %v", got, ProgressStallMin)
	}
}

func TestCalcUploadTimeout(t *testing.T) {
	t.Parallel()

	if got := CalcUploadTimeout(0); got != 30*time.Minute {
		t.Fatalf("unknown size: got %v", got)
	}
	// 10MiB 应钳到下限。
	if got := CalcUploadTimeout(10 * 1024 * 1024); got != UploadTimeoutMin {
		t.Fatalf("small file: got %v want %v", got, UploadTimeoutMin)
	}
	// 约 1GiB @ 512KiB/s ≈ 2048s + base → 数分钟，低于上限。
	got := CalcUploadTimeout(1024 * 1024 * 1024)
	if got < UploadTimeoutMin || got > UploadTimeoutMax {
		t.Fatalf("1GiB upload: got %v out of range", got)
	}
}

func TestWithProgressStallGuardCancels(t *testing.T) {
	t.Parallel()

	ctx, cancel, wrap := WithProgressStallGuard(context.Background(), 80*time.Millisecond, nil)
	defer cancel()

	// 从不回调进度 → 应因停滞取消。
	_ = wrap(nil)

	deadline := time.After(2 * time.Second)

	for {
		select {
		case <-ctx.Done():
			if !errors.Is(context.Cause(ctx), ErrTranscodeProgressStalled) {
				t.Fatalf("cause = %v, want %v", context.Cause(ctx), ErrTranscodeProgressStalled)
			}

			return
		case <-deadline:
			t.Fatal("stall guard did not cancel in time")
		}
	}
}

func TestWithProgressStallGuardKeepsAliveWithProgress(t *testing.T) {
	t.Parallel()

	ctx, cancel, wrap := WithProgressStallGuard(context.Background(), 200*time.Millisecond, nil)
	defer cancel()

	cb := wrap(func(float64) {})
	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := range 6 {
			cb(float64(i))
			time.Sleep(60 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		// 进度持续推进时不应取消
	case <-ctx.Done():
		t.Fatalf("should not stall while progress advances: %v", context.Cause(ctx))
	case <-time.After(2 * time.Second):
		t.Fatal("progress loop hung")
	}

	// 停止推进后应判定停滞。
	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), ErrTranscodeProgressStalled) {
			t.Fatalf("cause = %v", context.Cause(ctx))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected stall after silence")
	}
}

// TestWithProgressStallGuardNotifiesObserver 覆盖新增的 onStall 观察者回调
// （原实现在此处直接打 zap 日志，新架构改为注入回调，语义等价）。
func TestWithProgressStallGuardNotifiesObserver(t *testing.T) {
	t.Parallel()

	observed := make(chan time.Duration, 1)

	ctx, cancel, wrap := WithProgressStallGuard(
		context.Background(),
		80*time.Millisecond,
		func(_, limit time.Duration) { observed <- limit },
	)
	defer cancel()

	_ = wrap(nil)

	select {
	case limit := <-observed:
		if limit != 80*time.Millisecond {
			t.Fatalf("observer limit = %v, want 80ms", limit)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stall observer was not invoked")
	}

	<-ctx.Done()
}

func TestClassifyFFmpegCtxError(t *testing.T) {
	t.Parallel()

	if err := ClassifyFFmpegCtxError(context.Background(), nil, time.Minute, ""); err != nil {
		t.Fatalf("nil wait: %v", err)
	}

	stallCtx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrTranscodeProgressStalled)

	err := ClassifyFFmpegCtxError(stallCtx, errors.New("signal: killed"), time.Minute, "stderr")
	if !errors.Is(err, ErrTranscodeProgressStalled) {
		t.Fatalf("stall classify: %v", err)
	}

	toCtx, toCancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer toCancel()

	time.Sleep(2 * time.Millisecond)

	err = ClassifyFFmpegCtxError(toCtx, errors.New("signal: killed"), 15*time.Minute, "x")
	if !errors.Is(err, ErrTranscodeEncodeTimeout) {
		t.Fatalf("timeout classify: %v", err)
	}
}

func TestCalcAssembleAndAudioTimeouts(t *testing.T) {
	t.Parallel()

	if got := CalcAssembleTimeout(0); got != AssembleTimeoutMin {
		t.Fatalf("zero bytes assemble timeout = %v, want min %v", got, AssembleTimeoutMin)
	}

	if got := CalcAssembleTimeout(100 * 1024 * 1024 * 1024); got != AssembleTimeoutMax {
		t.Fatalf("huge assemble timeout = %v, want max %v", got, AssembleTimeoutMax)
	}

	if got := CalcAssembleTimeout(10 * 1024 * 1024 * 1024); got <= AssembleTimeoutMin || got >= AssembleTimeoutMax {
		t.Fatalf("mid-size assemble timeout out of range: %v", got)
	}

	if got := CalcAudioEncodeTimeout(0); got != AudioEncodeTimeoutMin {
		t.Fatalf("default audio timeout = %v, want min %v", got, AudioEncodeTimeoutMin)
	}

	if got := CalcAudioEncodeTimeout(100000); got != AudioEncodeTimeoutMax {
		t.Fatalf("huge audio timeout = %v, want max %v", got, AudioEncodeTimeoutMax)
	}

	if got := CalcAudioEncodeTimeout(7200); got != time.Duration(7200/5)*time.Second+2*time.Minute {
		t.Fatalf("2h audio timeout = %v", got)
	}
}
