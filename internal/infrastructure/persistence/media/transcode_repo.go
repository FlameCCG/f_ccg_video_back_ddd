// Package media 实现转码任务的资源、时长和任务有效性读写。
//
// transcode_repo.go 直接操作 videos/video_parts 及资源表，为 media 的资源端口提供实现。
// 这是当前共享数据库实现中的跨上下文表访问；不代表媒体领域直接导入 video 领域或两者已物理隔库。
// SQL 保存与对象存储上传分开完成，资源唯一约束和对象探测共同支持重试。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package media

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TranscodeResourceRepository 转码资源仓储的 GORM 实现。
type TranscodeResourceRepository struct {
	db  *gorm.DB
	txm tx.Manager
}

// 编译期断言：满足领域侧仓储端口。
var _ domain.TranscodeResourceRepository = (*TranscodeResourceRepository)(nil)

// NewTranscodeResourceRepository 构造转码资源仓储。
func NewTranscodeResourceRepository(db *gorm.DB, txm tx.Manager) *TranscodeResourceRepository {
	return &TranscodeResourceRepository{db: db, txm: txm}
}

// EnsureTaskActive 校验视频（及分P）仍存在且未被删除；已删除或不存在时返回 ErrVideoDeleted。
func (r *TranscodeResourceRepository) EnsureTaskActive(ctx context.Context, task domain.TranscodeTask) error {
	if errors.Is(context.Cause(ctx), domain.ErrVideoDeleted) {
		return domain.ErrVideoDeleted
	}

	if r == nil || r.db == nil {
		return errors.New("database not initialized")
	}

	var video po.Video

	err := gormx.DB(ctx, r.db).
		Select("id", "status").
		Where("id = ?", task.VideoID).
		Take(&video).Error
	if err != nil {
		return classifyTranscodeStateError(ctx, err)
	}

	if video.Status == enum.VideoStatusDeleted {
		return domain.ErrVideoDeleted
	}

	if task.VideoPartID == 0 {
		return nil
	}

	var part po.Part

	err = gormx.DB(ctx, r.db).
		Select("id").
		Where("id = ? AND video_id = ?", task.VideoPartID, task.VideoID).
		Take(&part).Error
	if err != nil {
		return classifyTranscodeStateError(ctx, err)
	}

	return nil
}

// UpdateDurationFromProbe 用探测到的时长回填分P/视频时长。
//
// 有分P 时先更新分P，再把视频总时长重算为各分P 之和（同一事务）；
// 单P 视频只在 duration 仍为 0 时回填。
func (r *TranscodeResourceRepository) UpdateDurationFromProbe(
	ctx context.Context,
	task domain.TranscodeTask,
	duration int,
) error {
	if duration <= 0 || task.VideoID == 0 {
		return nil
	}

	if task.VideoPartID > 0 {
		return r.txm.Do(ctx, func(txCtx context.Context) error {
			db := gormx.DB(txCtx, r.db)
			if err := db.Model(&po.Part{}).
				Where("id = ?", task.VideoPartID).
				Update("duration", duration).Error; err != nil {
				return err
			}

			return db.Exec(
				"UPDATE videos SET duration = (SELECT COALESCE(SUM(duration),0) FROM video_parts WHERE video_id = ?) WHERE id = ?",
				task.VideoID,
				task.VideoID,
			).Error
		})
	}

	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ? AND duration = 0", task.VideoID).
		Update("duration", duration).Error
}

