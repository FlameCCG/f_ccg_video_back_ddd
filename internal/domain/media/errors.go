package media

import "errors"

// ErrVideoDeleted 视频被硬删除导致转码任务作废时的取消原因。
// 仓储与应用层用 errors.Is 识别；超时与永久消息分类在 application/media/runtime。
var ErrVideoDeleted = errors.New("transcode task canceled because video was deleted")
