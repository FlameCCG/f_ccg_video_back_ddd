package runtime

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
)

const (
	transcodeProgressMinWrite = 500 * time.Millisecond
	transcodeProgressMinDelta = 1.0
	// progressFlushTimeout 进度写入的最长等待，保证进度上报永不阻塞 FFmpeg。
	progressFlushTimeout = 2 * time.Second
)

type progressStageWeight struct {
	name   string
	weight float64
}

// progressStageOrder 定义展示顺序与默认权重（总和 = 100）。
func progressStageOrder() [7]progressStageWeight {
	return [7]progressStageWeight{
		{mediadomain.TranscodeStagePrepare, 5},
		{mediadomain.TranscodeStageProbe, 5},
		{mediadomain.TranscodeStageSaveOriginal, 5},
		{mediadomain.TranscodeStageNormalize, 20},
		{mediadomain.TranscodeStageMultiRes, 45},
		{mediadomain.TranscodeStageDash, 15},
		{mediadomain.TranscodeStageCleanup, 5},
	}
}

// ProgressFailObserver 在任务被标记失败时回调，供基础设施层记录日志。
type ProgressFailObserver func(snapshot mediadomain.TranscodeProgress)

// TranscodeProgressTracker 按阶段权重跟踪单个任务的进度，通过注入的 ProgressStore 持久化快照。
type TranscodeProgressTracker struct {
	mu sync.Mutex

	store  ProgressStore
	onFail ProgressFailObserver

	videoID uint
	partID  uint

	status       string
	stage        string
	stagePercent float64
	message      string
	durationSec  int
	outTimeSec   float64
	errMsg       string
	startedAt    time.Time

	// completed 标记已结束的阶段（含被跳过的）。
	completed map[string]bool
	// skipped 从总权重中扣除，使剩余阶段重新归一化。
	skipped map[string]bool

	lastWrite   time.Time
	lastPercent float64
	pipelines   map[string]*mediadomain.TranscodePipelineProgress
}

// NewTranscodeProgressTracker 为视频或分 P 任务创建进度跟踪器。
//
// store 为 nil 时仅在内存累计进度，不写 Redis。
func NewTranscodeProgressTracker(
	videoID, partID uint,
	store ProgressStore,
	onFail ProgressFailObserver,
) *TranscodeProgressTracker {
	return &TranscodeProgressTracker{
		store:     store,
		onFail:    onFail,
		videoID:   videoID,
		partID:    partID,
		status:    mediadomain.TranscodeStatusRunning,
		stage:     mediadomain.TranscodeStagePrepare,
		completed: make(map[string]bool),
		skipped:   make(map[string]bool),
		startedAt: time.Now(),
		pipelines: make(map[string]*mediadomain.TranscodePipelineProgress),
	}
}

type progressTrackerCtxKey struct{}

// ContextWithProgressTracker 把跟踪器挂到 ctx 上，供 FFmpeg 进度回调取用。
func ContextWithProgressTracker(ctx context.Context, pt *TranscodeProgressTracker) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if pt == nil {
		return ctx
	}

	return context.WithValue(ctx, progressTrackerCtxKey{}, pt)
}

// ProgressTrackerFromContext 从 ctx 取出可选的进度跟踪器。
func ProgressTrackerFromContext(ctx context.Context) *TranscodeProgressTracker {
	if ctx == nil {
		return nil
	}

	pt, _ := ctx.Value(progressTrackerCtxKey{}).(*TranscodeProgressTracker)

	return pt
}

// SetDuration 记录媒体时长，供 FFmpeg 百分比计算使用。
func (t *TranscodeProgressTracker) SetDuration(sec int) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if sec > 0 {
		t.durationSec = sec
	}
}

