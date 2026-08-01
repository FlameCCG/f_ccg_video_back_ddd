package dynamic

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

// Service 动态命令用例服务（原 service/dynamic_service/dynamic_service.go）。
type Service struct {
	txm       tx.Manager
	dynamics  domain.DynamicRepository
	users     UserProfileQuery
	videos    VideoOwnershipQuery
	videoPins VideoPinWriter
	comments  DynamicCommentPurger
	logger    *zap.Logger
}

// NewService 构造动态命令用例服务。
func NewService(
	txm tx.Manager,
	dynamics domain.DynamicRepository,
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
	}
}

// Create 发布一条动态，并返回含作者资料的列表项。
func (s *Service) Create(ctx context.Context, userID uint, content, imageURL string) (*DynamicItem, error) {
	d, err := domain.NewDynamic(userID, content, imageURL)
	if err != nil {
		return nil, err
	}

	profile, err := s.users.ProfileByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if profile == nil {
		return nil, domain.ErrRecordNotFound
	}

	if err := s.dynamics.Create(ctx, d); err != nil {
		return nil, err
	}

	return &DynamicItem{
		ID:        d.ID,
		UserID:    userID,
		Username:  profile.Username,
		Avatar:    profile.Avatar,
		Content:   d.Content,
		ImageURL:  d.ImageURL,
		IsPinned:  d.IsPinned,
		PinnedAt:  d.PinnedAt,
		CreatedAt: d.CreatedAt,
	}, nil
}

// Delete 删除自己的动态，并在同一事务内级联清理其评论与评论点赞。
func (s *Service) Delete(ctx context.Context, userID, dynamicID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if dynamicID == 0 {
		return domain.ErrDynamicIDRequired
	}

	d, err := s.dynamics.FindByID(ctx, dynamicID)
	if err != nil {
		return err
	}

	if d == nil {
		return domain.ErrDynamicNotFound
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
func (s *Service) Pin(ctx context.Context, userID, dynamicID uint, pinned bool) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if dynamicID == 0 {
		return domain.ErrDynamicIDRequired
	}

	d, err := s.dynamics.FindByID(ctx, dynamicID)
	if err != nil {
		return err
	}

	if d == nil {
		return domain.ErrDynamicNotFound
	}

	if err := d.EnsurePinnableBy(userID); err != nil {
		return err
	}

	plan := domain.BuildPinPlan(pinned)
	now := time.Now()

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		if plan.ClearOwnPinned {
			// 与原实现一致：作者名缺失时跳过视频侧清理，只清动态侧。
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
			return s.dynamics.SetPinned(txCtx, dynamicID, &now)
		}

		return s.dynamics.ClearPinned(txCtx, dynamicID)
	})
}

// PinVideo 置顶/取消置顶视频：与动态置顶互斥（同一用户同一时间只允许置顶一个作品）。
func (s *Service) PinVideo(ctx context.Context, userID, videoID uint, pinned bool) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}

	profile, err := s.users.ProfileByID(ctx, userID)
	if err != nil {
		return err
	}

	if profile == nil {
		return domain.ErrRecordNotFound
	}

	if profile.Username == "" {
		return domain.ErrUserNotFound
	}

	owned, err := s.videos.ExistsPublishedByAuthor(ctx, videoID, profile.Username)
	if err != nil {
		return err
	}

	if !owned {
		return domain.ErrVideoPinForbidden
	}

	plan := domain.BuildPinPlan(pinned)
	now := time.Now()

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		if plan.ClearOwnPinned {
			if err := s.dynamics.ClearPinnedByUser(txCtx, userID); err != nil {
				return err
			}

			if err := s.videoPins.ClearPinnedByAuthor(txCtx, profile.Username); err != nil {
				return err
			}
		}

		if plan.Pinned {
			return s.videoPins.SetPinned(txCtx, videoID, profile.Username, &now)
		}

		return s.videoPins.ClearPinned(txCtx, videoID, profile.Username)
	})
}
