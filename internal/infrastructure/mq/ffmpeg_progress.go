package mq

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	media "f_ccg_video_back_ddd/internal/domain/media"
)

// attachFFmpegProgress 把 cmd.Stderr 接到一个 tee 上：既缓存错误文本，又解析进度。
//
// 需在构造 cmd 之后、Start 之前调用；返回的 afterStart 在 Start 成功后调用，
// waitProgress 必须在 Wait 之后（或 Start 失败后）调用。
// 会注入 "-progress pipe:2"，使 FFmpeg 即便在 -loglevel error 下也向 stderr 输出进度。
func attachFFmpegProgress(
	cmd *exec.Cmd,
	onProgress media.ProgressCallback,
) (stderrBuf *strings.Builder, afterStart, waitProgress func()) {
	if cmd == nil {
		b := &strings.Builder{}

		return b, func() {}, func() {}
	}

	// 确保进度写到 stderr (fd 2)。
	cmd.Args = media.InjectFFmpegProgressArgs(cmd.Args)

	pr, pw := io.Pipe()
	cmd.Stderr = pw

	var (
		buf strings.Builder
		wg  sync.WaitGroup
	)

	afterStart = func() {
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() { _ = pr.Close() }()

			parseFFmpegProgressStream(pr, &buf, onProgress)
		}()
	}

	waitProgress = func() {
		_ = pw.Close()

		wg.Wait()
	}

	return &buf, afterStart, waitProgress
}

// parseFFmpegProgressStream 逐行读取 FFmpeg stderr：缓存错误文本并按节流回调进度。
func parseFFmpegProgressStream(r io.Reader, errBuf *strings.Builder, onProgress media.ProgressCallback) {
	scanner := bufio.NewScanner(r)
	// FFmpeg 进度行很短；放宽上限以容纳错误转储。
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lastEmit time.Time

	for scanner.Scan() {
		line := scanner.Text()

		if errBuf != nil {
			// 保留 stderr 文本用于失败诊断（限制大小）。
			if errBuf.Len() < 32*1024 {
				errBuf.WriteString(line)
				errBuf.WriteByte('\n')
			}
		}

		outSec, ok := media.ParseFFmpegProgressLine(line)
		if !ok {
			// 兼容经典 stderr "time=HH:MM:SS.xx"
			if sec, ok2 := media.ParseFFmpegTimeEquals(line); ok2 {
				outSec, ok = sec, true
			}
		}

		if !ok || onProgress == nil {
			continue
		}

		now := time.Now()
		if now.Sub(lastEmit) < 400*time.Millisecond {
			continue
		}

		lastEmit = now

		onProgress(outSec)
	}

	if err := scanner.Err(); err != nil && errBuf != nil && errBuf.Len() < 32*1024 {
		errBuf.WriteString("ffmpeg progress scan error: ")
		errBuf.WriteString(err.Error())
		errBuf.WriteByte('\n')
	}
}

// progressCbForPipeline 返回绑定 tracker 与某条管线的进度回调。
func progressCbForPipeline(
	ctx context.Context,
	tracker *media.TranscodeProgressTracker,
	pipeline string,
) media.ProgressCallback {
	if tracker == nil || pipeline == "" {
		return nil
	}

	return func(outTimeSec float64) {
		if ctx != nil && ctx.Err() != nil {
			return
		}

		tracker.UpdatePipelineProgress(ctx, pipeline, outTimeSec)
	}
}