// ConfigurePipeline 登记一条管线及其目标分辨率（管理端展示用）。
func (t *TranscodeProgressTracker) ConfigurePipeline(ctx context.Context, name, message string, resolutions []uint16) {
	if t == nil || name == "" {
		return
	}

	res := append([]uint16(nil), resolutions...)

	t.mu.Lock()
	t.pipelines[name] = &mediadomain.TranscodePipelineProgress{
		Name:        name,
		Status:      mediadomain.TranscodeStatusQueued,
		Message:     message,
		Resolutions: res,
		UpdatedAt:   time.Now(),
	}
	t.mu.Unlock()

	t.flush(ctx, true)
}

// StartPipeline 把一条已登记的管线标记为编码中。
func (t *TranscodeProgressTracker) StartPipeline(ctx context.Context, name, message string) {
	t.updatePipeline(ctx, name, func(p *mediadomain.TranscodePipelineProgress) {
		p.Status = mediadomain.TranscodeStatusRunning
		p.Message = message
		p.Error = ""
	}, true)
}

// UpdatePipelineProgress 把共享 FFmpeg 时间轴进度应用到一条管线。
func (t *TranscodeProgressTracker) UpdatePipelineProgress(ctx context.Context, name string, outTimeSec float64) {
	if t == nil {
		return
	}

	t.mu.Lock()

	p := t.pipelines[name]
	if p == nil {
		t.mu.Unlock()

		return
	}

	percent := 50.0
	if t.durationSec > 0 {
		percent = outTimeSec / float64(t.durationSec) * 100
	}

	if percent < 0 {
		percent = 0
	}

	if percent > 99.5 {
		percent = 99.5
	}

	p.Status = mediadomain.TranscodeStatusRunning
	p.Percent = percent
	p.UpdatedAt = time.Now()
	t.outTimeSec = outTimeSec
	t.mu.Unlock()

	t.flush(ctx, false)
}

// MarkPipelineUploading 编码完成、外部存储尚未落定前，把管线固定在 99.5%。
func (t *TranscodeProgressTracker) MarkPipelineUploading(ctx context.Context, name, message string) {
	t.updatePipeline(ctx, name, func(p *mediadomain.TranscodePipelineProgress) {
		p.Status = mediadomain.TranscodeStatusRunning
		p.Percent = 99.5
		p.Message = message
	}, true)
}

// CompletePipeline 把一条管线标记为整体成功或整体失败。
func (t *TranscodeProgressTracker) CompletePipeline(ctx context.Context, name string, pipelineErr error) {
	t.updatePipeline(ctx, name, func(p *mediadomain.TranscodePipelineProgress) {
		if pipelineErr == nil {
			p.Status = mediadomain.TranscodeStatusSucceeded
			p.Percent = 100
			p.Message = "Pipeline 完成"
			p.Error = ""

			return
		}

		p.Status = mediadomain.TranscodeStatusFailed
		p.Error = errorString(pipelineErr)
		p.Message = "Pipeline 失败"
	}, true)
}

// SkipPipeline 把一条已登记的管线标记为有意不执行。
func (t *TranscodeProgressTracker) SkipPipeline(ctx context.Context, name, message string) {
	t.updatePipeline(ctx, name, func(p *mediadomain.TranscodePipelineProgress) {
		p.Status = mediadomain.TranscodePipelineSkipped
		p.Message = message
	}, true)
}

func (t *TranscodeProgressTracker) updatePipeline(
	ctx context.Context,
	name string,
	update func(*mediadomain.TranscodePipelineProgress),
	force bool,
) {
	if t == nil || update == nil {
		return
	}

	t.mu.Lock()

	p := t.pipelines[name]
	if p == nil {
		t.mu.Unlock()

		return
	}

	update(p)
	p.UpdatedAt = time.Now()
	t.mu.Unlock()

	t.flush(ctx, force)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}

	return TruncateErr(err.Error(), 500)
}

// SkipStage 把某个阶段标记为不适用（权重重新分配）。
func (t *TranscodeProgressTracker) SkipStage(stage string) {
	if t == nil || stage == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.skipped[stage] = true
	t.completed[stage] = true
}

