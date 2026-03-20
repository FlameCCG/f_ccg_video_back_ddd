package comment

import (
	"context"
	"fmt"

	commententity "f_ccg_video_back_ddd/internal/domain/comment/entity"
	commentrepo "f_ccg_video_back_ddd/internal/domain/comment/repository"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// 通知文案。
const (
	noticeTitleVideoComment   = "有人评论了你的视频"
	noticeTitleDynamicComment = "有人评论了你的动态"
	noticeTitleReply          = "有人回复了你"
	noticeTitleAtInComment    = "在评论中@了我"
	noticeTitleAtInDynamic    = "在动态评论中@了我"
	noticeTitleLikeComment    = "有人点赞了你的评论"
)

// scheduleLikeNotification 异步发送评论点赞通知。
func (s *NotificationDispatcher) scheduleLikeNotification(ctx context.Context, actionUserID, commentID uint) {
	s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
		s.sendCommentLikeNotification(asyncCtx, actionUserID, commentID)
	})
}

// sendCommentCreateNotifications 通知视频作者/被回复者。
func (s *NotificationDispatcher) sendCommentCreateNotifications(
	ctx context.Context,
	actionUser *UserProfile,
	video *VideoBrief,
	c *commententity.Comment,
	replyTo *ReplyInfo,
) {
	if actionUser == nil || video == nil || c == nil {
		return
	}

	receivers := newNoticeSet()
	// 视频作者
	if video.AuthorUsername != "" {
		author, err := s.related.UserProfileByUsername(ctx, video.AuthorUsername)
		if err == nil && author != nil && author.ID != actionUser.ID {
			receivers.put(author.ID, Notice{
				Type:             enum.NotificationApplyType,
				ReceiverID:       int(author.ID),
				ActionUserID:     actionUser.ID,
				ActionUserAvatar: actionUser.Avatar,
				ActionUserName:   actionUser.Username,
				Title:            noticeTitleVideoComment,
				Content:          c.Content,
				VideoID:          video.ID,
				VideoTitle:       video.Title,
				CommentID:        c.ID,
			})
		}
	}
	// 被回复者
	if replyTo != nil && replyTo.UserID > 0 && replyTo.UserID != actionUser.ID {
		receivers.put(replyTo.UserID, Notice{
			Type:             enum.NotificationApplyType,
			ReceiverID:       int(replyTo.UserID),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleReply,
			Content:          c.Content,
			VideoID:          video.ID,
			VideoTitle:       video.Title,
			CommentID:        c.ID,
		})
	}

	s.dispatchNotices(ctx, receivers.list(), "create comment notifications failed")
}

// sendDynamicCommentCreateNotifications 通知动态作者/被回复者。
func (s *NotificationDispatcher) sendDynamicCommentCreateNotifications(
	ctx context.Context,
	actionUser *UserProfile,
	dynamic *DynamicBrief,
	c *commententity.Comment,
	replyTo *ReplyInfo,
) {
	if actionUser == nil || dynamic == nil || c == nil {
		return
	}

	link := dynamicLink(dynamic.ID)

	receivers := newNoticeSet()
	// 动态作者
	if dynamic.UserID > 0 && dynamic.UserID != actionUser.ID {
		receivers.put(dynamic.UserID, Notice{
			Type:             enum.NotificationApplyType,
			ReceiverID:       int(dynamic.UserID),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleDynamicComment,
			Content:          c.Content,
			Link:             link,
			CommentID:        c.ID,
		})
	}
	// 被回复者
	if replyTo != nil && replyTo.UserID > 0 && replyTo.UserID != actionUser.ID {
		receivers.put(replyTo.UserID, Notice{
			Type:             enum.NotificationApplyType,
			ReceiverID:       int(replyTo.UserID),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleReply,
			Content:          c.Content,
			Link:             link,
			CommentID:        c.ID,
		})
	}

	s.dispatchNotices(ctx, receivers.list(), "create comment notifications failed")
}

// sendAtNotifications 视频评论中的 @ 通知。
func (s *NotificationDispatcher) sendAtNotifications(
	ctx context.Context,
	actionUser *UserProfile,
	video *VideoBrief,
	commentID uint,
	content string,
	atUserIDs []uint,
) {
	if len(atUserIDs) == 0 || actionUser == nil || video == nil {
		return
	}
	// 去重
	seen := make(map[uint]struct{})

	notifications := make([]Notice, 0, len(atUserIDs))

	for _, uid := range atUserIDs {
		if uid == 0 {
			continue
		}

		if _, ok := seen[uid]; ok {
			continue
		}

		seen[uid] = struct{}{}
		notifications = append(notifications, Notice{
			Type:             enum.NotificationAtType,
			ReceiverID:       int(uid),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleAtInComment,
			Content:          content,
			VideoID:          video.ID,
			VideoTitle:       video.Title,
			CommentID:        commentID,
		})
	}

	s.dispatchNotices(ctx, notifications, "create @ notifications failed")
}

// sendAtNotificationsForDynamic 动态评论中的 @ 通知。
func (s *NotificationDispatcher) sendAtNotificationsForDynamic(
	ctx context.Context,
	actionUser *UserProfile,
	dynamicID, commentID uint,
	content string,
	atUserIDs []uint,
) {
	if len(atUserIDs) == 0 || actionUser == nil || dynamicID == 0 {
		return
	}

	seen := make(map[uint]struct{})

	notifications := make([]Notice, 0, len(atUserIDs))

	for _, uid := range atUserIDs {
		if uid == 0 {
			continue
		}

		if _, ok := seen[uid]; ok {
			continue
		}

		seen[uid] = struct{}{}
		notifications = append(notifications, Notice{
			Type:             enum.NotificationAtType,
			ReceiverID:       int(uid),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleAtInDynamic,
			Content:          content,
			Link:             dynamicLink(dynamicID),
			CommentID:        commentID,
		})
	}

	s.dispatchNotices(ctx, notifications, "create @ notifications failed")
}

