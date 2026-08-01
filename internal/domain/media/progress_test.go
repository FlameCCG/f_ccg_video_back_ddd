package media

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeProgressStore 记录写入的进度快照，替代原测试对 global.RedisClient 的依赖。
type fakeProgressStore struct {
	mu        sync.Mutex
	snapshots []TranscodeProgress
	ttls      []time.Duration
	cleared   [][2]uint
}

func (s *fakeProgressStore) Write(_ context.Context, snapshot *TranscodeProgress, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots = append(s.snapshots, *snapshot)
	s.ttls = append(s.ttls, ttl)

	return nil
}

func (s *fakeProgressStore) Clear(_ context.Context, videoID, partID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleared = append(s.cleared, [2]uint{videoID, partID})
}

// last 返回最后一次写入的快照。
func (s *fakeProgressStore) last() (TranscodeProgress, time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.snapshots) == 0 {
		return TranscodeProgress{}, 0, false
	}

	return s.snapshots[len(s.snapshots)-1], s.ttls[len(s.ttls)-1], true
}

func TestPipelineProgressUpdatesSharedPercent(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}
	tracker := NewTranscodeProgressTracker(1, 0, store, nil)
	tracker.SetDuration(100)
	tracker.ConfigurePipeline(context.Background(), TranscodePipelineMP4, "queued", []uint16{1080, 720})
	tracker.StartPipeline(context.Background(), TranscodePipelineMP4, "encoding")
	tracker.UpdatePipelineProgress(context.Background(), TranscodePipelineMP4, 42)

	// UpdatePipelineProgress 走非强制 flush，受 500ms/1% 节流限制不会立刻落库；
	// 因此进度值从 tracker 自身读取（原测试直接读 tracker.pipelines 私有字段，等价）。
	percent, _, status, outTime := tracker.DiagnosticSnapshot()
	if percent != 42 {
		t.Fatalf("pipeline percent = %v, want 42", percent)
	}

	if status != TranscodeStatusRunning {
		t.Fatalf("status = %q, want running", status)
	}

	if outTime != 42 {
		t.Fatalf("outTimeSec = %v, want 42", outTime)
	}

	// 管线登记（ConfigurePipeline / StartPipeline）走强制 flush，快照里应带目标分辨率。
	snapshot, _, ok := store.last()
	if !ok {
		t.Fatal("no progress snapshot written")
	}

	if len(snapshot.Pipelines) != 1 {
		t.Fatalf("pipelines = %#v, want exactly one", snapshot.Pipelines)
	}

	pipeline := snapshot.Pipelines[0]
	if len(pipeline.Resolutions) != 2 || pipeline.Resolutions[0] != 1080 || pipeline.Resolutions[1] != 720 {
		t.Fatalf("resolutions = %#v, want [1080 720]", pipeline.Resolutions)
	}
}

// TestTrackerSucceedAndTTL 覆盖终态快照与 TTL 选择（运行中 2h / 终态 24h）。
func TestTrackerSucceedAndTTL(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}
	tracker := NewTranscodeProgressTracker(3, 7, store, nil)

	tracker.EnterStage(context.Background(), TranscodeStagePrepare, "准备源文件")

	running, ttl, ok := store.last()
	if !ok {
		t.Fatal("no running snapshot")
	}

	if running.Status != TranscodeStatusRunning {
		t.Fatalf("status = %q, want running", running.Status)
	}

	if ttl != TranscodeProgressRunningTTL {
		t.Fatalf("running ttl = %v, want %v", ttl, TranscodeProgressRunningTTL)
	}

	tracker.Succeed(context.Background())

	done, doneTTL, _ := store.last()
	if done.Status != TranscodeStatusSucceeded || done.Percent != 100 {
		t.Fatalf("succeed snapshot = %+v, want succeeded/100", done)
	}

	if doneTTL != TranscodeProgressFinalTTL {
		t.Fatalf("final ttl = %v, want %v", doneTTL, TranscodeProgressFinalTTL)
	}

	if done.VideoID != 3 || done.PartID != 7 {
		t.Fatalf("identity = (%d,%d), want (3,7)", done.VideoID, done.PartID)
	}
}

// TestTrackerFailInvokesObserver 覆盖失败观察者回调（原实现在 Fail 内直接打 zap 日志）。
func TestTrackerFailInvokesObserver(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}

	var observed TranscodeProgress

	tracker := NewTranscodeProgressTracker(9, 0, store, func(snapshot TranscodeProgress) {
		observed = snapshot
	})

	tracker.EnterStage(context.Background(), TranscodeStageMultiRes, "执行并行转码 Pipeline")
	tracker.Fail(context.Background(), "ffmpeg pipeline failed: signal: killed")

	if observed.Status != TranscodeStatusFailed {
		t.Fatalf("observed status = %q, want failed", observed.Status)
	}

	if observed.Error == "" {
		t.Fatal("observed snapshot must carry error text")
	}

	failed, ttl, _ := store.last()
	if failed.Status != TranscodeStatusFailed || failed.Message != "转码失败" {
		t.Fatalf("failed snapshot = %+v", failed)
	}

	if ttl != TranscodeProgressFinalTTL {
		t.Fatalf("failed ttl = %v, want final %v", ttl, TranscodeProgressFinalTTL)
	}
}