// SaveOriginalResource 落库源视频资源行。
//
// 先按 (分辨率, is_souce_video=true) 查已有行，查不到再放宽为仅 is_souce_video=true；
// 命中多行时保留 file_size 最大者并删除其余，避免历史重复行堆积。
func (r *TranscodeResourceRepository) SaveOriginalResource(
	ctx context.Context,
	task domain.TranscodeTask,
	meta domain.VideoMeta,
	sourceURL string,
	size int64,
	highBitrate bool,
) error {
	resolution := meta.Resolution()
	if highBitrate && resolution == enum.Resolution1080P {
		resolution = enum.Resolution1080PHigh
	}

	isVip := highBitrate || resolution == enum.Resolution2160P
	format := meta.CanonicalResourceFormat()
	codec := strings.TrimSpace(meta.Codec)
	sourceName := task.SourceDisplayName()
	fileURL := strings.TrimSpace(sourceURL)
	bitrate := meta.Bitrate
	fileSize := size
	normalizedRelative := task.NormalizedSourceRelativePath()

	return r.txm.Do(ctx, func(txCtx context.Context) error {
		db := gormx.DB(txCtx, r.db)
		// 行级锁：避免同一视频重复写入源资源，并与硬删除状态切换串行化。
		if err := lockActiveVideo(txCtx, db, task.VideoID); err != nil {
			return err
		}

		if task.VideoPartID > 0 {
			if err := lockVideoPart(txCtx, db, task.VideoID, task.VideoPartID); err != nil {
				return err
			}

			var list []po.VideoPartResource
			if err := db.Where(
				"video_part_id = ? AND resolution = ? AND is_souce_video = ?",
				task.VideoPartID, resolution, true,
			).Order("file_size DESC, id DESC").Find(&list).Error; err != nil {
				return err
			}

			if len(list) == 0 {
				if err := db.Where("video_part_id = ? AND is_souce_video = ?", task.VideoPartID, true).
					Order("file_size DESC, id DESC").
					Find(&list).Error; err != nil {
					return err
				}
			}

			if len(list) == 0 {
				return db.Create(&po.VideoPartResource{
					VideoPartID:    task.VideoPartID,
					Resolution:     resolution,
					SourceFileName: sourceName,
					FileURL:        fileURL,
					FileSize:       fileSize,
					Bitrate:        bitrate,
					Format:         format,
					Codec:          codec,
					IsVip:          isVip,
					IsSouceVideo:   true,
				}).Error
			}

			keep := list[0]
			update := originalResourceUpdateMap(
				resolution, isVip, normalizedRelative,
				sourceName, fileURL, format, codec, fileSize, bitrate,
				keep.SourceFileName, keep.FileURL, keep.Format, keep.Codec, keep.FileSize, keep.Bitrate,
			)

			if err := db.Model(&po.VideoPartResource{}).Where("id = ?", keep.ID).Updates(update).Error; err != nil {
				return err
			}

			return deleteDuplicateVideoPartResources(db, list[1:])
		}

		var list []po.VideoResource
		if err := db.Where(
			"video_id = ? AND resolution = ? AND is_souce_video = ?",
			task.VideoID, resolution, true,
		).Order("file_size DESC, id DESC").Find(&list).Error; err != nil {
			return err
		}

		if len(list) == 0 {
			if err := db.Where("video_id = ? AND is_souce_video = ?", task.VideoID, true).
				Order("file_size DESC, id DESC").
				Find(&list).Error; err != nil {
				return err
			}
		}

		if len(list) == 0 {
			return db.Create(&po.VideoResource{
				VideoID:        task.VideoID,
				Resolution:     resolution,
				SourceFileName: sourceName,
				FileURL:        fileURL,
				FileSize:       fileSize,
				Bitrate:        bitrate,
				Format:         format,
				Codec:          codec,
				IsVip:          isVip,
				IsSouceVideo:   true,
			}).Error
		}

		keep := list[0]
		update := originalResourceUpdateMap(
			resolution, isVip, normalizedRelative,
			sourceName, fileURL, format, codec, fileSize, bitrate,
			keep.SourceFileName, keep.FileURL, keep.Format, keep.Codec, keep.FileSize, keep.Bitrate,
		)

		if err := db.Model(&po.VideoResource{}).Where("id = ?", keep.ID).Updates(update).Error; err != nil {
			return err
		}

		return deleteDuplicateVideoResources(db, list[1:])
	})
}

// originalResourceUpdateMap 构造源资源行的更新字段。
//
// 空值回落到既有行的值；若既有行已指向本任务的规范化产物，则整组字段保持不动 ——
// 否则重试会把行 URL 重置回原始源，导致已完成的规范化目标被误判未完成而重编码。
func originalResourceUpdateMap(
	resolution enum.VideoResolution,
	isVip bool,
	normalizedRelative string,
	sourceName, fileURL, format, codec string,
	fileSize int64,
	bitrate int,
	keepName, keepURL, keepFormat, keepCodec string,
	keepSize int64,
	keepBitrate int,
) map[string]any {
	if sourceName == "" {
		sourceName = keepName
	}

	if fileURL == "" {
		fileURL = keepURL
	}

	if fileSize <= 0 {
		fileSize = keepSize
	}

	if bitrate <= 0 {
		bitrate = keepBitrate
	}

	if codec == "" {
		codec = keepCodec
	}

	if format == "" {
		format = keepFormat
	}

	// 断点续传：源行已指向本任务的规范化产物时不回退到原始源。
	if domain.StoredPathMatchesRelative(keepURL, normalizedRelative) {
		fileURL = keepURL
		fileSize = keepSize
		bitrate = keepBitrate
		format = keepFormat
		codec = keepCodec
	}

	return map[string]any{
		"resolution":       resolution,
		"source_file_name": sourceName,
		"file_url":         fileURL,
		"file_size":        fileSize,
		"bitrate":          bitrate,
		"format":           format,
		"codec":            codec,
		"is_vip":           isVip,
		"is_souce_video":   true,
	}
}

