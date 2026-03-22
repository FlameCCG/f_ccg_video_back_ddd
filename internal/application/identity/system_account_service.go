package identity

import (
	"context"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"fmt"
)

// SystemAccountService 在调用方的一次性初始化事务中执行账号领域动作。
type SystemAccountService struct {
	users   identitydomain.SystemAccountRepository
	folders DefaultFolderInitializer
}

func NewSystemAccountService(users identitydomain.SystemAccountRepository, folders DefaultFolderInitializer) *SystemAccountService {
	return &SystemAccountService{users: users, folders: folders}
}

func (s *SystemAccountService) UpsertSuperUser(ctx context.Context, username, passwordHash string, roleID uint) (uint, bool, error) {
	user, err := s.users.FindByUsernameForUpdate(ctx, username)
	if err != nil {
		return 0, false, fmt.Errorf("查询超级管理员用户失败: %w", err)
	}
	if user != nil {
		user.ProvisionAdministrator(passwordHash, roleID)
		if err := s.users.SaveSystemAccount(ctx, user); err != nil {
			return 0, false, fmt.Errorf("更新超级管理员用户密码失败: %w", err)
		}
		return user.ID(), false, nil
	}
	user, err = s.users.CreateSystemAccount(ctx, identitydomain.NewSystemUser(username, passwordHash, roleID))
	if err != nil {
		return 0, false, fmt.Errorf("创建超级管理员用户失败: %w", err)
	}
	if err := s.users.InitializeDefaults(ctx, user.ID()); err != nil {
		return 0, false, err
	}
	if err := s.folders.InitializeDefaultFolder(ctx, user.ID()); err != nil {
		return 0, false, err
	}
	return user.ID(), true, nil
}
