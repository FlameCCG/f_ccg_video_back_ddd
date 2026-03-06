package enum

// CommentType 标识评论归属的作品种类，字符串也是查询过滤与持久化契约。
type CommentType string

const (
	// VideoCommentType 表示视频评论，目标通过 video_id 指定。
	VideoCommentType CommentType = "video" // 在视频中评论
	// DynamicCommentType 表示动态评论，目标通过 dynamic_id 指定。
	DynamicCommentType CommentType = "dynamic" // 在动态中评论
	// ArticleCommentType 保留专栏评论的类型值；枚举存在不表示相关入口已经完整实现。
	ArticleCommentType CommentType = "article" // 在专栏中评论
)
