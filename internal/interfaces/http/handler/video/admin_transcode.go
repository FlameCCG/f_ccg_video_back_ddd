package video

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// sseHeartbeatInterval SSE 心跳间隔（与原实现的 20s 一致）。
	sseHeartbeatInterval = 20 * time.Second
	// maxTranscodeProgressIDs 单次进度查询的视频数量上限。
	maxTranscodeProgressIDs = 100
)

// SSE 事件名（属外部契约，与原实现逐字一致）。
const (
	sseEventError    = "error"
	sseEventSnapshot = "snapshot"
	sseEventProgress = "progress"
	sseEventDone     = "done"
	sseEventComplete = "complete"
	sseEventHeartbet = "heartbeat"
)

// AdminTranscodeProgressReq is query binding for progress snapshot / SSE.
type AdminTranscodeProgressReq struct {
	// VideoIDs comma-separated list, e.g. "1,2,3"
	VideoIDs string `form:"videoIds" binding:"required"`
	// Token optional; used by EventSource via query (also accepted by middleware).
	Token string `form:"token"`
}

// AdminTranscodeProgressView returns a one-shot progress snapshot (polling fallback).
func (h *AdminHandler) AdminTranscodeProgressView(c *gin.Context) {
	req := middlewares.GetBound[AdminTranscodeProgressReq](c)

	ids, err := parseVideoIDList(req.VideoIDs)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	items, err := h.progress.MGetByVideoIDs(c.Request.Context(), ids)
	if err != nil {
		h.log().Error("MGetTranscodeProgressByVideoIDs failed", zap.Error(err))
		res.FailWithMsg("读取转码进度失败", c)

		return
	}

	res.OkWithData(gin.H{"items": items}, c)
}

// AdminTranscodeProgressStreamView streams progress via SSE (server polls Redis).
func (h *AdminHandler) AdminTranscodeProgressStreamView(c *gin.Context) {
	req := middlewares.GetBound[AdminTranscodeProgressReq](c)

	ids, err := parseVideoIDList(req.VideoIDs)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		res.FailWithMsg("streaming unsupported", c)

		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	ctx := c.Request.Context()

	subscription, err := h.progress.Subscribe(ctx)
	if err != nil {
		writeSSE(c, flusher, sseEventError, gin.H{"message": "订阅转码进度失败"})

		return
	}

	defer func() { _ = subscription.Close() }()

	events := subscription.Events()

	// Initial snapshot.
	items, err := h.progress.MGetByVideoIDs(ctx, ids)
	if err != nil {
		writeSSE(c, flusher, sseEventError, gin.H{"message": "读取转码进度失败"})

		return
	}

	writeSSE(c, flusher, sseEventSnapshot, gin.H{"items": items})

	allowed := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}

	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			writeSSE(c, flusher, sseEventHeartbet, gin.H{"ts": time.Now().Unix()})
		case item, ok := <-events:
			if !ok {
				return
			}

			if _, allowedID := allowed[item.VideoID]; !allowedID {
				continue
			}

			writeSSE(c, flusher, sseEventProgress, item)

			if !h.progress.IsTerminalStatus(item.Status) {
				continue
			}

			writeSSE(c, flusher, sseEventDone, item)

			current, curErr := h.progress.MGetByVideoIDs(ctx, ids)
			if curErr == nil && h.allProgressTerminal(current, ids) {
				writeSSE(c, flusher, sseEventComplete, gin.H{"ok": true})
			}
		}
	}
}

// allProgressTerminal 判断给定视频的进度是否全部到达终态且无缺失。
func (h *AdminHandler) allProgressTerminal(items []app.TranscodeProgressItem, ids []uint) bool {
	if len(items) == 0 || len(ids) == 0 {
		return false
	}

	present := make(map[uint]bool, len(ids))

	for _, item := range items {
		present[item.VideoID] = true

		if !h.progress.IsTerminalStatus(item.Status) {
			return false
		}
	}

	for _, id := range ids {
		if !present[id] {
			return false
		}
	}

	return true
}

// parseVideoIDList 解析逗号分隔的 videoIds（去重、校验上限，文案与原实现逐字一致）。
func parseVideoIDList(raw string) ([]uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("videoIds 不能为空")
	}

	parts := strings.Split(raw, ",")
	ids := make([]uint, 0, len(parts))
	seen := make(map[uint]struct{}, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil || v == 0 {
			return nil, fmt.Errorf("无效的 videoId: %s", p)
		}

		id := uint(v)
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("videoIds 不能为空")
	}

	if len(ids) > maxTranscodeProgressIDs {
		return nil, fmt.Errorf("videoIds 最多 %d 个", maxTranscodeProgressIDs)
	}

	return ids, nil
}

// writeSSE 写一条 SSE 事件并立即 flush（格式与原实现逐字一致）。
func writeSSE(c *gin.Context, flusher http.Flusher, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	if event != "" {
		_, _ = fmt.Fprintf(c.Writer, "event: %s\n", event)
	}

	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()
}
