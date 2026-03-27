package identity

import (
	"context"
	"errors"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ identitydomain.SystemAccountRepository = (*UserRepository)(nil)

func (r *UserRepository) FindByUsernameForUpdate(ctx context.Context, username string) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ?", username).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(&row), nil
}

func (r *UserRepository) CreateSystemAccount(ctx context.Context, user *identitydomain.User) (*identitydomain.User, error) {
	row := toPOUser(user)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return nil, err
	}
	return toDomainUser(row), nil
}

func (r *UserRepository) SaveSystemAccount(ctx context.Context, user *identitydomain.User) error {
	return gormx.DB(ctx, r.db).Model(&po.User{}).Where("id = ?", user.ID()).Updates(map[string]any{
		"password": user.Password(),
		"level":    user.Level(),
		"exp":      user.Exp(), "current_role_id": user.CurrentRoleID(),
	}).Error
}
