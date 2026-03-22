package video

import (
	"context"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// DefaultFolderService 提供初始化所需的最小收藏夹用例，不要求 Redis 或 HTTP 装配。
type DefaultFolderService struct{ folders videodomain.FolderRepository }

func NewDefaultFolderService(folders videodomain.FolderRepository) *DefaultFolderService {
	return &DefaultFolderService{folders: folders}
}

func (s *DefaultFolderService) InitializeDefaultFolder(ctx context.Context, userID uint) error {
	if userID == 0 {
		return videodomain.ErrUserNotLogin
	}
	folder, err := s.folders.FindDefault(ctx, userID)
	if err != nil {
		return err
	}
	if folder != nil {
		return nil
	}
	_, err = s.folders.Create(ctx, videodomain.NewDefaultFavoriteFolder(userID))
	return err
}
