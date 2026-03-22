package identity

import (
	"context"
	"errors"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// 编译期断言：满足领域侧账号仓储接口。
var _ identitydomain.UserAccountRepository = (*UserRepository)(nil)

// ExistsByUsernameOrEmail 判断用户名或邮箱是否已被占用。
func (r *UserRepository) ExistsByUsernameOrEmail(ctx context.Context, username, email string) (bool, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Take(&row, "username = ? or email = ?", username, email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// FindByUsernameOrEmail 按「用户名或邮箱」登录标识读取用户；不存在时返回 (nil, nil)。
func (r *UserRepository) FindByUsernameOrEmail(ctx context.Context, identifier string) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Take(&row, "username = ? or email = ?", identifier, identifier).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainUser(&row), nil
}

// FindByEmail 按邮箱读取用户；不存在时返回 (nil, nil)。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Take(&row, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainUser(&row), nil
}

// FindByOpenIDAndSource 按第三方 openID + 注册来源读取用户；不存在时返回 (nil, nil)。
func (r *UserRepository) FindByOpenIDAndSource(
	ctx context.Context,
	openID, registerSource string,
) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Take(&row, "open_id = ? AND register_source = ?", openID, registerSource).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainUser(&row), nil
}

// CountByUsernameExcluding 统计除 excludeUserID 之外使用该用户名的账号数。
func (r *UserRepository) CountByUsernameExcluding(
	ctx context.Context,
	username string,
	excludeUserID uint,
) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("username = ? AND id != ?", username, excludeUserID).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// CountByEmailExcluding 统计除 excludeUserID 之外使用该邮箱的账号数。
func (r *UserRepository) CountByEmailExcluding(
	ctx context.Context,
	email string,
	excludeUserID uint,
) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("email = ? AND id != ?", email, excludeUserID).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Create 创建用户并写入 user_roles 关联，返回持久化后的用户。
//
// 要求在事务内调用，并在同一事务中调用 InitializeDefaults。
func (r *UserRepository) Create(ctx context.Context, user *identitydomain.User, roleID uint) (*identitydomain.User, error) {
	if user == nil {
		return nil, identitydomain.ErrUserNotFound
	}

	row := toPOUser(user)

	db := gormx.DB(ctx, r.db)
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}

	if err := db.Create(&po.UserRole{
		UserID: row.ID,
		RoleID: roleID,
	}).Error; err != nil {
		return nil, err
	}

	return toDomainUser(row), nil
}

// DeleteByID 删除用户（Casbin 绑定失败后的补偿路径）。
func (r *UserRepository) DeleteByID(ctx context.Context, userID uint) error {
	return gormx.DB(ctx, r.db).Delete(&po.User{}, userID).Error
}

// SaveCredentials 只保存领域动作改变的凭据列。
func (r *UserRepository) SaveCredentials(ctx context.Context, user *identitydomain.User) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", user.ID()).
		Updates(map[string]any{"password": user.Password(), "refresh_token_hash": user.RefreshTokenHash(), "must_change_password": user.MustChangePassword()}).Error
}

// SaveEmail 只保存绑定邮箱。
func (r *UserRepository) SaveEmail(ctx context.Context, user *identitydomain.User) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", user.ID()).
		Update("email", user.Email()).Error
}

// SaveCurrentRole 只保存当前角色 ID。
func (r *UserRepository) SaveCurrentRole(ctx context.Context, user *identitydomain.User) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", user.ID()).
		Update("current_role_id", user.CurrentRoleID()).Error
}

// UpdateProfile 部分更新用户资料。
func (r *UserRepository) UpdateProfile(
	ctx context.Context,
	userID uint,
	changes identitydomain.ProfileChanges,
) error {
	updates := profileUpdateColumns(changes)
	if len(updates) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
}

// profileUpdateColumns 把领域侧的部分更新集合翻译为列名 → 值。
func profileUpdateColumns(changes identitydomain.ProfileChanges) map[string]any {
	updates := make(map[string]any, 5)

	if changes.Username != nil {
		updates["username"] = *changes.Username
	}

	if changes.Avatar != nil {
		updates["avatar"] = *changes.Avatar
	}

	if changes.Description != nil {
		updates["description"] = *changes.Description
	}

	if changes.Gender != nil {
		updates["gender"] = *changes.Gender
	}

	if changes.Birthday != nil {
		updates["birthday"] = *changes.Birthday
	}

	return updates
}
