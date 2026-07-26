package identity

import (
	"encoding/json"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toPOUser 把用户领域实体转为 PO（逐字段显式映射，注册/OAuth 创建路径使用）。
func toPOUser(user *domain.User) *po.User {
	if user == nil {
		return nil
	}

	return &po.User{
		Model: po.Model{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		Username:       user.Username,
		Email:          user.Email,
		Password:       user.Password,
		Avatar:         user.Avatar,
		Description:    user.Description,
		Gender:         user.Gender,
		Birthday:       user.Birthday,
		Level:          user.Level,
		OpenID:         user.OpenID,
		RegisterSource: user.RegisterSource,
		Exp:            user.Exp,
		CoinCount:      user.CoinCount,
		Status:         user.Status,
		CurrentRoleID:  user.CurrentRoleID,
		RegisterIP:     user.RegisterIP,
		LastLoginIP:    user.LastLoginIP,
		RegisterAddr:   user.RegisterAddr,
		LastLoginAddr:  user.LastLoginAddr,
	}
}

// toDomainUserConf 把用户配置 PO 转为领域实体（LikeTags 在此解析）。
func toDomainUserConf(row *po.UserConf) *domain.UserConf {
	if row == nil {
		return nil
	}

	return &domain.UserConf{
		UserID:        row.UserID,
		LikeTags:      decodeLikeTags(row.LikeTags),
		BannerID:      row.BannerID,
		BannerURL:     row.BannerURL,
		OpenCollect:   row.OpenCollect,
		OpenFans:      row.OpenFans,
		OpenFollow:    row.OpenFollow,
		OpenLikeVideo: row.OpenLikeVideo,
		OpenCoinVideo: row.OpenCoinVideo,
	}
}

// toUserConfView 把用户配置 PO 转为读模型（LikeTags 保持原始 JSON 字节）。
func toUserConfView(row *po.UserConf) *app.UserConfView {
	if row == nil {
		return nil
	}

	return &app.UserConfView{
		LikeTags:      row.LikeTags,
		BannerID:      row.BannerID,
		BannerURL:     row.BannerURL,
		OpenCollect:   row.OpenCollect,
		OpenFans:      row.OpenFans,
		OpenFollow:    row.OpenFollow,
		OpenLikeVideo: row.OpenLikeVideo,
		OpenCoinVideo: row.OpenCoinVideo,
	}
}

// decodeLikeTags 解析 like_tags JSON；空值或解析失败时返回空切片
// （与原 resolveUserLikeTags 的容错行为一致）。
func decodeLikeTags(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}

	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return []string{}
	}

	return tags
}

// toUserBannerItem 把横幅 PO 转为用户横幅读模型。
func toUserBannerItem(row po.Banner) app.UserBannerItem {
	return app.UserBannerItem{
		ID:          row.ID,
		Cover:       row.Cover,
		Href:        row.Href,
		Show:        row.Show,
		Type:        int(row.Type),
		PartitionID: row.PartitionID,
	}
}