// UpdateOriginalResourceFile 把源资源行指向规范化后的产物。
func (r *TranscodeResourceRepository) UpdateOriginalResourceFile(
	ctx context.Context,
	task domain.TranscodeTask,
	res enum.VideoResolution,
	newFileName, url string,
	size int64,
	bitrate int,
) error {
	update := map[string]any{
		"source_file_name": newFileName,
		"file_url":         url,
		"file_size":        size,
		"bitrate":          bitrate,
		"format":           "mp4",
		"codec":            "h264",
	}

	return r.txm.Do(ctx, func(txCtx context.Context) error {
		db := gormx.DB(txCtx, r.db)
		if err := lockActiveVideo(txCtx, db, task.VideoID); err != nil {
			return err
		}

		if task.VideoPartID > 0 {
			if err := lockVideoPart(txCtx, db, task.VideoID, task.VideoPartID); err != nil {
				return err
			}

			result := db.
				Model(&po.VideoPartResource{}).
				Where("video_part_id = ? AND resolution = ? AND is_souce_video = ?", task.VideoPartID, res, true).
				Updates(update)
			if result.Error != nil {
				return result.Error
			}

			if result.RowsAffected == 0 {
				return fmt.Errorf(
					"original part resource not found: part=%d resolution=%s",
					task.VideoPartID,
					res,
				)
			}

			return nil
		}

		result := db.
			Model(&po.VideoResource{}).
			Where("video_id = ? AND resolution = ? AND is_souce_video = ?", task.VideoID, res, true).
			Updates(update)
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf(
				"original video resource not found: video=%d resolution=%s",
				task.VideoID,
				res,
			)
		}

		return nil
	})
}

// SaveVideoResource 按输出高度 upsert 一条派生资源行（4K 标记为 VIP）。
func (r *TranscodeResourceRepository) SaveVideoResource(
	ctx context.Context,
	task domain.TranscodeTask,
	res uint16,
	sourceName, url string,
	size int64,
	bitrate int,
) error {
	resolution := domain.MapResolutionEnumByHeight(int(res))

	return r.SaveSpecificVideoResource(
		ctx,
		task,
		resolution,
		sourceName,
		url,
		size,
		bitrate,
		resolution == enum.Resolution2160P,
		false,
	)
}

