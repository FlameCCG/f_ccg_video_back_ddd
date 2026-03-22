package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
)

// fakeProgressStore 记录写入的进度快照，供测试验证。
type fakeProgressStore struct {
	mu        sync.Mutex
	snapshots []mediadomain.TranscodeProgress
	ttls      []time.Duration
	cleared   [][2]uint
}

func (s *fakeProgressStore) Write(_ context.Context, snapshot *mediadomain.TranscodeProgress, ttl time.Duration) error {
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
func (s *fakeProgressStore) last() (mediadomain.TranscodeProgress, time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.snapshots) == 0 {
		return mediadomain.TranscodeProgress{}, 0, false
	}

	return s.snapshots[len(s.snapshots)-1], s.ttls[len(s.ttls)-1], true
}

func TestPipelineProgressUpdatesSharedPercent(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}
	tracker := NewTranscodeProgressTracker(1, 0, store, nil)
	tracker.SetDuration(100)
	tracker.ConfigurePipeline(context.Background(), mediadomain.TranscodePipelineMP4, "queued", []uint16{1080, 720})
	tracker.StartPipeline(context.Background(), mediadomain.TranscodePipelineMP4, "encoding")
	tracker.UpdatePipelineProgress(context.Background(), mediadomain.TranscodePipelineMP4, 42)

	// UpdatePipelineProgress 采用非强制 flush，受 500ms/1% 节流限制，不立即落库；测试从 tracker 的查询方法读取进度。
	percent, _, status, outTime := tracker.DiagnosticSnapshot()
	if percent != 42 {
		t.Fatalf("pipeline percent = %v, want 42", percent)
	}

	if status != mediadomain.TranscodeStatusRunning {
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

	tracker.EnterStage(context.Background(), mediadomain.TranscodeStagePrepare, "准备源文件")

	running, ttl, ok := store.last()
	if !ok {
		t.Fatal("no running snapshot")
	}

	if running.Status != mediadomain.TranscodeStatusRunning {
		t.Fatalf("status = %q, want running", running.Status)
	}

	if ttl != ProgressRunningTTL {
		t.Fatalf("running ttl = %v, want %v", ttl, ProgressRunningTTL)
	}

	tracker.Succeed(context.Background())

	done, doneTTL, _ := store.last()
	if done.Status != mediadomain.TranscodeStatusSucceeded || done.Percent != 100 {
		t.Fatalf("succeed snapshot = %+v, want succeeded/100", done)
	}

	if doneTTL != ProgressFinalTTL {
		t.Fatalf("final ttl = %v, want %v", doneTTL, ProgressFinalTTL)
	}

	if done.VideoID != 3 || done.PartID != 7 {
		t.Fatalf("identity = (%d,%d), want (3,7)", done.VideoID, done.PartID)
	}
}

// TestTrackerFailInvokesObserver 验证失败时调用观察者回调。
func TestTrackerFailInvokesObserver(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}

	var observed mediadomain.TranscodeProgress

	tracker := NewTranscodeProgressTracker(9, 0, store, func(snapshot mediadomain.TranscodeProgress) {
		observed = snapshot
	})

	tracker.EnterStage(context.Background(), mediadomain.TranscodeStageMultiRes, "执行并行转码 Pipeline")
	tracker.Fail(context.Background(), "ffmpeg pipeline failed: signal: killed")

	if observed.Status != mediadomain.TranscodeStatusFailed {
		t.Fatalf("observed status = %q, want failed", observed.Status)
	}

	if observed.Error == "" {
		t.Fatal("observed snapshot must carry error text")
	}

	failed, ttl, _ := store.last()
	if failed.Status != mediadomain.TranscodeStatusFailed || failed.Message != "转码失败" {
		t.Fatalf("failed snapshot = %+v", failed)
	}

	if ttl != ProgressFinalTTL {
		t.Fatalf("failed ttl = %v, want final %v", ttl, ProgressFinalTTL)
	}
}

// TestTrackerSkipStageRenormalizesWeights 跳过阶段后剩余阶段权重重新归一化。
func TestTrackerSkipStageRenormalizesWeights(t *testing.T) {
	t.Parallel()

	store := &fakeProgressStore{}
	tracker := NewTranscodeProgressTracker(1, 0, store, nil)

	tracker.SkipStage(mediadomain.TranscodeStageDash)
	tracker.SkipStage(mediadomain.TranscodeStageNormalize)
	tracker.EnterStage(context.Background(), mediadomain.TranscodeStagePrepare, "准备源文件")
	tracker.CompleteStage(context.Background(), mediadomain.TranscodeStagePrepare)

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
	tracker.EnterStage(context.Background(), mediadomain.TranscodeStageMultiRes, "执行并行转码 Pipeline")

	before, _, _ := store.last()

	tracker.SkipStageIfUnused(mediadomain.TranscodeStageMultiRes)
	tracker.EnterStage(context.Background(), mediadomain.TranscodeStageCleanup, "清理与刷新缓存")

	after, _, _ := store.last()
	if after.Percent <= before.Percent {
		t.Fatalf("percent must advance: before=%v after=%v", before.Percent, after.Percent)
	}
}

