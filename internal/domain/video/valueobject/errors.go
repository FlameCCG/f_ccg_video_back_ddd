package valueobject

import "errors"

// 上传相关错误。
var errSourceFileNameEmpty = errors.New("源文件名不能为空")
var errFileNameEmpty = errors.New("fileName 不能为空")
var errFileNameInvalid = errors.New("fileName 格式无效")
var errTotalChunksInvalid = errors.New("totalChunks 必须大于 0")
var errTotalChunksTooLarge = errors.New("totalChunks 超出限制")

var errNotAbsoluteURL = errors.New("not an absolute url")
