package dynamic

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ReadRecord 用户对某作者「最新作品」的已读位点。
//
// 一条记录代表 UserID 对 AuthorID 的最新作品已读到哪个作品（按作品发布时间比对）。
type ReadRecord struct {
	ID               uint
	UserID           uint
	AuthorID         uint
	LastReadWorkType enum.WorkType
	LastReadWorkID   uint
	LastReadWorkAt   time.Time
}

// ShouldAdvance 判定已读位点是否需要向前推进。
//
// 规则与原 upsertRead 一致：只有当目标作品发布时间**严格晚于**当前位点时才推进，
// 相等或更早都视为已读过，不产生写入。记录不存在（nil）时视为需要写入。
func (r *ReadRecord) ShouldAdvance(workAt time.Time) bool {
	if r == nil {
		return true
	}

	return r.LastReadWorkAt.Before(workAt)
}
