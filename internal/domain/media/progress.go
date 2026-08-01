package media

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"f_ccg_video_back_ddd/pkg/constants"
)

// 转码任务在 Redis 中的状态取值（外部契约，逐字保持）。
const (
	// TranscodeStatusQueued 已投递、等待消费。
	TranscodeStatusQueued = "queued"
	// TranscodeStatusRunning 正在转码。
	TranscodeStatusRunning = "running"
	// TranscodeStatusSucceeded 转码成功。
	TranscodeStatusSucceeded = "succeeded"
	// TranscodeStatusFailed 转码失败。
	TranscodeStatusFailed = "failed"
)

// 转码管线阶段（进度上报用）。
const (
	// TranscodeStageQueued 排队中。
	TranscodeStageQueued = "queued"
	// TranscodeStagePrepare 准备源文件。
	TranscodeStagePrepare = "prepare"
	// TranscodeStageProbe 探测元信息。
	TranscodeStageProbe = "probe"
	// TranscodeStageSaveOriginal 保存源视频资源。
	TranscodeStageSaveOriginal = "save_original"
	// TranscodeStageNormalize 源片规范化。
	TranscodeStageNormalize = "normalize"
	// TranscodeStageMultiRes 多分辨率转码。
	TranscodeStageMultiRes = "multi_res"
	// TranscodeStageDash DASH 产出。
	TranscodeStageDash = "dash"
	// TranscodeStageCleanup 清理与缓存刷新。
	TranscodeStageCleanup = "cleanup"
)

// 进度写入节流与 TTL（外部契约，逐字保持）。
const (
	// TranscodeProgressRunningTTL 运行中进度快照 TTL。
	TranscodeProgressRunningTTL = 2 * time.Hour
	// TranscodeProgressFinalTTL 终态进度快照 TTL。
	TranscodeProgressFinalTTL = 24 * time.Hour

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
		{TranscodeStagePrepare, 5},
		{TranscodeStageProbe, 5},
		{TranscodeStageSaveOriginal, 5},
		{TranscodeStageNormalize, 20},
		{TranscodeStageMultiRes, 45},
		{TranscodeStageDash, 15},
		{TranscodeStageCleanup, 5},
	}
}

// TranscodeProgress 是单个视频/分P 任务的进度快照（Redis 中存储的结构）。
type TranscodeProgress struct {
	VideoID      uint                        `json:"videoId"`
	PartID       uint                        `json:"partId,omitempty"`
	Status       string                      `json:"status"`
	Percent      float64                     `json:"percent"`
	Stage        string                      `json:"stage"`
	StagePercent float64                     `json:"stagePercent"`
	Message      string                      `json:"message,omitempty"`
	DurationSec  int                         `json:"durationSec,omitempty"`
	OutTimeSec   float64                     `json:"outTimeSec,omitempty"`
	Error        string                      `json:"error,omitempty"`
	UpdatedAt    time.Time                   `json:"updatedAt"`
	StartedAt    time.Time                   `json:"startedAt,omitempty"`
	Pipelines    []TranscodePipelineProgress `json:"pipelines,omitempty"`
}

// TranscodePipelineProgress 描述一条 FFmpeg 管线的进度（共享时间轴，无单输出百分比）。
type TranscodePipelineProgress struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Percent     float64   `json:"percent"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	Resolutions []uint16  `json:"resolutions,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Terminal 报告进度是否已进入终态。
func (p TranscodeProgress) Terminal() bool {
	return p.Status == TranscodeStatusSucceeded || p.Status == TranscodeStatusFailed
}

// TTL 返回该快照应使用的 Redis TTL：终态 24 小时，其余 2 小时。
func (p TranscodeProgress) TTL() time.Duration {
	if p.Terminal() {
		return TranscodeProgressFinalTTL
	}

	return TranscodeProgressRunningTTL
}

// Normalize 在写入前夹紧百分比区间并刷新更新时间（与原 writeTranscodeProgress 一致）。
func (p *TranscodeProgress) Normalize() {
	if p == nil {
		return
	}

	if p.Percent < 0 {
		p.Percent = 0
	}

	if p.Percent > 100 {
		p.Percent = 100
	}

	p.UpdatedAt = time.Now()
}

// TranscodeProgressKey 构造视频/分P 进度的 Redis key。
func TranscodeProgressKey(videoID, partID uint) string {
	if partID > 0 {
		return fmt.Sprintf("%s%d:%d", constants.TranscodeProgressKeyPrefix, videoID, partID)
	}

	return fmt.Sprintf("%s%d", constants.TranscodeProgressKeyPrefix, videoID)
}

// TranscodeProgressPartsKey 构造某视频下各分P 进度的哈希 key。
func TranscodeProgressPartsKey(videoID uint) string {
	return fmt.Sprintf("%s%d:parts", constants.TranscodeProgressKeyPrefix, videoID)
}

// NewQueuedProgress 构造「已投递、排队等待转码」的初始进度快照。
func NewQueuedProgress(videoID, partID uint) *TranscodeProgress {
	now := time.Now()

	return &TranscodeProgress{
		VideoID:   videoID,
		PartID:    partID,
		Status:    TranscodeStatusQueued,
		Percent:   0,
		Stage:     TranscodeStageQueued,
		Message:   "排队等待转码",
		UpdatedAt: now,
		StartedAt: now,
	}
}

// ProgressFailObserver 在任务被标记失败时回调，供基础设施层记录日志。
type ProgressFailObserver func(snapshot TranscodeProgress)

