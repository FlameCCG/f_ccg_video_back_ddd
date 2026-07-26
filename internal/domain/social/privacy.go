package social

// MaxForeignListPage 非本人查看关注/粉丝列表时允许翻到的最大页码。
//
// 深翻页会带来大量 JOIN + OFFSET 扫描，故对他人视角设上限（与原实现一致）。
const MaxForeignListPage = 5

// FollowPrivacy 关注/粉丝列表的公开性快照。
//
// Exists 为 false 表示用户不存在；两个开关缺省（无 user_confs 行）时视为公开。
type FollowPrivacy struct {
	Exists     bool
	OpenFollow bool
	OpenFans   bool
}

// ActionPrivacy 最近点赞/投币的公开性快照（对齐关注列表隐私策略）。
type ActionPrivacy struct {
	Exists        bool
	OpenLikeVideo bool
	OpenCoinVideo bool
}

// EnsureFollowListVisible 判定关注列表对浏览者是否可见。
//
// 判定顺序与原 api/social_api/focus_list.go 一致：
// 用户是否存在 → 是否公开关注 → 他人视角的翻页上限。
func EnsureFollowListVisible(privacy FollowPrivacy, viewerID, ownerID, page int) error {
	if !privacy.Exists {
		return ErrUserNotFound
	}

	if !privacy.OpenFollow {
		return ErrFollowNotPublic
	}

	return ensureListPageAllowed(viewerID, ownerID, page)
}

// EnsureFansListVisible 判定粉丝列表对浏览者是否可见。
//
// 判定顺序与原 api/social_api/fans_list.go 一致：
// 用户是否存在 → 是否公开粉丝 → 他人视角的翻页上限。
func EnsureFansListVisible(privacy FollowPrivacy, viewerID, ownerID, page int) error {
	if !privacy.Exists {
		return ErrUserNotFound
	}

	if !privacy.OpenFans {
		return ErrFansNotPublic
	}

	return ensureListPageAllowed(viewerID, ownerID, page)
}

// ensureListPageAllowed 非本人查看时限制翻页深度。
func ensureListPageAllowed(viewerID, ownerID, page int) error {
	if viewerID != ownerID && page > MaxForeignListPage {
		return ErrForeignPageLimited
	}

	return nil
}

// EnsureRelationQueryable 校验关注关系查询的双方：查询自己没有关注关系。
func EnsureRelationQueryable(viewerID, targetID int) error {
	if viewerID == targetID {
		return ErrSelfRelation
	}

	return nil
}
