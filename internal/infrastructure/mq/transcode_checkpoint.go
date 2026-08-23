package mq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

// -----------------------------------------------------------------------------
// 转码断点续传（checkpoint）
//
// 目标：Kafka 重投递 / 进程内重试时不重复编码。任务被拆成可独立确认完成的单元
// （每个 MP4 target、一整套 DASH、远程源暂存文件），完成状态分三层：
//
//	L1 已入库：MySQL 资源行 file_url 与本任务期望产物路径匹配，且远端对象 Stat 有效
//	    → 编码、上传、落库全部跳过。
//	L2 本地已编码：断点目录中存在「已提交」的产物（ffmpeg 成功后经 probe+时长校验、
//	    原子 rename 提交），复用前再次校验 → 只补上传与落库。
//	L3 未完成 → 正常进入 FFmpeg 编码。
//
// 断点目录按 (videoId, partId, fileHash) 确定性命名，跨尝试可寻址；FFmpeg 输出先写
// attempt 子目录，成功校验后 rename 提交，保证崩溃安全（提交即完成标记）。
// 任务成功 / 视频删除 / 永久失败时整目录删除；孤儿目录由启动时的定期清扫兜底。
// -----------------------------------------------------------------------------

const (
	// transcodeCheckpointDirName 默认断点根目录名（挂在系统临时目录下，可用 transcode.checkpointDir 覆盖）。
	transcodeCheckpointDirName = "transcode-checkpoints"
	// transcodeCheckpointMaxAge 孤儿断点目录的最长保留时间，超龄即被清扫。
	transcodeCheckpointMaxAge = 7 * 24 * time.Hour
	// transcodeCheckpointSweepInterval 后台清扫周期。
	transcodeCheckpointSweepInterval = 12 * time.Hour
	// transcodeDashCommittedDirName DASH 整目录提交后的目录名。
	transcodeDashCommittedDirName = "dash-committed"
)

// transcodeCheckpoint 管理单个转码任务的本地断点目录。
// ephemeral 为 true 表示断点续传被禁用：目录随任务结束删除，不跨尝试复用。
type transcodeCheckpoint struct {
	root      string
	ephemeral bool
	logger    *zap.Logger
	// audioMu 串行化共享音轨的生成：MP4 与 DASH 管线并发调用 ensureCheckpointAudioTrack。
	audioMu sync.Mutex
}

// checkpointManager 依据转码配置解析断点根目录、创建任务断点目录并清扫孤儿目录。
//
// 原实现直接读 global.Config()；新架构由构造函数注入 *infraconfig.Provider（支持热更新）。
type checkpointManager struct {
	cfgProvider *infraconfig.Provider
	logger      *zap.Logger
}

// newCheckpointManager 构造断点目录管理器。
func newCheckpointManager(cfgProvider *infraconfig.Provider, logger *zap.Logger) *checkpointManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &checkpointManager{cfgProvider: cfgProvider, logger: logger}
}

// transcodeConf 读取当前转码配置（可能为 nil）。
func (m *checkpointManager) transcodeConf() *config.Transcode {
	if m == nil || m.cfgProvider == nil {
		return nil
	}

	cfg := m.cfgProvider.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Transcode
}

// base 返回断点根目录（配置优先，默认系统临时目录下固定子目录）。
func (m *checkpointManager) base() string {
	if tc := m.transcodeConf(); tc != nil {
		if dir := strings.TrimSpace(tc.CheckpointDir); dir != "" {
			return dir
		}
	}

	return filepath.Join(os.TempDir(), transcodeCheckpointDirName)
}

// disabled 报告断点续传是否被配置关闭。
func (m *checkpointManager) disabled() bool {
	tc := m.transcodeConf()

	return tc != nil && tc.CheckpointDisable
}

// open 打开（或创建）任务的断点目录。
// 禁用断点续传时退化为一次性临时目录，行为与历史版本一致。
func (m *checkpointManager) open(task media.TranscodeTask) (*transcodeCheckpoint, error) {
	if m.disabled() {
		dir, err := os.MkdirTemp("", "transcode-ephemeral-*")
		if err != nil {
			return nil, fmt.Errorf("create ephemeral transcode dir: %w", err)
		}

		return &transcodeCheckpoint{root: dir, ephemeral: true, logger: m.logger}, nil
	}

	root := filepath.Join(m.base(), task.CheckpointKey())
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create transcode checkpoint dir: %w", err)
	}

	return &transcodeCheckpoint{root: root, logger: m.logger}, nil
}

