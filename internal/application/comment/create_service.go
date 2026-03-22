package comment

import (
	"context"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"golang.org/x/sync/errgroup"
)

// Create 发布评论。
//
// 先用领域 Draft 规范化业务意图，再经查询端口并发取得判断需要的事实；这些预取不构成数据库快照事务。
// 回复保存与根回复数由仓储事务覆盖，视频评论数和通知有自己的更新步骤，排障应按完成阶段查看。
//
// 流程并发预取（评论者 / 评论对象 / @ 名单）→
// 回复时校验父评论并推导 rootID → 落库（回复与根评论回复数同事务）→
// 同步更新视频评论数 → 异步热度打点与通知。
func (s *CreateService) Create(ctx context.Context, cmd CreateCommand) (*CommentItem, error) {
	draft, _, err := (commentdomain.Draft{
		UserID:        cmd.UserID,
		VideoID:       cmd.VideoID,
		DynamicID:     cmd.DynamicID,
		Content:       cmd.Content,
		ParentID:      cmd.ParentID,
		AtUserIDs:     cmd.AtUserIDs,
		Pictures:      cmd.Pictures,
		CommenterIP:   cmd.IP,
		CommenterAddr: cmd.Addr,
	}).Normalize()
	if err != nil {
		return nil, err
	}

	var (
		video      *VideoBrief
		dynamic    *DynamicBrief
		actor      = UserProfile{ID: draft.UserID}
		filteredAt []uint
	)

	candidates := draft.NormalizeAtUserIDs()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		profile, err := s.related.UserProfile(groupCtx, draft.UserID)
		if err != nil {
			return err
		}
		// 用户记录缺失时保留最小 actor，避免因缓存过期账号阻塞发评
		if profile != nil {
			actor = *profile
		}

		return nil
	})

	if draft.VideoID > 0 {
		group.Go(func() error {
			brief, err := s.related.PublishedVideoBrief(groupCtx, draft.VideoID)
			if err != nil {
				return err
			}

			if brief == nil {
				return commentdomain.ErrVideoNotPublished
			}

			video = brief

			return nil
		})
	} else {
		group.Go(func() error {
			brief, err := s.related.DynamicBrief(groupCtx, draft.DynamicID)
			if err != nil {
				return err
			}

			if brief == nil {
				return commentdomain.ErrDynamicNotFound
			}

			dynamic = brief

			return nil
		})
	}

	if len(candidates) > 0 {
		group.Go(func() error {
			filteredAt = s.related.AllowedAtUserIDs(groupCtx, draft.UserID, candidates)

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	draft.AtUserIDs = filteredAt

	// 如果是回复，检查父评论是否存在并自动计算 rootID
	var (
		replyTo *ReplyInfo
		rootID  *uint
	)

	if draft.HasParent() {
		parent, err := s.query.ParentBrief(ctx, *draft.ParentID, draft.VideoID, draft.DynamicID)
		if err != nil {
			return nil, err
		}

		if parent == nil {
			return nil, commentdomain.ErrParentNotFound
		}

		replyTo = &ReplyInfo{
			UserID:   parent.UserID,
			Username: parent.Username,
		}
		rootID = parent.RootForReply(*draft.ParentID)
	}

	entity, err := commentdomain.NewComment(draft, rootID)
	if err != nil {
		return nil, err
	}

	if rootID != nil && *rootID > 0 {
		if err := s.txm.Do(ctx, func(txCtx context.Context) error {
			created, err := s.comments.Create(txCtx, entity)
			if err != nil {
				return err
			}

			entity = created

			return s.comments.IncrReplyCount(txCtx, *rootID)
		}); err != nil {
			return nil, err
		}
	} else {
		created, err := s.comments.Create(ctx, entity)
		if err != nil {
			return nil, err
		}

		entity = created
	}

	if draft.VideoID > 0 {
		// 评论数同步更新，避免进程崩溃导致计数丢失；热度可异步
		s.videos.UpdateCommentCount(ctx, draft.VideoID, 1)

		videoIDSnapshot := draft.VideoID
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.hotScore.RecordHotScoreForComment(asyncCtx, videoIDSnapshot, 1)
		})
	}

	s.notifications.scheduleCreateNotifications(ctx, actor, video, dynamic, entity, replyTo)

	return &CommentItem{
		ID:            entity.ID(),
		Content:       entity.Content(),
		UserID:        entity.UserID(),
		Username:      actor.Username,
		Avatar:        actor.Avatar,
		VideoID:       entity.VideoID(),
		DynamicID:     entity.DynamicID(),
		ParentID:      entity.ParentID(),
		RootID:        entity.RootID(),
		AtUserIDs:     entity.AtUserIDs(),
		Pictures:      entity.Pictures(),
		LikeCount:     0,
		ReplyCount:    0,
		CommenterAddr: entity.CommenterAddr(),
		CreatedAt:     entity.CreatedAt(),
		IsLiked:       false,
		ReplyTo:       replyTo,
	}, nil
}

// CreateService 负责创建评论与回复。
type CreateService struct {
	txm           tx.Manager
	comments      commentdomain.Repository
	query         Query
	related       RelatedQuery
	videos        VideoCommentCounter
	hotScore      VideoHotScoreRecorder
	tasks         *ctxutil.DetachedTaskGroup
	notifications *NotificationDispatcher
}

func NewCreateService(txm tx.Manager, comments commentdomain.Repository, query Query, related RelatedQuery, videos VideoCommentCounter, hotScore VideoHotScoreRecorder, tasks *ctxutil.DetachedTaskGroup, notifications *NotificationDispatcher) *CreateService {
	return &CreateService{txm: txm, comments: comments, query: query, related: related, videos: videos, hotScore: hotScore, tasks: tasks, notifications: notifications}
}
