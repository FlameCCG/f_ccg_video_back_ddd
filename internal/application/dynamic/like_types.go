package dynamic

// DynamicLikeResult 动态点赞切换结果（读模型）.
type DynamicLikeResult struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}