// TestTrackerSkipStageRenormalizesWeights 跳过阶段后剩余阶段权重重新归一化。
func TestTrackerSkipStageRenormalizesWeights(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}
	tracker := NewTranscodeProgressTracker(1, 0, store, nil)

	tracker.SkipStage(TranscodeStageDash)
	tracker.SkipStage(TranscodeStageNormalize)
	tracker.EnterStage(context.Background(), TranscodeStagePrepare, "准备源文件")
	tracker.CompleteStage(context.Background(), TranscodeStagePrepare)

	snapshot, _, ok := store.last()
	if !ok {
		t.Fatal("no snapshot")
	}

	// 剩余权重 = 100 - 15(dash) - 20(normalize) = 65；prepare 权重 5 → 5/65 ≈ 7.69%
	if snapshot.Percent < 7.5 || snapshot.Percent > 7.9 {
		t.Fatalf("percent after skips = %v, want ≈7.69", snapshot.Percent)
	}
}

// TestTrackerSkipStageIfUnusedKeepsRunningStage 已进入的阶段不会被 SkipStageIfUnused 跳过。
func TestTrackerSkipStageIfUnusedKeepsRunningStage(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}
	tracker := NewTranscodeProgressTracker(1, 0, store, nil)
	tracker.EnterStage(context.Background(), TranscodeStageMultiRes, "执行并行转码 Pipeline")

	before, _, _ := store.last()

	tracker.SkipStageIfUnused(TranscodeStageMultiRes)
	tracker.EnterStage(context.Background(), TranscodeStageCleanup, "清理与刷新缓存")

	after, _, _ := store.last()
	if after.Percent <= before.Percent {
		t.Fatalf("percent must advance: before=%v after=%v", before.Percent, after.Percent)
	}
}

// TestTrackerNilStoreIsSafe store 为 nil 时只在内存累计（与原 RedisClient 缺失行为一致）。
func TestTrackerNilStoreIsSafe(t *testing.T) {
	t.Parallel()

	tracker := NewTranscodeProgressTracker(1, 0, nil, nil)
	tracker.SetDuration(100)
	tracker.EnterStage(context.Background(), TranscodeStagePrepare, "准备源文件")
	tracker.UpdateFFmpegProgress(context.Background(), 50)
	tracker.Succeed(context.Background())

	percent, stage, status, _ := tracker.DiagnosticSnapshot()
	if status != TranscodeStatusSucceeded || percent != 100 || stage != TranscodeStageCleanup {
		t.Fatalf("diagnostic = (%v,%q,%q)", percent, stage, status)
	}
}

func TestAggregateProgressPercent(t *testing.T) {
	t.Parallel()

	pipelines := []TranscodePipelineProgress{
		{Name: TranscodePipelineDASH, Percent: 98.958325},
		{Name: TranscodePipelineMP4, Percent: 100},
	}

	if got := aggregateProgressPercent(TranscodeStatusSucceeded, 100, pipelines); got != 100 {
		t.Fatalf("succeeded aggregate = %v, want 100", got)
	}

	wantRunning := (98.958325 + 100) / 2
	if got := aggregateProgressPercent(TranscodeStatusRunning, 80, pipelines); got != wantRunning {
		t.Fatalf("running aggregate = %v, want %v", got, wantRunning)
	}

	if got := aggregateProgressPercent(TranscodeStatusRunning, 42, nil); got != 42 {
		t.Fatalf("aggregate without pipelines = %v, want 42", got)
	}
}

// TestNilTrackerMethodsAreSafe nil tracker 的全部方法必须安全（消费者在早失败路径上会传 nil）。
func TestNilTrackerMethodsAreSafe(t *testing.T) {
	t.Parallel()

	var tracker *TranscodeProgressTracker

	tracker.SetDuration(10)
	tracker.EnterStage(context.Background(), TranscodeStagePrepare, "x")
	tracker.CompleteStage(context.Background(), TranscodeStagePrepare)
	tracker.UpdateFFmpegProgress(context.Background(), 1)
	tracker.ConfigurePipeline(context.Background(), TranscodePipelineMP4, "x", nil)
	tracker.StartPipeline(context.Background(), TranscodePipelineMP4, "x")
	tracker.UpdatePipelineProgress(context.Background(), TranscodePipelineMP4, 1)
	tracker.MarkPipelineUploading(context.Background(), TranscodePipelineMP4, "x")
	tracker.CompletePipeline(context.Background(), TranscodePipelineMP4, nil)
	tracker.SkipPipeline(context.Background(), TranscodePipelineMP4, "x")
	tracker.SkipStage(TranscodeStageDash)
	tracker.SkipStageIfUnused(TranscodeStageDash)
	tracker.Succeed(context.Background())
	tracker.Fail(context.Background(), "x")

	percent, stage, status, outTime := tracker.DiagnosticSnapshot()
	if percent != 0 || stage != "" || status != "" || outTime != 0 {
		t.Fatalf("nil tracker diagnostic = (%v,%q,%q,%v)", percent, stage, status, outTime)
	}
}

