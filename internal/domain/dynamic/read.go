package dynamic

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ReadRecord 用户对某作者「最新作品」的已读位点。
//
// 一条记录代表用户对作者的最新作品已读到哪个作品（按作品发布时间比对）。
type ReadRecord struct {
	id               uint
	userID           uint
	authorID         uint
	lastReadWorkType enum.WorkType
	lastReadWorkID   uint
	lastReadWorkAt   time.Time
}

// ReadRecordSnapshot 已读位点的持久化快照。字段公开，重建时不重复执行创建校验。
type ReadRecordSnapshot struct {
	ID               uint
	UserID           uint
	AuthorID         uint
	LastReadWorkType enum.WorkType
	LastReadWorkID   uint
	LastReadWorkAt   time.Time
}

// NewReadRecord 构造一条尚未持久化的已读位点。ID 由仓储写入后通过返回值给出。
func NewReadRecord(userID, authorID uint, workType enum.WorkType, workID uint, workAt time.Time) *ReadRecord {
	return &ReadRecord{
		userID:           userID,
		authorID:         authorID,
		lastReadWorkType: workType,
		lastReadWorkID:   workID,
		lastReadWorkAt:   workAt,
	}
}

// RestoreReadRecord 从持久化快照恢复已读位点，不重新执行创建校验。
func RestoreReadRecord(snapshot ReadRecordSnapshot) *ReadRecord {
	return &ReadRecord{
		id:               snapshot.ID,
		userID:           snapshot.UserID,
		authorID:         snapshot.AuthorID,
		lastReadWorkType: snapshot.LastReadWorkType,
		lastReadWorkID:   snapshot.LastReadWorkID,
		lastReadWorkAt:   snapshot.LastReadWorkAt,
	}
}

func (r *ReadRecord) ID() uint {
	if r == nil {
		return 0
	}

	return r.id
}

func (r *ReadRecord) UserID() uint {
	if r == nil {
		return 0
	}

	return r.userID
}

func (r *ReadRecord) AuthorID() uint {
	if r == nil {
		return 0
	}

	return r.authorID
}

func (r *ReadRecord) LastReadWorkType() enum.WorkType {
	if r == nil {
		return 0
	}

	return r.lastReadWorkType
}

func (r *ReadRecord) LastReadWorkID() uint {
	if r == nil {
		return 0
	}

	return r.lastReadWorkID
}

func (r *ReadRecord) LastReadWorkAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.lastReadWorkAt
}

// Snapshot 返回当前已读位点的副本。
func (r *ReadRecord) Snapshot() ReadRecordSnapshot {
	if r == nil {
		return ReadRecordSnapshot{}
	}

	return ReadRecordSnapshot{
		ID:               r.id,
		UserID:           r.userID,
		AuthorID:         r.authorID,
		LastReadWorkType: r.lastReadWorkType,
		LastReadWorkID:   r.lastReadWorkID,
		LastReadWorkAt:   r.lastReadWorkAt,
	}
}

// ShouldAdvance 判定已读位点是否需要向前推进。
//
// 规则：只有当目标作品发布时间严格晚于当前位点时才推进，
// 相等或更早都视为已读过，不产生写入。记录不存在（nil）时视为需要写入。
func (r *ReadRecord) ShouldAdvance(workAt time.Time) bool {
	if r == nil {
		return true
	}

	return r.lastReadWorkAt.Before(workAt)
}

// Advance 在作品严格晚于当前位点时推进已读作品与时间。
// 记录不存在，或作品不比当前位点新时，保持原状态并返回 false。
func (r *ReadRecord) Advance(workType enum.WorkType, workID uint, workAt time.Time) bool {
	if r == nil || !r.ShouldAdvance(workAt) {
		return false
	}

	r.lastReadWorkType = workType
	r.lastReadWorkID = workID
	r.lastReadWorkAt = workAt

	return true
}