// SaveSpecificVideoResource 按显式分辨率枚举 upsert 一条资源行。
func (r *TranscodeResourceRepository) SaveSpecificVideoResource(
	ctx context.Context,
	task domain.TranscodeTask,
	res enum.VideoResolution,
	sourceName, url string,
	size int64,
	bitrate int,
	isVip, isSource bool,
) error {
	return r.txm.Do(ctx, func(txCtx context.Context) error {
		db := gormx.DB(txCtx, r.db)
		if err := lockActiveVideo(txCtx, db, task.VideoID); err != nil {
			return err
		}

		if task.VideoPartID > 0 {
			if err := lockVideoPart(txCtx, db, task.VideoID, task.VideoPartID); err != nil {
				return err
			}

			var list []po.VideoPartResource
			if err := db.Where(
				"video_part_id = ? AND resolution = ? AND is_souce_video = ?",
				task.VideoPartID, res, isSource,
			).Order("file_size DESC, id DESC").Find(&list).Error; err != nil {
				return err
			}

			if len(list) == 0 && isSource && strings.TrimSpace(url) != "" {
				if err := db.Where(
					"video_part_id = ? AND is_souce_video = ? AND file_url = ?",
					task.VideoPartID, true, url,
				).Order("file_size DESC, id DESC").Find(&list).Error; err != nil {
					return err
				}
			}

			if len(list) == 0 {
				return db.Create(&po.VideoPartResource{
					VideoPartID:    task.VideoPartID,
					Resolution:     res,
					SourceFileName: sourceName,
					FileURL:        url,
					FileSize:       size,
					Bitrate:        bitrate,
					Format:         "mp4",
					Codec:          "h264",
					IsVip:          isVip,
					IsSouceVideo:   isSource,
				}).Error
			}

			keep := list[0]
			update := videoResourceUpdateMap(
				res, sourceName, url, size, bitrate, isVip, isSource,
				keep.SourceFileName, keep.FileURL, keep.FileSize, keep.Bitrate,
			)

			if err := db.Model(&po.VideoPartResource{}).Where("id = ?", keep.ID).Updates(update).Error; err != nil {
				return err
			}

			return deleteDuplicateVideoPartResources(db, list[1:])
		}

		var list []po.VideoResource
		if err := db.Where(
			"video_id = ? AND resolution = ? AND is_souce_video = ?",
			task.VideoID, res, isSource,
		).Order("file_size DESC, id DESC").Find(&list).Error; err != nil {
			return err
		}

		if len(list) == 0 && isSource && strings.TrimSpace(url) != "" {
			if err := db.Where(
				"video_id = ? AND is_souce_video = ? AND file_url = ?",
				task.VideoID, true, url,
			).Order("file_size DESC, id DESC").Find(&list).Error; err != nil {
				return err
			}
		}

		if len(list) == 0 {
			return db.Create(&po.VideoResource{
				VideoID:        task.VideoID,
				Resolution:     res,
				SourceFileName: sourceName,
				FileURL:        url,
				FileSize:       size,
				Bitrate:        bitrate,
				Format:         "mp4",
				Codec:          "h264",
				IsVip:          isVip,
				IsSouceVideo:   isSource,
			}).Error
		}

		keep := list[0]
		update := videoResourceUpdateMap(
			res, sourceName, url, size, bitrate, isVip, isSource,
			keep.SourceFileName, keep.FileURL, keep.FileSize, keep.Bitrate,
		)

		if err := db.Model(&po.VideoResource{}).Where("id = ?", keep.ID).Updates(update).Error; err != nil {
			return err
		}

		return deleteDuplicateVideoResources(db, list[1:])
	})
}

// SaveDASHResource 把整套 DASH 落库为一行资源(Format="dash", Resolution=自适应)。
// 重试时按 (video[_part]_id, resolution=自适应, is_souce_video=false) 去重并原地更新。
func (r *TranscodeResourceRepository) SaveDASHResource(
	ctx context.Context,
	task domain.TranscodeTask,
	sourceName, mpdURL string,
	totalSize int64,
) error {
	name := strings.TrimSpace(sourceName)
	updates := map[string]any{
		"source_file_name": name,
		"file_url":         mpdURL,
		"file_size":        totalSize,
		"format":           "dash",
		"codec":            "h264",
	}

	return r.txm.Do(ctx, func(txCtx context.Context) error {
		db := gormx.DB(txCtx, r.db)
		if err := lockActiveVideo(txCtx, db, task.VideoID); err != nil {
			return err
		}

		if task.VideoPartID > 0 {
			if err := lockVideoPart(txCtx, db, task.VideoID, task.VideoPartID); err != nil {
				return err
			}

			var existing po.VideoPartResource

			err := db.Where(
				"video_part_id = ? AND resolution = ? AND is_souce_video = ?",
				task.VideoPartID, enum.ResolutionAuto, false,
			).First(&existing).Error
			if err == nil {
				return db.Model(&existing).Updates(updates).Error
			}

			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			return db.Create(&po.VideoPartResource{
				VideoPartID:    task.VideoPartID,
				Resolution:     enum.ResolutionAuto,
				SourceFileName: name,
				FileURL:        mpdURL,
				FileSize:       totalSize,
				Bitrate:        0,
				Format:         "dash",
				Codec:          "h264",
				IsVip:          false,
				IsSouceVideo:   false,
			}).Error
		}

		var existing po.VideoResource

		err := db.Where(
			"video_id = ? AND resolution = ? AND is_souce_video = ?",
			task.VideoID, enum.ResolutionAuto, false,
		).First(&existing).Error
		if err == nil {
			return db.Model(&existing).Updates(updates).Error
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		return db.Create(&po.VideoResource{
			VideoID:        task.VideoID,
			Resolution:     enum.ResolutionAuto,
			SourceFileName: name,
			FileURL:        mpdURL,
			FileSize:       totalSize,
			Bitrate:        0,
			Format:         "dash",
			Codec:          "h264",
			IsVip:          false,
			IsSouceVideo:   false,
		}).Error
	})
}

// PersistedResourceURLs 返回 video/part 维度下匹配 (resolution, isSource) 的资源行 file_url 列表。
// resolution 为空字符串时不按分辨率过滤（供源行查询使用）。
func (r *TranscodeResourceRepository) PersistedResourceURLs(
	ctx context.Context,
	task domain.TranscodeTask,
	resolution enum.VideoResolution,
	isSource bool,
) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("database not initialized")
	}

	var urls []string

	if task.VideoPartID > 0 {
		query := gormx.DB(ctx, r.db).
			Model(&po.VideoPartResource{}).
			Where("video_part_id = ? AND is_souce_video = ?", task.VideoPartID, isSource)
		if resolution != "" {
			query = query.Where("resolution = ?", resolution)
		}

		err := query.Order("file_size DESC, id DESC").Pluck("file_url", &urls).Error

		return urls, err
	}

	query := gormx.DB(ctx, r.db).
		Model(&po.VideoResource{}).
		Where("video_id = ? AND is_souce_video = ?", task.VideoID, isSource)
	if resolution != "" {
		query = query.Where("resolution = ?", resolution)
	}

	err := query.Order("file_size DESC, id DESC").Pluck("file_url", &urls).Error

	return urls, err
}

