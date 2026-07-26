package platform

import (
	"context"
	"encoding/json"
	"fmt"

	domain "f_ccg_video_back_ddd/internal/domain/platform"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

// CommentLikeSyncService 评论点赞增量落库用例（原 service/cron_service/sync_comment_like.go）。
//
// 批内按 (commentId,userId) 合并为最后意图，再相对 DB 行状态写一次（count ±0/±1）。
type CommentLikeSyncService struct {
	txm        tx.Manager
	queue      CommentLikeQueue
	projection CommentLikeProjection
}

// NewCommentLikeSyncService 构造评论点赞增量落库用例服务。
func NewCommentLikeSyncService(
	txm tx.Manager,
	queue CommentLikeQueue,
	projection CommentLikeProjection,
) *CommentLikeSyncService {
	return &CommentLikeSyncService{txm: txm, queue: queue, projection: projection}
}

// Run 循环消费评论点赞增量队列，直到队列取空或出现失败。
//
// 失败语义与原实现一致：
//   - 取批失败：记录警告并结束本轮；
//   - 整批都是坏消息：直接确认丢弃，避免堵死队列；
//   - 落库失败：不确认，processing 队列保留，下轮整批重试（apply 相对行状态幂等）；
//   - 确认失败：记录警告并结束本轮。
func (s *CommentLikeSyncService) Run(ctx context.Context) {
	if !s.queue.Available() {
		zap.L().Warn("skip SyncCommentLike: redis not configured")

		return
	}

	// Cron 任务是调度 root 场景，没有请求上下文。
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		items, err := s.queue.NextBatch(ctx)
		if err != nil {
			zap.L().Warn("load comment like batch failed", zap.Error(err))

			return
		}

		if len(items) == 0 {
			return
		}

		intents, decodeOK := domain.FoldCommentLikeIntents(decodeCommentLikeEvents(items))
		if !decodeOK {
			// 整批都是坏消息：直接 ack 丢掉，避免堵死
			if err := s.queue.Ack(ctx, len(items)); err != nil {
				zap.L().Warn("ack comment like batch failed", zap.Error(err))
			}

			continue
		}

		if err := s.applyIntents(ctx, intents); err != nil {
			zap.L().Warn("apply comment like intents failed", zap.Error(err))
			// 不 ack：processing 保留，下次整批重试（apply 相对行状态幂等）
			return
		}

		if err := s.queue.Ack(ctx, len(items)); err != nil {
			zap.L().Warn("ack comment like batch failed", zap.Int("batchSize", len(items)), zap.Error(err))

			return
		}
	}
}

// decodeCommentLikeEvents 按入队顺序解码原始消息，解码失败的条目记录警告后丢弃。
func decodeCommentLikeEvents(items []string) []domain.CommentLikeEvent {
	events := make([]domain.CommentLikeEvent, 0, len(items))

	for _, raw := range items {
		var evt domain.CommentLikeEvent
		if err := json.Unmarshal([]byte(raw), &evt); err != nil {
			zap.L().Warn("unmarshal comment like event failed", zap.Error(err))

			continue
		}

		events = append(events, evt)
	}

	return events
}

// applyIntents 逐条落库最终意图，任一条失败即终止本批并携带定位信息返回。
func (s *CommentLikeSyncService) applyIntents(ctx context.Context, intents map[domain.CommentLikeIntentKey]string) error {
	for key, action := range intents {
		evt := domain.CommentLikeEvent{
			CommentID: key.CommentID,
			UserID:    key.UserID,
			Action:    action,
		}
		if err := s.applyFinalIntent(ctx, evt); err != nil {
			return fmt.Errorf("commentId=%d userId=%d action=%s: %w", key.CommentID, key.UserID, action, err)
		}
	}

	return nil
}

// applyFinalIntent 按最终意图相对 DB 行状态在一个事务内写一次：
// like + 无行 → INSERT + count+1；unlike + 有行 → DELETE + count-1；其余 noop。
// 评论已删除时直接跳过，避免堵队列 / 复活孤儿 comment_likes。
func (s *CommentLikeSyncService) applyFinalIntent(ctx context.Context, evt domain.CommentLikeEvent) error {
	return s.txm.Do(ctx, func(txCtx context.Context) error {
		exists, err := s.projection.CommentExists(txCtx, evt.CommentID)
		if err != nil {
			return err
		}

		if !exists {
			return nil
		}

		switch evt.Action {
		case domain.LikeActionLike:
			inserted, insertErr := s.projection.InsertLike(txCtx, evt.CommentID, evt.UserID)
			if insertErr != nil {
				return insertErr
			}

			if !inserted {
				// 已有行：最终已赞，count 不动
				return nil
			}

			return s.projection.IncrLikeCount(txCtx, evt.CommentID)
		case domain.LikeActionUnlike:
			deleted, deleteErr := s.projection.DeleteLike(txCtx, evt.CommentID, evt.UserID)
			if deleteErr != nil {
				return deleteErr
			}

			if !deleted {
				// 本无行：最终未赞，count 不动
				return nil
			}

			return s.projection.DecrLikeCount(txCtx, evt.CommentID)
		default:
			return nil
		}
	})
}
