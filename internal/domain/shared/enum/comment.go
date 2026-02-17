package enum

// CommentType 类型.
type CommentType string

// ArticleCommentType 及相关导出常量.
const (
	// VideoCommentType 常量.
	VideoCommentType CommentType = "video" // 在视频中评论
	// DynamicCommentType 常量.
	DynamicCommentType CommentType = "dynamic" // 在动态中评论
	// ArticleCommentType 常量.
	ArticleCommentType CommentType = "article" // 在专栏中评论
)
