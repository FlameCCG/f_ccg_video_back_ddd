package social

// MaxForeignListPage 非本人查看关注/粉丝列表时允许翻到的最大页码。
//
// 深翻页会带来大量 JOIN + OFFSET 扫描，故对他人视角设上限。
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

// PrivacyPolicy 关注列表可见性、关系查询资格和写入资格。
//
// 无配置依赖，零值即可用。隐私输入本身只承载快照，判定放在本策略上。
type PrivacyPolicy struct{}

// EnsureFollowListVisible 判定关注列表对浏览者是否可见。
//
// 判定顺序用户是否存在 → 本人绕过隐私与翻页限制 → 是否公开关注 → 他人视角的翻页上限。
// 判定不修改 privacy。
func (p PrivacyPolicy) EnsureFollowListVisible(privacy FollowPrivacy, viewerID, ownerID, page int) error {
	if !privacy.Exists {
		return ErrUserNotFound
	}

	if viewerID == ownerID {
		return nil
	}

	if !privacy.OpenFollow {
		return ErrFollowNotPublic
	}

	return p.ensureListPageAllowed(viewerID, ownerID, page)
}

// EnsureFansListVisible 判定粉丝列表对浏览者是否可见。
//
// 判定顺序用户是否存在 → 本人绕过隐私与翻页限制 → 是否公开粉丝 → 他人视角的翻页上限。
// 判定不修改 privacy。
func (p PrivacyPolicy) EnsureFansListVisible(privacy FollowPrivacy, viewerID, ownerID, page int) error {
	if !privacy.Exists {
		return ErrUserNotFound
	}

	if viewerID == ownerID {
		return nil
	}

	if !privacy.OpenFans {
		return ErrFansNotPublic
	}

	return p.ensureListPageAllowed(viewerID, ownerID, page)
}

// ensureListPageAllowed 非本人查看时限制翻页深度。
func (PrivacyPolicy) ensureListPageAllowed(viewerID, ownerID, page int) error {
	if viewerID != ownerID && page > MaxForeignListPage {
		return ErrForeignPageLimited
	}

	return nil
}

// EnsureRelationQueryable 校验关注关系查询的双方：查询自己没有关注关系。
func (PrivacyPolicy) EnsureRelationQueryable(viewerID, targetID int) error {
	if viewerID == targetID {
		return ErrSelfRelation
	}

	return nil
}

// EnsureUserID 读取隐私前要求用户 ID。空 ID 不是合法的关系主体。
func (PrivacyPolicy) EnsureUserID(userID uint) error {
	if userID == 0 {
		return ErrUserIDRequired
	}

	return nil
}

// EnsureTargetExists 关注前目标用户必须存在。
//
// 存在性查询失败仍由应用层按「目标用户不存在」呈现，此处只判定查询得到的事实。
func (PrivacyPolicy) EnsureTargetExists(exists bool) error {
	if !exists {
		return ErrTargetUserNotFound
	}

	return nil
}

// EnsureFocusRemoved 取消关注必须命中已有关系。未删除任何行时保持「未关注」。
func (PrivacyPolicy) EnsureFocusRemoved(affected int64) error {
	if affected == 0 {
		return ErrNotFocused
	}

	return nil
}