// TestContextProgressTracker 覆盖 tracker 在 context 中的存取。
func TestContextProgressTracker(t *testing.T) {
	t.Parallel()

	if ProgressTrackerFromContext(context.Background()) != nil {
		t.Fatal("empty context must not carry a tracker")
	}

	//nolint:staticcheck // 显式覆盖 nil context 的兜底分支
	if ProgressTrackerFromContext(nil) != nil {
		t.Fatal("nil context must not carry a tracker")
	}

	tracker := NewTranscodeProgressTracker(1, 0, nil, nil)

	ctx := ContextWithProgressTracker(context.Background(), tracker)
	if got := ProgressTrackerFromContext(ctx); got != tracker {
		t.Fatalf("tracker roundtrip failed: %p vs %p", got, tracker)
	}

	// nil tracker 不改动 context。
	if ContextWithProgressTracker(ctx, nil) != ctx {
		t.Fatal("nil tracker must leave context unchanged")
	}
}

// TestNewQueuedProgress 覆盖投递后的排队快照（外部契约字段）。
func TestNewQueuedProgress(t *testing.T) {
	t.Parallel()

	snapshot := NewQueuedProgress(11, 2)
	if snapshot.Status != TranscodeStatusQueued || snapshot.Stage != TranscodeStageQueued {
		t.Fatalf("queued snapshot = %+v", snapshot)
	}

	if snapshot.Message != "排队等待转码" {
		t.Fatalf("queued message = %q", snapshot.Message)
	}

	if snapshot.Percent != 0 {
		t.Fatalf("queued percent = %v, want 0", snapshot.Percent)
	}

	if snapshot.TTL() != TranscodeProgressRunningTTL {
		t.Fatalf("queued ttl = %v, want running ttl", snapshot.TTL())
	}
}

// TestTranscodeProgressNormalizeClampsPercent 写入前百分比夹紧到 [0,100]。
func TestTranscodeProgressNormalizeClampsPercent(t *testing.T) {
	t.Parallel()

	low := &TranscodeProgress{Percent: -5}
	low.Normalize()

	if low.Percent != 0 {
		t.Fatalf("negative percent = %v, want 0", low.Percent)
	}

	high := &TranscodeProgress{Percent: 250}
	high.Normalize()

	if high.Percent != 100 {
		t.Fatalf("overflow percent = %v, want 100", high.Percent)
	}

	if high.UpdatedAt.IsZero() {
		t.Fatal("Normalize must refresh UpdatedAt")
	}
}

// TestProgressFingerprintDetectsChanges SSE 变化检测依赖指纹区分快照。
func TestProgressFingerprintDetectsChanges(t *testing.T) {
	t.Parallel()

	base := TranscodeProgress{VideoID: 1, Status: TranscodeStatusRunning, Percent: 10, Stage: TranscodeStageMultiRes}
	same := base

	if ProgressFingerprint(base) != ProgressFingerprint(same) {
		t.Fatal("identical snapshots must share a fingerprint")
	}

	changed := base
	changed.Percent = 11

	if ProgressFingerprint(base) == ProgressFingerprint(changed) {
		t.Fatal("percent change must alter the fingerprint")
	}
}

// TestTranscodeProgressPartsKey 分P 集合 key 与单条 key 必须区分。
func TestTranscodeProgressPartsKey(t *testing.T) {
	t.Parallel()

	if TranscodeProgressPartsKey(3) == TranscodeProgressKey(3, 0) {
		t.Fatal("parts hash key must differ from the video-level key")
	}

	if PartFieldKey(7) != "7" {
		t.Fatalf("PartFieldKey(7) = %q, want 7", PartFieldKey(7))
	}
}

// TestTruncateErr 错误文本超长截断并追加省略号。
func TestTruncateErr(t *testing.T) {
	t.Parallel()

	if got := TruncateErr("  boom  ", 100); got != "boom" {
		t.Fatalf("TruncateErr trims = %q", got)
	}

	if got := TruncateErr("abcdef", 3); got != "abc..." {
		t.Fatalf("TruncateErr(abcdef,3) = %q, want abc...", got)
	}

	if got := TruncateErr("abcdef", 0); got != "abcdef" {
		t.Fatalf("TruncateErr with zero max = %q, want unchanged", got)
	}
}