// startJanitor 启动断点目录清扫：立即扫一次，此后周期扫，随 ctx 退出。
func (m *checkpointManager) startJanitor(ctx context.Context) {
	if m.disabled() {
		return
	}

	base := m.base()

	go func() {
		removeExpiredTranscodeCheckpoints(base, transcodeCheckpointMaxAge, m.logger)

		ticker := time.NewTicker(transcodeCheckpointSweepInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				removeExpiredTranscodeCheckpoints(base, transcodeCheckpointMaxAge, m.logger)
			}
		}
	}()
}

// AttemptDir 创建一次尝试专用的输出目录（唯一命名，避免并发尝试互踩）。
func (c *transcodeCheckpoint) AttemptDir(prefix string) (string, error) {
	return os.MkdirTemp(c.root, prefix+"-attempt-*")
}

// CommitFile 将 attempt 目录中的产物原子提交到断点根目录（rename 即完成标记）。
func (c *transcodeCheckpoint) CommitFile(srcPath, name string) (string, error) {
	dest := filepath.Join(c.root, filepath.Base(name))
	if err := os.Rename(srcPath, dest); err != nil {
		// Windows 上目标被占用时 rename 覆盖可能失败：先删再试一次。
		if removeErr := os.Remove(dest); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return "", fmt.Errorf("commit transcode output %q: %w", name, err)
		}

		if err := os.Rename(srcPath, dest); err != nil {
			return "", fmt.Errorf("commit transcode output %q: %w", name, err)
		}
	}

	return dest, nil
}

// CommittedFilePath 返回断点根目录中的提交产物路径与是否存在（size>0 的常规文件才算数）。
func (c *transcodeCheckpoint) CommittedFilePath(name string) (string, bool) {
	dest := filepath.Join(c.root, filepath.Base(name))

	info, err := os.Stat(dest)
	if err != nil || info.IsDir() || info.Size() <= 0 {
		return dest, false
	}

	return dest, true
}

// CommitDir 将整个 attempt 目录原子提交为断点根目录下的 name 子目录（DASH 用）。
func (c *transcodeCheckpoint) CommitDir(srcDir, name string) (string, error) {
	dest := filepath.Join(c.root, filepath.Base(name))
	if err := os.RemoveAll(dest); err != nil {
		return "", fmt.Errorf("clear stale committed dir %q: %w", name, err)
	}

	if err := os.Rename(srcDir, dest); err != nil {
		return "", fmt.Errorf("commit transcode output dir %q: %w", name, err)
	}

	return dest, nil
}

// CommittedDirPath 返回提交子目录路径与是否存在。
func (c *transcodeCheckpoint) CommittedDirPath(name string) (string, bool) {
	dest := filepath.Join(c.root, filepath.Base(name))

	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		return dest, false
	}

	return dest, true
}

// DiscardCommitted 删除一个校验失败的提交产物（文件或目录），迫使重新编码。
func (c *transcodeCheckpoint) DiscardCommitted(name string) {
	dest := filepath.Join(c.root, filepath.Base(name))
	if err := os.RemoveAll(dest); err != nil {
		c.logger.Warn("discard invalid transcode checkpoint artifact failed",
			zap.String("path", dest),
			zap.Error(err),
		)
	}
}

// Remove 删除整个断点目录（任务成功 / 视频删除 / 永久失败时调用）。
func (c *transcodeCheckpoint) Remove() {
	if c == nil || strings.TrimSpace(c.root) == "" {
		return
	}

	if err := os.RemoveAll(c.root); err != nil {
		c.logger.Warn("remove transcode checkpoint dir failed",
			zap.String("dir", c.root),
			zap.Error(err),
		)
	}
}

// persistent 报告断点目录是否可跨尝试复用（分段转码的前置条件）。
func (c *transcodeCheckpoint) persistent() bool {
	return c != nil && !c.ephemeral
}

// Persistent 实现应用层的 Checkpoint 端口，语义同 persistent。
func (c *transcodeCheckpoint) Persistent() bool {
	return c.persistent()
}

// -----------------------------------------------------------------------------
// L2 本地已编码产物的复用校验
// -----------------------------------------------------------------------------

