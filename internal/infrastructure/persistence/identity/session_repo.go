package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// SetRefreshToken stores a refresh digest if the password version still matches.
func (r *UserRepository) SetRefreshToken(ctx context.Context, userID uint, passwordHash, tokenHash string) (bool, error) {
	result := gormx.DB(ctx, r.db).Model(&po.User{}).Where("id = ? AND password = ?", userID, passwordHash).Update("refresh_token_hash", tokenHash)
	return result.RowsAffected == 1, result.Error
}

// RotateRefreshToken atomically replaces the expected refresh digest.
func (r *UserRepository) RotateRefreshToken(ctx context.Context, userID uint, passwordHash, oldHash, newHash string) (bool, error) {
	result := gormx.DB(ctx, r.db).Model(&po.User{}).Where("id = ? AND password = ? AND refresh_token_hash = ?", userID, passwordHash, oldHash).Update("refresh_token_hash", newHash)
	return result.RowsAffected == 1, result.Error
}