// TestTrackerNilStoreIsSafe 验证 store 为 nil 时只在内存累计进度。
func TestTrackerNilStoreIsSafe(t *testing.T) {
	t.Parallel()

	tracker := NewTranscodeProgressTracker(1, 0, nil, nil)
	tracker.SetDuration(100)
	tracker.EnterStage(context.Background(), mediadomain.TranscodeStagePrepare, "准备源文件")
	tracker.UpdateFFmpegProgress(context.Background(), 50)
	tracker.Succeed(context.Background())

	percent, stage, status, _ := tracker.DiagnosticSnapshot()
	if status != mediadomain.TranscodeStatusSucceeded || percent != 100 || stage != mediadomain.TranscodeStageCleanup {
		t.Fatalf("diagnostic = (%v,%q,%q)", percent, stage, status)
	}
}

func TestAggregateProgressPercent(t *testing.T) {
	t.Parallel()

	pipelines := []mediadomain.TranscodePipelineProgress{
		{Name: mediadomain.TranscodePipelineDASH, Percent: 98.958325},
		{Name: mediadomain.TranscodePipelineMP4, Percent: 100},
	}

	if got := aggregateProgressPercent(mediadomain.TranscodeStatusSucceeded, 100, pipelines); got != 100 {
		t.Fatalf("succeeded aggregate = %v, want 100", got)
	}

	wantRunning := (98.958325 + 100) / 2
	if got := aggregateProgressPercent(mediadomain.TranscodeStatusRunning, 80, pipelines); got != wantRunning {
		t.Fatalf("running aggregate = %v, want %v", got, wantRunning)
	}

	if got := aggregateProgressPercent(mediadomain.TranscodeStatusRunning, 42, nil); got != 42 {
		t.Fatalf("aggregate without pipelines = %v, want 42", got)
	}
}

// TestNilTrackerMethodsAreSafe nil tracker 的全部方法必须安全（消费者在早失败路径上会传 nil）。
func TestNilTrackerMethodsAreSafe(t *testing.T) {
	t.Parallel()

	var tracker *TranscodeProgressTracker

	tracker.SetDuration(10)
	tracker.EnterStage(context.Background(), mediadomain.TranscodeStagePrepare, "x")
	tracker.CompleteStage(context.Background(), mediadomain.TranscodeStagePrepare)
	tracker.UpdateFFmpegProgress(context.Background(), 1)
	tracker.ConfigurePipeline(context.Background(), mediadomain.TranscodePipelineMP4, "x", nil)
	tracker.StartPipeline(context.Background(), mediadomain.TranscodePipelineMP4, "x")
	tracker.UpdatePipelineProgress(context.Background(), mediadomain.TranscodePipelineMP4, 1)
	tracker.MarkPipelineUploading(context.Background(), mediadomain.TranscodePipelineMP4, "x")
	tracker.CompletePipeline(context.Background(), mediadomain.TranscodePipelineMP4, nil)
	tracker.SkipPipeline(context.Background(), mediadomain.TranscodePipelineMP4, "x")
	tracker.SkipStage(mediadomain.TranscodeStageDash)
	tracker.SkipStageIfUnused(mediadomain.TranscodeStageDash)
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

	snapshot := mediadomain.NewQueuedProgress(11, 2, time.Now())
	if snapshot.Status != mediadomain.TranscodeStatusQueued || snapshot.Stage != mediadomain.TranscodeStageQueued {
		t.Fatalf("queued snapshot = %+v", snapshot)
	}

	if snapshot.Message != "排队等待转码" {
		t.Fatalf("queued message = %q", snapshot.Message)
	}

	if snapshot.Percent != 0 {
		t.Fatalf("queued percent = %v, want 0", snapshot.Percent)
	}

	if ProgressTTL(*snapshot) != ProgressRunningTTL {
		t.Fatalf("queued ttl = %v, want running ttl", ProgressTTL(*snapshot))
	}
}

// TestTranscodeProgressNormalizeClampsPercent 写入前百分比夹紧到 [0,100]。
func TestTranscodeProgressNormalizeClampsPercent(t *testing.T) {
	t.Parallel()

	low := &mediadomain.TranscodeProgress{Percent: -5}
	low.Normalize(time.Now())

	if low.Percent != 0 {
		t.Fatalf("negative percent = %v, want 0", low.Percent)
	}

	high := &mediadomain.TranscodeProgress{Percent: 250}
	high.Normalize(time.Now())

	if high.Percent != 100 {
		t.Fatalf("overflow percent = %v, want 100", high.Percent)
	}

	if high.UpdatedAt.IsZero() {
		t.Fatal("Normalize must refresh UpdatedAt")
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