// transcodeOutputValidator 校验一个候选复用产物是否完整可用。
type transcodeOutputValidator func(ctx context.Context, outputPath string) error

// mp4CheckpointValidator 返回 MP4 提交产物的校验器：probe 成功且时长与源片吻合。
func mp4CheckpointValidator(sourceDuration int, logger *zap.Logger) transcodeOutputValidator {
	return func(ctx context.Context, outputPath string) error {
		meta, err := ProbeVideo(ctx, outputPath, logger)
		if err != nil {
			return fmt.Errorf("probe committed output: %w", err)
		}

		return media.ValidateTranscodedDuration(sourceDuration, meta.Duration)
	}
}

// reusableCommittedOutput 检查 target 是否已有可复用的本地提交产物；校验失败时清除产物。
func reusableCommittedOutput(
	ctx context.Context,
	ckpt *transcodeCheckpoint,
	target media.TranscodeTarget,
	validate transcodeOutputValidator,
) (string, bool) {
	if !ckpt.persistent() {
		return "", false
	}

	name := strings.TrimSpace(target.FileName)
	if name == "" {
		return "", false
	}

	committedPath, ok := ckpt.CommittedFilePath(name)
	if !ok {
		return "", false
	}

	if validate != nil {
		if err := validate(ctx, committedPath); err != nil {
			ckpt.logger.Warn("committed transcode output invalid, will re-encode",
				zap.String("targetId", target.ID),
				zap.String("path", committedPath),
				zap.Error(err),
			)
			ckpt.DiscardCommitted(name)

			return "", false
		}
	}

	return committedPath, true
}

// reusableCommittedDashDir 检查断点目录中已提交的整套 DASH 是否可复用：
// manifest 可 probe 且时长与源片吻合；校验失败即丢弃，迫使重新编码。
func reusableCommittedDashDir(ctx context.Context, ckpt *transcodeCheckpoint, sourceDuration int) (string, bool) {
	if !ckpt.persistent() {
		return "", false
	}

	dir, ok := ckpt.CommittedDirPath(transcodeDashCommittedDirName)
	if !ok {
		return "", false
	}

	if err := validateDASHOutput(ctx, dir, sourceDuration, ckpt.logger); err != nil {
		ckpt.logger.Warn("committed dash checkpoint invalid, will re-encode",
			zap.String("dir", dir),
			zap.Error(err),
		)
		ckpt.DiscardCommitted(transcodeDashCommittedDirName)

		return "", false
	}

	return dir, true
}

// -----------------------------------------------------------------------------
// L1 已入库判定：DB 资源行 + 远端对象 Stat
// -----------------------------------------------------------------------------

// transcodeTargetPersisted 判断 MP4 target 是否已完整入库：
// 资源行 file_url 指向本任务期望产物路径，且远端对象仍然存在。
// 匹配时返回资源行的 file_url（源目标续传需要用它接续 currentSourceURL）。
// 任何查询/Stat 失败都按「未完成」处理（宁可重做，不可漏产物）。
func transcodeTargetPersisted(
	ctx context.Context,
	resources media.TranscodeResourceRepository,
	task media.TranscodeTask,
	target media.TranscodeTarget,
	driver media.StorageDriver,
	logger *zap.Logger,
) (string, bool) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if driver == nil || resources == nil || strings.TrimSpace(target.RelativePath) == "" {
		return "", false
	}

	resolution := target.DBResolution
	if target.IsSource {
		// 源行不过滤分辨率：规范化可能改变行的 resolution 标记。
		resolution = enum.VideoResolution("")
	}

	urls, err := resources.PersistedResourceURLs(ctx, task, resolution, target.IsSource)
	if err != nil {
		logger.Debug("lookup persisted transcode resource failed",
			zap.String("targetId", target.ID),
			zap.Error(err),
		)

		return "", false
	}

	for _, storedURL := range urls {
		if !media.StoredPathMatchesRelative(storedURL, target.RelativePath) {
			continue
		}

		size, statErr := driver.StatStoredFile(ctx, storedURL)
		if statErr != nil || size <= 0 {
			logger.Warn("persisted transcode output missing in storage, will redo",
				zap.String("targetId", target.ID),
				zap.String("fileUrl", storedURL),
				zap.Error(statErr),
			)

			return "", false
		}

		return storedURL, true
	}

	return "", false
}

