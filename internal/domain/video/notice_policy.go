package video

// NoticePolicy 决定视频互动通知的接收资格，外层负责查询资料与投递。
type NoticePolicy struct{}

func (NoticePolicy) LikeRecipient(actorID, authorID uint) bool {
	return authorID != 0 && authorID != actorID
}
