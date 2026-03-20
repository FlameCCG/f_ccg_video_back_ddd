package video

import (
	"context"
)

// AdminVideoQuery 管理端视频列表读模型查询端口。
type AdminVideoQuery interface {
	List(ctx context.Context, params AdminVideoListParams) ([]AdminVideoItem, int64, error)
}
