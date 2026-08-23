package mq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	mediaapp "f_ccg_video_back_ddd/internal/application/media"
	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// TranscodeEngine 是转码执行侧的基础设施适配器：ffprobe 探测、断点目录管理、
// 源文件准备、「已入库」判定与 FFmpeg 编码执行。
//
// 它把原 service/mq_service 中所有直接触碰进程/文件系统/DB 的能力收敛到一处，
// 使应用层（internal/application/media）只做编排。包内无包级可变状态。
type TranscodeEngine struct {
	checkpoints *checkpointManager
	resources   media.TranscodeResourceRepository
	httpClient  *http.Client
	logger      *zap.Logger
}

// 编译期断言：满足应用层声明的四个执行侧端口。
var (
	_ mediaapp.MediaProber          = (*TranscodeEngine)(nil)
	_ mediaapp.CheckpointManager    = (*TranscodeEngine)(nil)
	_ mediaapp.SourceStager         = (*TranscodeEngine)(nil)
	_ mediaapp.ArtifactStateChecker = (*TranscodeEngine)(nil)
	_ mediaapp.TranscodeEncoder     = (*TranscodeEngine)(nil)
)

// NewTranscodeEngine 构造转码执行引擎。
func NewTranscodeEngine(
	cfgProvider *infraconfig.Provider,
	resources media.TranscodeResourceRepository,
	logger *zap.Logger,
) *TranscodeEngine {
	return &TranscodeEngine{
		checkpoints: newCheckpointManager(cfgProvider, logger),
		resources:   resources,
		logger:      logger,
		// 与原实现一致：源体积探测用带超时的 HEAD 客户端，下载走 http.DefaultClient。
		httpClient: &http.Client{Timeout: ctxutil.ExternalHeadTimeout},
	}
}

// -----------------------------------------------------------------------------
// MediaProber
// -----------------------------------------------------------------------------

// ProbeVideo 探测本地媒体文件元数据。
func (e *TranscodeEngine) ProbeVideo(ctx context.Context, path string) (media.VideoMeta, error) {
	return ProbeVideo(ctx, path, e.logger)
}

// SourceSize 返回任务源文件字节数：本地源 os.Stat，远程源 HTTP HEAD。
func (e *TranscodeEngine) SourceSize(ctx context.Context, task media.TranscodeTask) (int64, error) {
	if task.StorageType == enum.StorageTypeLocal {
		info, err := os.Stat(task.SourcePath)
		if err != nil {
			return 0, err
		}

		return info.Size(), nil
	}

	return e.statHTTPFile(ctx, task.SourcePath)
}

// statHTTPFile 用 HEAD 请求获取远程资源大小。
func (e *TranscodeEngine) statHTTPFile(ctx context.Context, source string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, source, nil)
	if err != nil {
		return 0, err
	}

	client := e.httpClient
	if client == nil {
		client = &http.Client{Timeout: ctxutil.ExternalHeadTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return 0, fmt.Errorf("head source failed: %s", resp.Status)
	}

	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}

	return 0, errors.New("content length unavailable")
}

// -----------------------------------------------------------------------------
// CheckpointManager
// -----------------------------------------------------------------------------

// OpenCheckpoint 打开（或创建）任务的断点目录。
func (e *TranscodeEngine) OpenCheckpoint(task media.TranscodeTask) (mediaapp.Checkpoint, error) {
	return e.checkpoints.open(task)
}

// StartCheckpointJanitor 启动断点目录孤儿清扫（随 ctx 退出）。
func (e *TranscodeEngine) StartCheckpointJanitor(ctx context.Context) {
	e.checkpoints.startJanitor(ctx)
}

// -----------------------------------------------------------------------------
// SourceStager
// -----------------------------------------------------------------------------

// PrepareSource 准备可供 ffmpeg 读取的本地源文件（原 prepareSourceFile）。
func (e *TranscodeEngine) PrepareSource(
	ctx context.Context,
	task media.TranscodeTask,
	ckpt mediaapp.Checkpoint,
) (string, func(), error) {
	if task.StorageType == enum.StorageTypeLocal {
		if _, err := os.Stat(task.SourcePath); err != nil {
			return "", nil, err
		}

		return task.SourcePath, func() {}, nil
	}

	if err := media.EnsureRemoteSource(task.SourcePath); err != nil {
		return "", nil, err
	}

	return e.stageRemoteSource(ctx, task.SourcePath, asCheckpoint(ckpt))
}

