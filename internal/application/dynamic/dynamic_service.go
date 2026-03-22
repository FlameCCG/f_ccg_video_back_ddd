package dynamic

import (
	"context"
	"time"

	dynamicdomain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

// Service 动态命令用例服务。
type Service struct {
	txm       tx.Manager
	dynamics  dynamicdomain.DynamicRepository
	users     UserProfileQuery
	videos    VideoOwnershipQuery
	videoPins VideoPinWriter
	comments  DynamicCommentPurger
	logger    *zap.Logger
	pins      dynamicdomain.PinPolicy
}

// NewService 构造动态命令用例服务。
func NewService(
	txm tx.Manager,
	dynamics dynamicdomain.DynamicRepository,
	users UserProfileQuery,
	videos VideoOwnershipQuery,
	videoPins VideoPinWriter,
	comments DynamicCommentPurger,
	logger *zap.Logger,
) *Service {
	return &Service{
		txm:       txm,
		dynamics:  dynamics,
		users:     users,
		videos:    videos,
		videoPins: videoPins,
		comments:  comments,
		logger:    logger,
		pins:      dynamicdomain.PinPolicy{},
	}
}

// Create 发布一条动态，并返回含作者资料的列表项。
func (s *Service) Create(ctx context.Context, userID uint, content, imageURL string) (*DynamicItem, error) {
	d, err := dynamicdomain.NewDynamic(userID, content, imageURL)
	if err != nil {
		return nil, err
	}

	profile, err := s.users.ProfileByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if profile == nil {
		return nil, dynamicdomain.ErrRecordNotFound
	}

	stored, err := s.dynamics.Create(ctx, d)
	if err != nil {
		return nil, err
	}

	return &DynamicItem{
		ID:        stored.ID(),
		UserID:    userID,
		Username:  profile.Username,
		Avatar:    profile.Avatar,
		Content:   stored.Content(),
		ImageURL:  stored.ImageURL(),
		IsPinned:  stored.IsPinned(),
		PinnedAt:  stored.PinnedAt(),
		CreatedAt: stored.CreatedAt(),
	}, nil
}

// Delete 删除自己的动态，并在同一事务内级联清理其评论与评论点赞。
func (s *Service) Delete(ctx context.Context, userID, dynamicID uint) error {
	if userID == 0 {
		return dynamicdomain.ErrUserNotLogin
	}

	if dynamicID == 0 {
		return dynamicdomain.ErrDynamicIDRequired
	}

	d, err := s.dynamics.FindByID(ctx, dynamicID)
	if err != nil {
		return err
	}

	if d == nil {
		return dynamicdomain.ErrDynamicNotFound
	}

	if err := d.EnsureDeletableBy(userID); err != nil {
		return err
	}

	var (
		deletedComments     int64
		deletedCommentLikes int64
	)

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		comments, commentLikes, purgeErr := s.comments.PurgeByDynamic(txCtx, dynamicID)
		if purgeErr != nil {
			return purgeErr
		}

		deletedComments, deletedCommentLikes = comments, commentLikes

		return s.dynamics.DeleteByID(txCtx, dynamicID)
	}); err != nil {
		return err
	}

	s.logger.Info("delete dynamic ok",
		zap.Uint("dynamicId", dynamicID),
		zap.Int64("commentDeleted", deletedComments),
		zap.Int64("commentLikeDeleted", deletedCommentLikes),
	)

	return nil
}

// Pin 置顶/取消置顶动态：与视频置顶互斥（同一用户同一时间只允许置顶一个作品）。
//
// 本上下文声明 VideoPinWriter 端口，在同一个事务 context 中清除视频置顶并更新动态。
// 当前同库适配器使这些写入可共同回滚；它不是仅靠跨上下文方法调用就获得的分布式事务。
func (s *Service) Pin(ctx context.Context, userID, dynamicID uint, pinned bool) error {
	if userID == 0 {
		return dynamicdomain.ErrUserNotLogin
	}

	if dynamicID == 0 {
		return dynamicdomain.ErrDynamicIDRequired
	}

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		d, err := s.dynamics.FindByIDForUpdate(txCtx, dynamicID)
		if err != nil {
			return err
		}
		if err := d.SetPinnedBy(userID, pinned, time.Now()); err != nil {
			return err
		}
		plan := s.pins.BuildPinPlan(d.IsPinned())

		if plan.ClearOwnPinned {
			// 作者名缺失时跳过视频侧清理，只清动态侧。
			username, usernameErr := s.users.UsernameByID(txCtx, userID)
			if usernameErr != nil {
				return usernameErr
			}

			if username != "" {
				if err := s.videoPins.ClearPinnedByAuthor(txCtx, username); err != nil {
					return err
				}
			}

			if err := s.dynamics.ClearPinnedByUser(txCtx, userID); err != nil {
				return err
			}
		}

		if plan.Pinned {
			return s.dynamics.SetPinned(txCtx, dynamicID, d.PinnedAt())
		}

		return s.dynamics.ClearPinned(txCtx, dynamicID)
	})
}

// PinVideo 置顶/取消置顶视频：与动态置顶互斥（同一用户同一时间只允许置顶一个作品）。
func (s *Service) PinVideo(ctx context.Context, userID, videoID uint, pinned bool) error {
	if userID == 0 {
		return dynamicdomain.ErrUserNotLogin
	}

	if videoID == 0 {
		return dynamicdomain.ErrVideoIDRequired
	}

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		profile, err := s.users.ProfileByID(txCtx, userID)
		if err != nil {
			return err
		}
		if profile == nil {
			return dynamicdomain.ErrRecordNotFound
		}
		owned := false
		if profile.Username != "" {
			owned, err = s.videos.ExistsPublishedByAuthorForUpdate(txCtx, videoID, profile.Username)
			if err != nil {
				return err
			}
		}
		if err := s.pins.EnsureVideoPinnable(profile.Username, owned); err != nil {
			return err
		}
		plan := s.pins.BuildPinPlan(pinned)
		now := time.Now()

		if plan.ClearOwnPinned {
			if err := s.dynamics.ClearPinnedByUser(txCtx, userID); err != nil {
				return err
			}

			if err := s.videoPins.ClearPinnedByAuthor(txCtx, profile.Username); err != nil {
				return err
			}
		}

		if plan.Pinned {
			owned, err = s.videoPins.SetPinned(txCtx, videoID, profile.Username, &now)
		} else {
			owned, err = s.videoPins.ClearPinned(txCtx, videoID, profile.Username)
		}
		if err != nil {
			return err
		}
		return s.pins.EnsureVideoPinnable(profile.Username, owned)
	})
}
