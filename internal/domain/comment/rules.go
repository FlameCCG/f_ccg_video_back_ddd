package comment

const (
	// MaxContentRunes 评论文字长度上限（按 rune 计）。
	MaxContentRunes = 1000
	// DefaultPageSize 列表默认每页数量。
	DefaultPageSize = 20
	// MaxPageSize 前台列表每页数量上限。
	MaxPageSize = 50
	// MaxAdminPageSize 管理端列表每页数量上限。
	MaxAdminPageSize = 100
)

// PagePolicy 纠偏评论列表分页。
type PagePolicy struct{}

// Normalize 纠偏前台列表分页参数（每页上限 MaxPageSize）。
func (p PagePolicy) Normalize(page, pageSize int) (int, int) {
	return p.normalize(page, pageSize, MaxPageSize)
}

// NormalizeAdmin 纠偏管理端列表分页参数（每页上限 MaxAdminPageSize）。
func (p PagePolicy) NormalizeAdmin(page, pageSize int) (int, int) {
	return p.normalize(page, pageSize, MaxAdminPageSize)
}

func (PagePolicy) normalize(page, pageSize, maxPageSize int) (int, int) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = DefaultPageSize
	}

	return page, pageSize
}

// CreatorPolicy 创作中心列表的作者标识校验。
type CreatorPolicy struct{}

// RequireAuthor 拒绝缺失的作者 ID。
func (CreatorPolicy) RequireAuthor(authorID uint) error {
	if authorID == 0 {
		return ErrAuthorIDRequired
	}

	return nil
}

// LoginPolicy 操作者登录态校验。
type LoginPolicy struct{}

// Require 拒绝未登录操作者。
func (LoginPolicy) Require(userID uint) error {
	if userID == 0 {
		return ErrUserNotLogin
	}

	return nil
}

// PinPolicy 置顶请求在加载评论之前的标识校验。
type PinPolicy struct{}

// Require 按登录态、评论 ID 的顺序拒绝无效置顶请求。
func (PinPolicy) Require(userID, commentID uint) error {
	if err := (LoginPolicy{}).Require(userID); err != nil {
		return err
	}

	if commentID == 0 {
		return ErrCommentIDRequired
	}

	return nil
}

// Removal 批量删除要落库的 ID 与计数变化。
type Removal struct {
	IDs         []uint
	ReplyDeltas map[uint]int64
	VideoDeltas map[uint]int64
}

// DeletePolicy 计算管理端批量删除的资格与计数。
type DeletePolicy struct{}

// RequireIDs 拒绝空的评论 ID 列表。
func (DeletePolicy) RequireIDs(commentIDs []uint) error {
	if len(commentIDs) == 0 {
		return ErrCommentIDsRequired
	}

	return nil
}

// RootIDs 按出现顺序返回选中项里的一级评论 ID，跳过 ID 为 0 的记录。
func (DeletePolicy) RootIDs(comments []Comment) []uint {
	rootIDs := make([]uint, 0, len(comments))

	for _, c := range comments {
		if c.ID() == 0 || !c.IsRoot() {
			continue
		}

		rootIDs = append(rootIDs, c.ID())
	}

	return rootIDs
}

// Removal 汇总待删 ID、需要扣减回复数的根评论，以及视频评论数变化。
//
// 根评论与它的回复一起删除时不扣回复数；同一条评论只计一次。
func (p DeletePolicy) Removal(selected, children []Comment) Removal {
	roots := make(map[uint]struct{})

	for _, c := range selected {
		if c.ID() != 0 && c.IsRoot() {
			roots[c.ID()] = struct{}{}
		}
	}

	replyDeltas := make(map[uint]int64)

	for _, c := range selected {
		if c.ID() == 0 || c.IsRoot() {
			continue
		}

		rootID := *c.RootID()
		if _, ok := roots[rootID]; ok {
			continue
		}

		replyDeltas[rootID]++
	}

	ids, videoDeltas := p.collect(selected, children)

	return Removal{IDs: ids, ReplyDeltas: replyDeltas, VideoDeltas: videoDeltas}
}

func (DeletePolicy) collect(groups ...[]Comment) ([]uint, map[uint]int64) {
	seen := make(map[uint]struct{})
	videoDeltas := make(map[uint]int64)

	var ids []uint

	for _, group := range groups {
		for _, c := range group {
			id := c.ID()
			if id == 0 {
				continue
			}

			if _, ok := seen[id]; ok {
				continue
			}

			seen[id] = struct{}{}
			ids = append(ids, id)

			if videoID := c.VideoID(); videoID > 0 {
				videoDeltas[videoID]++
			}
		}
	}

	return ids, videoDeltas
}