// reuseStagedSource 尝试复用断点目录中已提交的暂存源。
//
// 能拿到远端 Content-Length 时核对大小，不一致则丢弃；拿不到时信任提交时的完整性校验。
func (e *TranscodeEngine) reuseStagedSource(
	ctx context.Context,
	sourceURL string,
	ckpt *transcodeCheckpoint,
) (string, bool) {
	if !ckpt.persistent() {
		return "", false
	}

	name := media.StagedSourceCheckpointName(sourceURL)

	committedPath, ok := ckpt.CommittedFilePath(name)
	if !ok {
		return "", false
	}

	info, err := os.Stat(committedPath)
	if err != nil || info.Size() <= 0 {
		return "", false
	}

	if remoteSize, err := e.statHTTPFile(ctx, sourceURL); err == nil && remoteSize > 0 && remoteSize != info.Size() {
		e.logger.Warn("staged transcode source size mismatch, will re-download",
			zap.String("sourceURL", sourceURL),
			zap.Int64("stagedSize", info.Size()),
			zap.Int64("remoteSize", remoteSize),
		)
		ckpt.DiscardCommitted(name)

		return "", false
	}

	e.logger.Info("staged transcode source reused from checkpoint",
		zap.String("sourceURL", sourceURL),
		zap.String("stagedPath", committedPath),
		zap.Int64("size", info.Size()),
	)

	return committedPath, true
}

// stageRemoteSource 下载远程源到本地（优先落断点目录并原子提交）。
func (e *TranscodeEngine) stageRemoteSource(
	ctx context.Context,
	sourceURL string,
	ckpt *transcodeCheckpoint,
) (string, func(), error) {
	// 断点复用：上次重试已完整下载过的源直接用，cleanup 交由断点目录生命周期管理。
	if stagedPath, ok := e.reuseStagedSource(ctx, sourceURL, ckpt); ok {
		return stagedPath, func() {}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("create remote source request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download remote source: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", nil, fmt.Errorf("download remote source returned status %d", resp.StatusCode)
	}

	pattern := media.StagedSourceTempPattern(req.URL.Path)

	var file *os.File
	if ckpt.persistent() {
		// 下载进断点目录（同盘保证后续 rename 提交为原子操作）。
		file, err = os.CreateTemp(ckpt.root, pattern)
	} else {
		file, err = os.CreateTemp("", pattern)
	}

	if err != nil {
		return "", nil, fmt.Errorf("create staged source file: %w", err)
	}

	stagedPath := file.Name()
	cleanup := func() { _ = os.Remove(stagedPath) }

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		_ = file.Close()
		cleanup()

		return "", nil, fmt.Errorf("copy remote source: %w", err)
	}

	if err := file.Close(); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("close staged source file: %w", err)
	}

	if written <= 0 {
		cleanup()

		return "", nil, errors.New("remote source is empty")
	}

	if resp.ContentLength >= 0 && written != resp.ContentLength {
		cleanup()

		return "", nil, fmt.Errorf(
			"remote source truncated: downloaded %d bytes, expected %d",
			written,
			resp.ContentLength,
		)
	}

	if ckpt.persistent() {
		// 校验通过后原子提交，供后续重试复用；提交失败退回一次性暂存语义。
		committedPath, commitErr := ckpt.CommitFile(stagedPath, media.StagedSourceCheckpointName(sourceURL))
		if commitErr == nil {
			stagedPath = committedPath
			cleanup = func() {}
		} else {
			e.logger.Warn("commit staged transcode source failed, keep ephemeral copy",
				zap.String("sourceURL", sourceURL),
				zap.Error(commitErr),
			)
		}
	}

	e.logger.Info("remote transcode source staged locally",
		zap.String("sourceURL", sourceURL),
		zap.String("stagedPath", stagedPath),
		zap.Int64("size", written),
	)

	return stagedPath, cleanup, nil
}

// -----------------------------------------------------------------------------
// ArtifactStateChecker
// -----------------------------------------------------------------------------

// TargetPersisted 判断 MP4 target 是否已完整入库。
func (e *TranscodeEngine) TargetPersisted(
	ctx context.Context,
	task media.TranscodeTask,
	target media.TranscodeTarget,
	driver media.StorageDriver,
) (string, bool) {
	return transcodeTargetPersisted(ctx, e.resources, task, target, driver, e.logger)
}

// DASHPersisted 判断整套 DASH 是否已完整入库。
func (e *TranscodeEngine) DASHPersisted(
	ctx context.Context,
	task media.TranscodeTask,
	driver media.StorageDriver,
) bool {
	return dashOutputPersisted(ctx, e.resources, task, driver, e.logger)
}

// -----------------------------------------------------------------------------
// 辅助：Checkpoint 端口 → 内部断点目录类型
// -----------------------------------------------------------------------------

// asCheckpoint 把应用层的 Checkpoint 端口还原为内部实现类型。
// 端口的唯一实现就是 *transcodeCheckpoint（由本引擎创建），
// 传入其它实现或 nil 时返回 nil，调用方按「无断点目录」处理（等价原 ckpt == nil 分支）。
func asCheckpoint(ckpt mediaapp.Checkpoint) *transcodeCheckpoint {
	if ckpt == nil {
		return nil
	}

	inner, _ := ckpt.(*transcodeCheckpoint)

	return inner
}

// -----------------------------------------------------------------------------
// 辅助：产物文件名
// -----------------------------------------------------------------------------

// outputFileName 返回 target 的产物文件名；缺失时按序号与分辨率兜底（与原实现一致）。
func outputFileName(target media.TranscodeTarget, index int) string {
	name := strings.TrimSpace(target.FileName)
	if name == "" {
		name = fmt.Sprintf("out_%d_%dp.mp4", index, target.Resolution)
	}

	return filepath.Base(name)
}
