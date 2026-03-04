package video

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ScheduleStore 将发布计划保存在 MySQL，视频状态与计划共用同一事务。
// legacy 仅负责读取旧 Redis 任务，不参与新任务的成功条件。
type ScheduleStore struct {
	db     *gorm.DB
	legacy app.LegacyScheduleStore
}

var _ app.ScheduleStore = (*ScheduleStore)(nil)

func NewScheduleStore(db *gorm.DB, legacy app.LegacyScheduleStore) *ScheduleStore {
	return &ScheduleStore{db: db, legacy: legacy}
}

func (s *ScheduleStore) Available() bool { return s != nil && s.db != nil }

func (s *ScheduleStore) Add(ctx context.Context, videoID uint, when time.Time) error {
	return s.save(ctx, videoID, &when)
}

func (s *ScheduleStore) Remove(ctx context.Context, videoID uint) error {
	return s.save(ctx, videoID, nil)
}

func (s *ScheduleStore) save(ctx context.Context, videoID uint, when *time.Time) error {
	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}
	return gormx.DB(ctx, s.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"publish_at"}),
	}).Create(&po.VideoPublishSchedule{VideoID: videoID, PublishAt: when}).Error
}

func (s *ScheduleStore) Score(ctx context.Context, videoID uint) (float64, bool, error) {
	var row po.VideoPublishSchedule
	err := gormx.DB(ctx, s.db).Take(&row, "video_id = ?", videoID).Error
	if err == nil {
		if row.PublishAt == nil {
			return 0, false, nil
		}
		return float64(row.PublishAt.Unix()), true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, err
	}
	if s.legacy == nil || !s.legacy.Available() {
		return 0, false, nil
	}
	// 审核可能早于后台迁移执行。先持久化已知的旧计划，再做审核决策。
	score, ok, err := s.legacy.Score(ctx, videoID)
	if err != nil || !ok {
		return 0, false, err
	}
	if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 253402300799 {
		return 0, false, fmt.Errorf("invalid legacy publish time for video %d", videoID)
	}
	when := time.Unix(int64(score), 0)
	if err := gormx.DB(ctx, s.db).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&po.VideoPublishSchedule{VideoID: videoID, PublishAt: &when}).Error; err != nil {
		return 0, false, err
	}
	// 不覆盖已提交的取消/改期；冲突时以数据库中的新计划为准。
	return s.Score(ctx, videoID)
}

func (s *ScheduleStore) DueMembers(ctx context.Context, nowUnix int64) ([]string, error) {
	var ids []uint
	if err := gormx.DB(ctx, s.db).Model(&po.VideoPublishSchedule{}).
		Where("publish_at <= ?", time.Unix(nowUnix, 0)).
		Order("publish_at ASC, video_id ASC").Limit(100).
		Pluck("video_id", &ids).Error; err != nil {
		return nil, err
	}
	members := make([]string, 0, len(ids))
	for _, id := range ids {
		members = append(members, strconv.FormatUint(uint64(id), 10))
	}
	return members, nil
}

func (s *ScheduleStore) RemoveMember(ctx context.Context, member string) error {
	id, err := strconv.ParseUint(member, 10, strconv.IntSize)
	if err != nil || id == 0 {
		if s.legacy != nil && s.legacy.Available() {
			return s.legacy.RemoveMember(ctx, member)
		}
		return nil
	}
	return s.Remove(ctx, uint(id))
}

// RecoverLegacy 先提交数据库恢复结果，再清理旧任务；任一步失败均可重试。
// 与改稿、审核、到期发布保持相同的锁顺序：先视频行，再计划行。
func (s *ScheduleStore) RecoverLegacy(ctx context.Context) error {
	if s.legacy == nil || !s.legacy.Available() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	members, err := s.legacy.DueMembers(ctx, math.MaxInt64)
	if err != nil {
		return err
	}
	var failures []error
	for _, member := range members {
		id, parseErr := strconv.ParseUint(member, 10, strconv.IntSize)
		if parseErr == nil && id != 0 {
			err = gormx.DB(ctx, s.db).Transaction(func(db *gorm.DB) error {
				txCtx := gormx.WithTx(ctx, db)
				video, err := NewVideoRepository(s.db).FindByIDForUpdate(txCtx, uint(id))
				if err != nil {
					return err
				}
				if video == nil || (video.Status != enum.VideoStatusScheduled && video.Status != enum.VideoStatusReview) {
					return s.Remove(txCtx, uint(id))
				}
				_, _, err = s.Score(txCtx, uint(id))
				return err
			})
			if err != nil {
				failures = append(failures, err)
				continue
			}
		}
		if err := s.legacy.RemoveMember(ctx, member); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