// SkipStageIfUnused 跳过从未运行过的阶段（既非当前阶段也未完成）。
func (t *TranscodeProgressTracker) SkipStageIfUnused(stage string) {
	if t == nil || stage == "" {
		return
	}

	t.mu.Lock()

	if t.completed[stage] || t.stage == stage {
		t.mu.Unlock()

		return
	}

	t.skipped[stage] = true
	t.completed[stage] = true
	t.mu.Unlock()
}

// EnterStage 进入新阶段并立即刷新进度。
func (t *TranscodeProgressTracker) EnterStage(ctx context.Context, stage, message string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	// 推进时把上一个阶段标记为完成。
	if t.stage != "" && t.stage != stage && t.stage != mediadomain.TranscodeStageQueued {
		t.completed[t.stage] = true
	}

	t.status = mediadomain.TranscodeStatusRunning
	t.stage = stage
	t.stagePercent = 0
	t.outTimeSec = 0
	t.message = message
	t.mu.Unlock()

	t.flush(ctx, true)
}

// CompleteStage 标记阶段完成但不进入下一个阶段。
func (t *TranscodeProgressTracker) CompleteStage(ctx context.Context, stage string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	t.completed[stage] = true

	if t.stage == stage {
		t.stagePercent = 1
	}

	t.mu.Unlock()

	t.flush(ctx, true)
}

// UpdateFFmpegProgress 用 out_time / duration 更新当前阶段百分比。
func (t *TranscodeProgressTracker) UpdateFFmpegProgress(ctx context.Context, outTimeSec float64) {
	if t == nil {
		return
	}

	t.mu.Lock()
	t.outTimeSec = outTimeSec

	if t.durationSec > 0 {
		p := outTimeSec / float64(t.durationSec)
		if p < 0 {
			p = 0
		}

		if p > 1 {
			p = 1
		}

		t.stagePercent = p
	} else if outTimeSec > 0 {
		// 时长未知：温和爬升，阶段内封顶。
		t.stagePercent = 0.5
	}

	t.mu.Unlock()

	t.flush(ctx, false)
}

// Succeed 标记任务成功结束。
func (t *TranscodeProgressTracker) Succeed(ctx context.Context) {
	if t == nil {
		return
	}

	t.mu.Lock()

	for _, s := range progressStageOrder() {
		t.completed[s.name] = true
	}

	t.status = mediadomain.TranscodeStatusSucceeded
	t.stage = mediadomain.TranscodeStageCleanup
	t.stagePercent = 1
	t.message = "转码完成"
	t.errMsg = ""
	t.mu.Unlock()

	t.flush(ctx, true)
}

// Fail 标记任务失败并写入错误摘要。
func (t *TranscodeProgressTracker) Fail(ctx context.Context, errMsg string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	t.status = mediadomain.TranscodeStatusFailed
	t.message = "转码失败"
	t.errMsg = TruncateErr(errMsg, 500)

	percent := aggregateProgressPercent(
		t.status,
		t.calcPercentLocked(),
		clonePipelineProgress(t.pipelines),
	)

	snapshot := mediadomain.TranscodeProgress{
		VideoID:    t.videoID,
		PartID:     t.partID,
		Status:     t.status,
		Percent:    percent,
		Stage:      t.stage,
		OutTimeSec: t.outTimeSec,
		Error:      t.errMsg,
	}
	t.mu.Unlock()

	if t.onFail != nil {
		t.onFail(snapshot)
	}

	t.flush(ctx, true)
}