// TranscodeProgressTracker 按阶段权重跟踪单个运行中任务的进度。
//
// 原实现直接读写 global.RedisClient；新架构通过注入的 ProgressStore 端口落地，
// 阶段权重与百分比算法逐行保持一致。
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
	pipelines   map[string]*TranscodePipelineProgress
}

// NewTranscodeProgressTracker 为一个视频/分P 任务创建进度跟踪器。
// store 为 nil 时只在内存中累计进度（不落 Redis），与原 RedisClient 缺失时行为一致。
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
		status:    TranscodeStatusRunning,
		stage:     TranscodeStagePrepare,
		completed: make(map[string]bool),
		skipped:   make(map[string]bool),
		startedAt: time.Now(),
		pipelines: make(map[string]*TranscodePipelineProgress),
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
	t.pipelines[name] = &TranscodePipelineProgress{
		Name:        name,
		Status:      TranscodeStatusQueued,
		Message:     message,
		Resolutions: res,
		UpdatedAt:   time.Now(),
	}
	t.mu.Unlock()

	t.flush(ctx, true)
}

// StartPipeline 把一条已登记的管线标记为编码中。
func (t *TranscodeProgressTracker) StartPipeline(ctx context.Context, name, message string) {
	t.updatePipeline(ctx, name, func(p *TranscodePipelineProgress) {
		p.Status = TranscodeStatusRunning
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

	p.Status = TranscodeStatusRunning
	p.Percent = percent
	p.UpdatedAt = time.Now()
	t.outTimeSec = outTimeSec
	t.mu.Unlock()

	t.flush(ctx, false)
}

// MarkPipelineUploading 编码完成、外部存储尚未落定前，把管线固定在 99.5%。
func (t *TranscodeProgressTracker) MarkPipelineUploading(ctx context.Context, name, message string) {
	t.updatePipeline(ctx, name, func(p *TranscodePipelineProgress) {
		p.Status = TranscodeStatusRunning
		p.Percent = 99.5
		p.Message = message
	}, true)
}

// CompletePipeline 把一条管线标记为整体成功或整体失败。
func (t *TranscodeProgressTracker) CompletePipeline(ctx context.Context, name string, pipelineErr error) {
	t.updatePipeline(ctx, name, func(p *TranscodePipelineProgress) {
		if pipelineErr == nil {
			p.Status = TranscodeStatusSucceeded
			p.Percent = 100
			p.Message = "Pipeline 完成"
			p.Error = ""

			return
		}

		p.Status = TranscodeStatusFailed
		p.Error = errorString(pipelineErr)
		p.Message = "Pipeline 失败"
	}, true)
}

// SkipPipeline 把一条已登记的管线标记为有意不执行。
func (t *TranscodeProgressTracker) SkipPipeline(ctx context.Context, name, message string) {
	t.updatePipeline(ctx, name, func(p *TranscodePipelineProgress) {
		p.Status = TranscodePipelineSkipped
		p.Message = message
	}, true)
}

func (t *TranscodeProgressTracker) updatePipeline(
	ctx context.Context,
	name string,
	update func(*TranscodePipelineProgress),
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
	if t.stage != "" && t.stage != stage && t.stage != TranscodeStageQueued {
		t.completed[t.stage] = true
	}

	t.status = TranscodeStatusRunning
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

	t.status = TranscodeStatusSucceeded
	t.stage = TranscodeStageCleanup
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
	t.status = TranscodeStatusFailed
	t.message = "转码失败"
	t.errMsg = TruncateErr(errMsg, 500)

	percent := aggregateProgressPercent(
		t.status,
		t.calcPercentLocked(),
		clonePipelineProgress(t.pipelines),
	)

	snapshot := TranscodeProgress{
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
	snapshot := TranscodeProgress{
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

	_ = t.store.Write(wctx, &snapshot, snapshot.TTL())
}

func clonePipelineProgress(src map[string]*TranscodePipelineProgress) []TranscodePipelineProgress {
	out := make([]TranscodePipelineProgress, 0, len(src))

	for _, p := range src {
		if p == nil {
			continue
		}

		item := *p
		item.Resolutions = append([]uint16(nil), p.Resolutions...)
		out = append(out, item)
	}

	slices.SortFunc(out, func(a, b TranscodePipelineProgress) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out
}

func legacyPipelinePercent(pipelines []TranscodePipelineProgress) float64 {
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
	pipelines []TranscodePipelineProgress,
) float64 {
	if status == TranscodeStatusSucceeded {
		return 100
	}

	if len(pipelines) > 0 {
		return legacyPipelinePercent(pipelines)
	}

	return fallback
}

func (t *TranscodeProgressTracker) calcPercentLocked() float64 {
	if t.status == TranscodeStatusSucceeded {
		return 100
	}

	if t.status == TranscodeStatusQueued {
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

	if pct > 99.5 && t.status == TranscodeStatusRunning {
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

// ProgressFingerprint 生成进度快照指纹，供 SSE 变化检测使用。
func ProgressFingerprint(p TranscodeProgress) string {
	return fmt.Sprintf("%d:%d:%s:%.1f:%s:%.0f:%s",
		p.VideoID, p.PartID, p.Status, p.Percent, p.Stage, p.StagePercent, p.Message)
}

// PartFieldKey 返回分P 进度在哈希中的字段名。
func PartFieldKey(partID uint) string {
	return strconv.FormatUint(uint64(partID), 10)
}
