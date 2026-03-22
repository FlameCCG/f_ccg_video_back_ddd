package comment

type NoticePolicy struct{}

// 视频作者是否存在由查询投影表达，无需额外检查 ID 非零。
func (NoticePolicy) VideoAuthor(actorID, authorID uint) bool { return authorID != actorID }
func (NoticePolicy) DynamicAuthor(actorID, authorID uint) bool {
	return authorID > 0 && authorID != actorID
}
func (NoticePolicy) Reply(actorID, receiverID uint) bool {
	return receiverID > 0 && receiverID != actorID
}
func (c *Comment) ShouldNotifyLike(actorID uint) bool { return c != nil && c.userID != actorID }

// LastByReceiver 保留接收者的首次顺序，后写覆盖先写，使回复通知优先于作者通知。
func (NoticePolicy) LastByReceiver(receivers []uint) []int {
	order := make([]uint, 0, len(receivers))
	last := make(map[uint]int, len(receivers))
	for index, id := range receivers {
		if _, exists := last[id]; !exists {
			order = append(order, id)
		}
		last[id] = index
	}
	indices := make([]int, 0, len(order))
	for _, id := range order {
		indices = append(indices, last[id])
	}
	return indices
}

func (NoticePolicy) AtRecipients(receivers []uint) []uint {
	selected := make([]uint, 0, len(receivers))
	seen := make(map[uint]struct{}, len(receivers))
	for _, id := range receivers {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		selected = append(selected, id)
	}
	return selected
}
