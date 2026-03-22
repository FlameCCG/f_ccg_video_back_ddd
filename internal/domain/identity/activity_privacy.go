package identity

// ActionVisibility 是查看最近动作所需的隐私事实，不依赖其它上下文的模型。
type ActionVisibility struct {
	Exists        bool
	OpenLikeVideo bool
	OpenCoinVideo bool
}

func (ActionVisibility) IsOwner(targetID, viewerID uint) bool { return targetID == viewerID }

func (v ActionVisibility) EnsureVisible(targetID, viewerID uint, action string) error {
	if v.IsOwner(targetID, viewerID) {
		return nil
	}
	if !v.Exists {
		return ErrUserNotFound
	}
	switch action {
	case "like":
		if !v.OpenLikeVideo {
			return ErrRecentLikedNotPublic
		}
	case "coin":
		if !v.OpenCoinVideo {
			return ErrRecentCoinedNotPublic
		}
	}
	return nil
}