// lockActiveVideo 以行锁读取未删除的视频，与硬删除状态切换串行化。
func lockActiveVideo(ctx context.Context, db *gorm.DB, videoID uint) error {
	if db == nil || videoID == 0 {
		return domain.ErrVideoDeleted
	}

	var video po.Video

	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ? AND status <> ?", videoID, enum.VideoStatusDeleted).
		Take(&video).Error

	return classifyTranscodeStateError(ctx, err)
}

// lockVideoPart 以行锁读取分P，避免同一分P 重复写入资源（并发/重试任务）。
func lockVideoPart(ctx context.Context, db *gorm.DB, videoID, partID uint) error {
	var part po.Part

	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ? AND video_id = ?", partID, videoID).
		Take(&part).Error
	if err != nil {
		return classifyTranscodeStateError(ctx, err)
	}

	return nil
}

// classifyTranscodeStateError 把「记录不存在」与「取消原因为视频删除」统一归为 ErrVideoDeleted。
func classifyTranscodeStateError(ctx context.Context, err error) error {
	if errors.Is(context.Cause(ctx), domain.ErrVideoDeleted) || errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrVideoDeleted
	}

	return err
}

// videoResourceUpdateMap 构造派生资源行的更新字段；空值回落到既有行的值。
func videoResourceUpdateMap(
	res enum.VideoResolution,
	sourceName string,
	url string,
	size int64,
	bitrate int,
	isVip bool,
	isSource bool,
	fallbackName string,
	fallbackURL string,
	fallbackSize int64,
	fallbackBitrate int,
) map[string]any {
	if strings.TrimSpace(sourceName) == "" {
		sourceName = fallbackName
	}

	if strings.TrimSpace(url) == "" {
		url = fallbackURL
	}

	if size <= 0 {
		size = fallbackSize
	}

	if bitrate <= 0 {
		bitrate = fallbackBitrate
	}

	return map[string]any{
		"resolution":       res,
		"source_file_name": sourceName,
		"file_url":         url,
		"file_size":        size,
		"bitrate":          bitrate,
		"format":           "mp4",
		"codec":            "h264",
		"is_vip":           isVip,
		"is_souce_video":   isSource,
	}
}

func deleteDuplicateVideoPartResources(db *gorm.DB, list []po.VideoPartResource) error {
	if len(list) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}

	return db.Where("id IN ?", ids).Delete(&po.VideoPartResource{}).Error
}

func deleteDuplicateVideoResources(db *gorm.DB, list []po.VideoResource) error {
	if len(list) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}

	return db.Where("id IN ?", ids).Delete(&po.VideoResource{}).Error
}