// sendCommentLikeNotification 点赞评论通知。
func (s *NotificationDispatcher) sendCommentLikeNotification(ctx context.Context, actionUserID, commentID uint) {
	if actionUserID == 0 || commentID == 0 {
		return
	}

	actionUser, err := s.related.UserProfile(ctx, actionUserID)
	if err != nil || actionUser == nil {
		return
	}

	source, err := s.comments.FindNoticeSource(ctx, commentID)
	if err != nil || source == nil {
		return
	}

	if source.UserID == actionUserID {
		return
	}

	var (
		videoID    uint
		videoTitle string
	)

	if source.VideoID > 0 {
		// 视频信息读取失败时留空，不阻断通知。
		if video, videoErr := s.related.VideoBrief(ctx, source.VideoID); videoErr == nil && video != nil {
			videoID = video.ID
			videoTitle = video.Title
		}
	}

	link := ""
	if source.DynamicID > 0 {
		link = dynamicLink(source.DynamicID)
	}

	notice := Notice{
		Type:             enum.NotificationLikeCommentType,
		ReceiverID:       int(source.UserID),
		ActionUserID:     actionUser.ID,
		ActionUserAvatar: actionUser.Avatar,
		ActionUserName:   actionUser.Username,
		Title:            noticeTitleLikeComment,
		Content:          source.Content,
		Link:             link,
		VideoID:          videoID,
		VideoTitle:       videoTitle,
		CommentID:        source.ID,
	}

	if err := s.notifier.CreateNotices(ctx, []Notice{notice}); err != nil {
		s.logger.Warn("create comment like notification failed", zap.Error(err))

		return
	}

	s.notifier.InvalidateCount(notice.ReceiverID)
}

// dispatchNotices 批量落库通知并按去重后的接收者失效角标缓存。
func (s *NotificationDispatcher) dispatchNotices(ctx context.Context, notices []Notice, failMsg string) {
	if len(notices) == 0 {
		return
	}

	if err := s.notifier.CreateNotices(ctx, notices); err != nil {
		s.logger.Warn(failMsg, zap.Error(err))

		return
	}

	seen := make(map[int]struct{}, len(notices))

	for _, n := range notices {
		if _, ok := seen[n.ReceiverID]; ok {
			continue
		}

		seen[n.ReceiverID] = struct{}{}
		s.notifier.InvalidateCount(n.ReceiverID)
	}
}

// dynamicLink 动态详情跳转链接。
func dynamicLink(dynamicID uint) string {
	return fmt.Sprintf("/dynamic?dynamicId=%d", dynamicID)
}

// noticeSet 按接收者去重的通知集合，保持插入顺序、后写覆盖先写。
//
// 语义等于 map[uint]UserGlobalNotification（同一接收者只发一条，
// 「有人回复了你」覆盖「有人评论了你的视频/动态」），但落库顺序确定。
type noticeSet struct {
	order   []uint
	notices map[uint]Notice
}

func newNoticeSet() *noticeSet {
	return &noticeSet{notices: make(map[uint]Notice, 2)}
}

func (s *noticeSet) put(receiverID uint, notice Notice) {
	if _, ok := s.notices[receiverID]; !ok {
		s.order = append(s.order, receiverID)
	}

	s.notices[receiverID] = notice
}

func (s *noticeSet) list() []Notice {
	list := make([]Notice, 0, len(s.order))
	for _, id := range s.order {
		list = append(list, s.notices[id])
	}

	return list
}

// NotificationDispatcher 负责通知组装、去重与派发。
type NotificationDispatcher struct {
	comments commentrepo.Repository
	related  RelatedQuery
	notifier Notifier
	tasks    *ctxutil.DetachedTaskGroup
	logger   *zap.Logger
}

func NewNotificationDispatcher(comments commentrepo.Repository, related RelatedQuery, notifier Notifier, tasks *ctxutil.DetachedTaskGroup, logger *zap.Logger) *NotificationDispatcher {
	return &NotificationDispatcher{comments: comments, related: related, notifier: notifier, tasks: tasks, logger: logger}
}

// scheduleCreateNotifications 捕获通知快照，并由托管任务组派发评论、回复与 @ 通知。
func (s *NotificationDispatcher) scheduleCreateNotifications(ctx context.Context, actor UserProfile, video *VideoBrief, dynamic *DynamicBrief, c *commententity.Comment, replyTo *ReplyInfo) {
	userSnapshot := actor
	commentSnapshot := *c

	switch {
	case video != nil:
		videoSnapshot := *video

		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.sendCommentCreateNotifications(asyncCtx, &userSnapshot, &videoSnapshot, &commentSnapshot, replyTo)
		})
	case dynamic != nil:
		dynamicSnapshot := *dynamic

		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.sendDynamicCommentCreateNotifications(asyncCtx, &userSnapshot, &dynamicSnapshot, &commentSnapshot, replyTo)
		})
	}

	// 发送 @ 通知
	if len(c.AtUserIDs) > 0 {
		atUserIDs := c.AtUserIDs
		content := c.Content

		switch {
		case video != nil:
			videoSnapshot := *video

			s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
				s.sendAtNotifications(asyncCtx, &userSnapshot, &videoSnapshot, commentSnapshot.ID, content, atUserIDs)
			})
		case dynamic != nil:
			dynamicIDSnapshot := dynamic.ID

			s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
				s.sendAtNotificationsForDynamic(asyncCtx, &userSnapshot, dynamicIDSnapshot, commentSnapshot.ID, content, atUserIDs)
			})
		}
	}

}