// dashOutputPersisted 判断整套 DASH 是否已完整入库：
// 自适应资源行 file_url 指向本任务的 manifest.mpd，且 manifest 对象存在。
// manifest 最后上传，因此它存在即代表各 representation 均已上传完成。
func dashOutputPersisted(
	ctx context.Context,
	resources media.TranscodeResourceRepository,
	task media.TranscodeTask,
	driver media.StorageDriver,
	logger *zap.Logger,
) bool {
	if logger == nil {
		logger = zap.NewNop()
	}

	if driver == nil || resources == nil || strings.TrimSpace(task.OutputPath) == "" {
		return false
	}

	expectedRel := task.DashManifestRelativePath()

	urls, err := resources.PersistedResourceURLs(ctx, task, enum.ResolutionAuto, false)
	if err != nil {
		logger.Debug("lookup persisted dash resource failed",
			zap.Uint("videoId", task.VideoID),
			zap.Error(err),
		)

		return false
	}

	for _, storedURL := range urls {
		if !media.StoredPathMatchesRelative(storedURL, expectedRel) {
			continue
		}

		size, statErr := driver.StatStoredFile(ctx, storedURL)
		if statErr != nil || size <= 0 {
			logger.Warn("persisted dash manifest missing in storage, will redo",
				zap.Uint("videoId", task.VideoID),
				zap.String("fileUrl", storedURL),
				zap.Error(statErr),
			)

			return false
		}

		return true
	}

	return false
}

// -----------------------------------------------------------------------------
// 孤儿断点目录清扫
// -----------------------------------------------------------------------------

// removeExpiredTranscodeCheckpoints 删除 base 下超龄的断点目录，返回删除数量。
func removeExpiredTranscodeCheckpoints(base string, maxAge time.Duration, logger *zap.Logger) int {
	if logger == nil {
		logger = zap.NewNop()
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn("read transcode checkpoint base failed", zap.String("dir", base), zap.Error(err))
		}

		return 0
	}

	removed := 0
	deadline := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(deadline) {
			continue
		}

		full := filepath.Join(base, entry.Name())
		if err := os.RemoveAll(full); err != nil {
			logger.Warn("remove expired transcode checkpoint failed", zap.String("dir", full), zap.Error(err))

			continue
		}

		removed++

		logger.Info("expired transcode checkpoint removed",
			zap.String("dir", full),
			zap.Time("modTime", info.ModTime()),
		)
	}

	return removed
}

// ensureTranscodeCheckpoint 保证管线始终有可用断点目录；调用方未提供时创建一次性目录。
func ensureTranscodeCheckpoint(
	ckpt *transcodeCheckpoint,
	logger *zap.Logger,
) (*transcodeCheckpoint, func(), error) {
	if ckpt != nil {
		return ckpt, func() {}, nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	dir, err := os.MkdirTemp("", "transcode-ephemeral-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create transcode temp dir failed: %w", err)
	}

	ephemeral := &transcodeCheckpoint{root: dir, ephemeral: true, logger: logger}

	return ephemeral, ephemeral.Remove, nil
}

// committedChunkFiles 返回块目录中所有所需产物（全部存在且非空才算块完成）。
func committedChunkFiles(ckpt *transcodeCheckpoint, dirName string, names []string) (map[string]string, bool) {
	dir, ok := ckpt.CommittedDirPath(dirName)
	if !ok {
		return nil, false
	}

	files := make(map[string]string, len(names))

	for _, name := range names {
		p := filepath.Join(dir, filepath.Base(name))

		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Size() <= 0 {
			return nil, false
		}

		files[name] = p
	}

	return files, true
}

// discardTranscodeChunks 删除某条管线的全部块产物（最终产物提交后块即无用）。
func discardTranscodeChunks(ckpt *transcodeCheckpoint, pipeline string) {
	if ckpt == nil || strings.TrimSpace(ckpt.root) == "" {
		return
	}

	entries, err := os.ReadDir(ckpt.root)
	if err != nil {
		return
	}

	prefix := pipeline + "-chunk-"

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		if err := os.RemoveAll(filepath.Join(ckpt.root, entry.Name())); err != nil {
			ckpt.logger.Warn("discard transcode chunk dir failed",
				zap.String("dir", entry.Name()),
				zap.Error(err),
			)
		}
	}
}
