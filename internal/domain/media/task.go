package media

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// TranscodeTask 是一次转码任务的领域实体。
//
// 线上 Kafka 消息的 JSON 结构由 infrastructure/mq 的 TranscodePayload 承载，
// 本类型只描述任务身份与业务派生规则，不带序列化 tag。
type TranscodeTask struct {
	// ResolutionList 投稿时配置的目标分辨率阶梯。
	ResolutionList []uint16
	// StorageType 源文件所在存储类型（本地 / MinIO）。
	StorageType enum.VideoStorageType
	// SourcePath 源文件路径（本地绝对路径或可访问 URL）。
	SourcePath string
	// OutputPath 产物在存储中的相对目录前缀。
	OutputPath string
	// FileHash 上传时的文件指纹，用于产物命名与断点目录寻址。
	FileHash string
	// FileName 用户上传时的原始文件名。
	FileName string
	// VideoID 所属视频 ID。
	VideoID uint
	// VideoPartID 所属分P ID；为 0 表示单P视频。
	VideoPartID uint
}

// Validate 校验任务的必要字段；缺失即为永久失败（不可重试）。
func (t TranscodeTask) Validate() error {
	if t.VideoID == 0 || t.SourcePath == "" || t.OutputPath == "" {
		return errors.New("invalid payload data")
	}

	return nil
}

// SourceDisplayName 返回落库用的源文件展示名（超长按 50 个字符截断）。
func (t TranscodeTask) SourceDisplayName() string {
	name := strings.TrimSpace(t.FileName)
	if name != "" {
		return t.truncateRunes(name, 50)
	}

	hash := strings.TrimSpace(t.FileHash)
	if hash == "" {
		return ""
	}

	return "_source_" + hash + ".mp4"
}

// NormalizedSourceStorageName 返回源片规范化的确定性对象名。
//
// 转码输出不得复用输入源对象 key（有 ValidateSourceTargetIsolation 兜底）；
// 确定性命名让重试原地覆盖同一对象，并支撑断点续传按名字匹配已提交/已入库的产物。
func (t TranscodeTask) NormalizedSourceStorageName() string {
	identity := strings.TrimSpace(t.FileHash)
	if identity == "" {
		identity = fmt.Sprintf("video-%d-part-%d", t.VideoID, t.VideoPartID)
	}

	return fmt.Sprintf("_normalized_%s.mp4", identity)
}

// NormalizedSourceRelativePath 返回源片规范化产物的相对存储路径。
func (t TranscodeTask) NormalizedSourceRelativePath() string {
	return path.Join(t.OutputPath, t.NormalizedSourceStorageName())
}

// DashPrefix 返回整套 DASH 产物的存储前缀。
func (t TranscodeTask) DashPrefix() string {
	return path.Join(t.OutputPath, "dash")
}

// DashManifestRelativePath 返回 DASH manifest 的相对存储路径。
func (t TranscodeTask) DashManifestRelativePath() string {
	return path.Join(t.OutputPath, "dash", DashManifestName)
}

// DashRepresentationPrefix 返回 DASH 初始化文件与媒体分片的命名前缀。
func (t TranscodeTask) DashRepresentationPrefix() string {
	fileHash := strings.TrimSpace(t.FileHash)
	if fileHash == "" {
		fileHash = "rep"
	}

	return fileHash
}

// TaskKey 返回任务的取消登记键。
func (t TranscodeTask) TaskKey() TaskKey {
	return TaskKey{VideoID: t.VideoID, PartID: t.VideoPartID}
}

// truncateRunes 按 rune 数截断字符串。
func (t TranscodeTask) truncateRunes(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}

	return string(r[:maxLen])
}
