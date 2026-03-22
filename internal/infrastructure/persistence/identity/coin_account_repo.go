package identity

import (
	"context"
	"errors"
	"slices"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LockAccounts 是账户所有者提供的锁定能力，不让视频仓储直接操作 users。
func (r *CoinAccountRepository) LockAccounts(ctx context.Context, userIDs []uint) error {
	ordered := slices.Clone(userIDs)
	slices.Sort(ordered)
	ordered = slices.Compact(ordered)
	for _, id := range ordered {
		if id == 0 {
			continue
		}
		if err := gormx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Take(&po.User{}, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return identitydomain.ErrUserRecordNotFound
			}
			return err
		}
	}
	return nil
}

// FindForUpdate 在调用方事务中锁定并重建硬币账户。
func (r *CoinAccountRepository) FindForUpdate(ctx context.Context, userID uint) (*identitydomain.CoinAccount, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "coin_count").
		Take(&row, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	return identitydomain.RestoreCoinAccount(row.ID, row.CoinCount), nil
}

// SaveBalance 仅保存账户余额，不覆盖用户资料和经验。
func (r *CoinAccountRepository) SaveBalance(ctx context.Context, account *identitydomain.CoinAccount) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", account.UserID()).
		Update("coin_count", account.Balance()).Error
}

// CoinAccountRepository 保存账户状态，不加载 User 资料。
type CoinAccountRepository struct{ db *gorm.DB }

var _ identitydomain.CoinAccountRepository = (*CoinAccountRepository)(nil)

func NewCoinAccountRepository(db *gorm.DB) *CoinAccountRepository {
	return &CoinAccountRepository{db: db}
}
