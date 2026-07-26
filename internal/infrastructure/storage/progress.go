package storage

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// progressLogger 以 io.Writer 形式挂在 TeeReader 上，按固定节流打印传输进度。
type progressLogger struct {
	totalSize   int64
	currentSize int64
	label       string
	lastLogTime time.Time
}

func newProgressLogger(size int64, label string) *progressLogger {
	return &progressLogger{
		totalSize: size,
		label:     label,
	}
}

// Write 实现 io.Writer 接口，用于 TeeReader 回调。
func (p *progressLogger) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddInt64(&p.currentSize, int64(n))

	// 防止日志刷屏：每 3 秒打印一次进度
	now := time.Now()
	if now.Sub(p.lastLogTime) > 3*time.Second {
		p.printProgress()
		p.lastLogTime = now
	}

	return n, nil
}

func (p *progressLogger) finish() {
	p.printProgress() // 确保最后 100% 被打印
	zap.L().Info("transfer finished", zap.String("label", p.label), zap.Int64("total_bytes", p.currentSize))
}

func (p *progressLogger) printProgress() {
	currentMB := float64(p.currentSize) / 1024 / 1024

	// 场景 A: 已知总大小 (本地文件上传) -> 显示百分比
	if p.totalSize > 0 {
		percent := float64(p.currentSize) / float64(p.totalSize) * 100
		zap.L().Debug("upload progress",
			zap.String("label", p.label),
			zap.String("progress", fmt.Sprintf("%.2f%%", percent)),
			zap.String("size", fmt.Sprintf("%.2f MB / %.2f MB", currentMB, float64(p.totalSize)/1024/1024)),
		)

		return
	}

	// 场景 B: 未知总大小 (流式转码) -> 只显示已上传大小，
	// 避免出现 "progress: -50%" 这类由 size=-1 推导出的荒谬百分比。
	zap.L().Debug("streaming progress",
		zap.String("label", p.label),
		zap.String("uploaded", fmt.Sprintf("%.2f MB", currentMB)),
	)
}
