package identity

import (
	"slices"
	"strings"
)

// UserConf 用户配置（用户聚合内的实体，一对一挂在 User 下）。
//
// LikeTags 为已解析后的标签集合：持久化层负责 JSON 编解码，
// 解析失败时返回空切片。
type UserConf struct {
	userID        uint
	likeTags      []string
	bannerID      uint
	bannerURL     string
	openCollect   bool
	openFans      bool
	openFollow    bool
	openLikeVideo bool
	openCoinVideo bool
}

// UserConfSnapshot 是用户配置的持久化快照。
type UserConfSnapshot struct {
	UserID        uint
	LikeTags      []string
	BannerID      uint
	BannerURL     string
	OpenCollect   bool
	OpenFans      bool
	OpenFollow    bool
	OpenLikeVideo bool
	OpenCoinVideo bool
}

// UserConfUpdate 是用户配置的更新入参；命令字段保持公开。
type UserConfUpdate struct {
	OpenCollect   *bool
	OpenFans      *bool
	OpenFollow    *bool
	OpenLikeVideo *bool
	OpenCoinVideo *bool
	BannerID      *uint
	BannerURL     string
	LikeTags      *[]string
}

// NewUserConf 与既有数据库缺省一致，不把新配置初始化为全部关闭。
func NewUserConf(userID, bannerID uint) *UserConf {
	return &UserConf{userID: userID, bannerID: bannerID, openCollect: true, openFans: true, openFollow: true, openLikeVideo: true, openCoinVideo: true}
}

// RestoreUserConf 从快照恢复用户配置，不重新规范化标签。
func RestoreUserConf(snapshot UserConfSnapshot) *UserConf {
	return &UserConf{
		userID:        snapshot.UserID,
		likeTags:      slices.Clone(snapshot.LikeTags),
		bannerID:      snapshot.BannerID,
		bannerURL:     snapshot.BannerURL,
		openCollect:   snapshot.OpenCollect,
		openFans:      snapshot.OpenFans,
		openFollow:    snapshot.OpenFollow,
		openLikeVideo: snapshot.OpenLikeVideo,
		openCoinVideo: snapshot.OpenCoinVideo,
	}
}

// Snapshot 返回脱离实体的副本。
func (c *UserConf) Snapshot() UserConfSnapshot {
	if c == nil {
		return UserConfSnapshot{}
	}

	return UserConfSnapshot{
		UserID:        c.userID,
		LikeTags:      slices.Clone(c.likeTags),
		BannerID:      c.bannerID,
		BannerURL:     c.bannerURL,
		OpenCollect:   c.openCollect,
		OpenFans:      c.openFans,
		OpenFollow:    c.openFollow,
		OpenLikeVideo: c.openLikeVideo,
		OpenCoinVideo: c.openCoinVideo,
	}
}

func (c *UserConf) UserID() uint        { return c.orZero().userID }
func (c *UserConf) BannerID() uint      { return c.orZero().bannerID }
func (c *UserConf) BannerURL() string   { return c.orZero().bannerURL }
func (c *UserConf) OpenCollect() bool   { return c.orZero().openCollect }
func (c *UserConf) OpenFans() bool      { return c.orZero().openFans }
func (c *UserConf) OpenFollow() bool    { return c.orZero().openFollow }
func (c *UserConf) OpenLikeVideo() bool { return c.orZero().openLikeVideo }
func (c *UserConf) OpenCoinVideo() bool { return c.orZero().openCoinVideo }

// LikeTags 返回标签副本。
func (c *UserConf) LikeTags() []string {
	if c == nil {
		return nil
	}

	return slices.Clone(c.likeTags)
}

func (c *UserConf) orZero() UserConf {
	if c == nil {
		return UserConf{}
	}

	return *c
}

// PublicLikeTags 返回去空白去重后的标签；配置缺失时为空切片而非 nil。
func (c *UserConf) PublicLikeTags() []string {
	if c == nil {
		return []string{}
	}

	return (ProfilePolicy{}).NormalizeLikeTags(c.likeTags)
}

// CustomBannerURL 返回去空白后的自定义横幅；没有时为空串。
func (c *UserConf) CustomBannerURL() string {
	if c == nil {
		return ""
	}

	return strings.TrimSpace(c.bannerURL)
}

// SystemBannerID 解析主页展示用的系统横幅：用户已选优先，否则用站点默认。
func (c *UserConf) SystemBannerID(configuredDefault uint) uint {
	bannerID := (BannerPolicy{}).ResolveDefaultID(configuredDefault)
	if c != nil && c.bannerID > 0 {
		bannerID = c.bannerID
	}

	return bannerID
}

// ListedBanner 返回用户显式选择。自定义 URL 优先且 ID 为 0；都没有时两者为零值。
func (c *UserConf) ListedBanner() (bannerID uint, customURL string) {
	if c == nil {
		return 0, ""
	}

	if url := strings.TrimSpace(c.bannerURL); url != "" {
		return 0, url
	}

	return c.bannerID, ""
}

// Plan 组装部分更新。选择系统横幅时同时清空自定义 URL。失败时不修改配置。
func (c *UserConf) Plan(update UserConfUpdate) (UserConfChanges, error) {
	if c == nil {
		return UserConfChanges{}, ErrUserConfNotFound
	}

	changes := UserConfChanges{
		OpenCollect:   update.OpenCollect,
		OpenFans:      update.OpenFans,
		OpenFollow:    update.OpenFollow,
		OpenLikeVideo: update.OpenLikeVideo,
		OpenCoinVideo: update.OpenCoinVideo,
	}

	if update.LikeTags != nil {
		normalized := (ProfilePolicy{}).NormalizeLikeTags(*update.LikeTags)
		changes.LikeTags = &normalized
	}

	bannerURL := strings.TrimSpace(update.BannerURL)

	switch {
	case update.BannerID != nil:
		bannerID := *update.BannerID
		cleared := ""
		changes.BannerID = &bannerID
		changes.BannerURL = &cleared
	case bannerURL != "":
		changes.BannerURL = &bannerURL
	}

	if changes.IsEmpty() {
		return UserConfChanges{}, ErrNoFieldsToUpdate
	}

	return changes, nil
}

// UserConfChanges 表示用户配置中允许更新的字段；nil 表示不更新。
type UserConfChanges struct {
	OpenCollect   *bool
	OpenFans      *bool
	OpenFollow    *bool
	OpenLikeVideo *bool
	OpenCoinVideo *bool
	BannerID      *uint
	BannerURL     *string
	LikeTags      *[]string
}

// IsEmpty 报告是否没有任何字段需要更新。
func (c UserConfChanges) IsEmpty() bool {
	return c.OpenCollect == nil &&
		c.OpenFans == nil &&
		c.OpenFollow == nil &&
		c.OpenLikeVideo == nil &&
		c.OpenCoinVideo == nil &&
		c.BannerID == nil &&
		c.BannerURL == nil &&
		c.LikeTags == nil
}