// DiagnosticSnapshot 返回当前进度字段，供中断日志使用。
func (t *TranscodeProgressTracker) DiagnosticSnapshot() (percent float64, stage, status string, outTimeSec float64) {
	if t == nil {
		return 0, "", "", 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	percent = aggregateProgressPercent(
		t.status,
		t.calcPercentLocked(),
		clonePipelineProgress(t.pipelines),
	)

	return percent, t.stage, t.status, t.outTimeSec
}

func (t *TranscodeProgressTracker) flush(ctx context.Context, force bool) {
	if t == nil {
		return
	}

	t.mu.Lock()

	pipelines := clonePipelineProgress(t.pipelines)
	percent := aggregateProgressPercent(t.status, t.calcPercentLocked(), pipelines)
	snapshot := mediadomain.TranscodeProgress{
		VideoID:      t.videoID,
		PartID:       t.partID,
		Status:       t.status,
		Percent:      percent,
		Stage:        t.stage,
		StagePercent: t.stagePercent * 100,
		Message:      t.message,
		DurationSec:  t.durationSec,
		OutTimeSec:   t.outTimeSec,
		Error:        t.errMsg,
		StartedAt:    t.startedAt,
		Pipelines:    pipelines,
	}

	now := time.Now()
	if !force {
		if now.Sub(t.lastWrite) < transcodeProgressMinWrite &&
			absFloat(percent-t.lastPercent) < transcodeProgressMinDelta {
			t.mu.Unlock()

			return
		}
	}

	t.lastWrite = now
	t.lastPercent = percent
	t.mu.Unlock()

	if t.store == nil {
		return
	}

	writeCtx := ctx
	if writeCtx == nil {
		writeCtx = context.Background()
	}

	// 最佳努力写入：用短超时保证进度上报永不阻塞 FFmpeg。
	wctx, cancel := context.WithTimeout(context.WithoutCancel(writeCtx), progressFlushTimeout)
	defer cancel()

	_ = t.store.Write(wctx, &snapshot, ProgressTTL(snapshot))
}

func clonePipelineProgress(src map[string]*mediadomain.TranscodePipelineProgress) []mediadomain.TranscodePipelineProgress {
	out := make([]mediadomain.TranscodePipelineProgress, 0, len(src))

	for _, p := range src {
		if p == nil {
			continue
		}

		item := *p
		item.Resolutions = append([]uint16(nil), p.Resolutions...)
		out = append(out, item)
	}

	slices.SortFunc(out, func(a, b mediadomain.TranscodePipelineProgress) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out
}

func legacyPipelinePercent(pipelines []mediadomain.TranscodePipelineProgress) float64 {
	if len(pipelines) == 0 {
		return 0
	}

	var total float64
	for _, p := range pipelines {
		total += p.Percent
	}

	return total / float64(len(pipelines))
}

func aggregateProgressPercent(
	status string,
	fallback float64,
	pipelines []mediadomain.TranscodePipelineProgress,
) float64 {
	if status == mediadomain.TranscodeStatusSucceeded {
		return 100
	}

	if len(pipelines) > 0 {
		return legacyPipelinePercent(pipelines)
	}

	return fallback
}

func (t *TranscodeProgressTracker) calcPercentLocked() float64 {
	if t.status == mediadomain.TranscodeStatusSucceeded {
		return 100
	}

	if t.status == mediadomain.TranscodeStatusQueued {
		return 0
	}

	var totalWeight float64

	for _, s := range progressStageOrder() {
		if t.skipped[s.name] {
			continue
		}

		totalWeight += s.weight
	}

	if totalWeight <= 0 {
		return 0
	}

	// 已完成且不是当前阶段的算满权重；当前阶段按 stagePercent 折算。
	var (
		doneWeight    float64
		currentWeight float64
	)

	for _, s := range progressStageOrder() {
		if t.skipped[s.name] {
			continue
		}

		if t.completed[s.name] && s.name != t.stage {
			doneWeight += s.weight

			continue
		}

		if s.name == t.stage {
			currentWeight = s.weight
			if t.completed[s.name] {
				doneWeight += s.weight
				currentWeight = 0
			}
		}
	}

	pct := (doneWeight + currentWeight*t.stagePercent) / totalWeight * 100

	if pct > 99.5 && t.status == mediadomain.TranscodeStatusRunning {
		pct = 99.5
	}

	return pct
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}

	return v
}

// TruncateErr 截断错误文本，超长追加省略号。
func TruncateErr(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "..."
}
