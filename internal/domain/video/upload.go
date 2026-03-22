package video

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxSourceFileNameLen 与 video_resources / video_part_resources.source_file_name 列长一致。
const MaxSourceFileNameLen = 255

// maxTotalChunks 单次合并允许的最大分片数。
const maxTotalChunks = 100000

// ValidateSourceFileName 校验源文件名长度并提供展示截断规则，避免数据库写入发生 Data too long。
// UploadPolicy 校验投稿上传的文件名、分片集合与分 P 数量。
type UploadPolicy struct{}

// DailyLimit 使用当次配置中的不同源文件额度，缺省或非正数沿用每日十个。
func (UploadPolicy) DailyLimit(configured int) int {
	if configured > 0 {
		return configured
	}
	return 10
}

func (UploadPolicy) EnsureDailyClaim(allowed bool, limit int) error {
	if !allowed {
		return fmt.Errorf("%w（每日最多 %d 个不同文件）", ErrDailyUploadLimit, limit)
	}
	return nil
}

func (UploadPolicy) SourceFileName(fileName string) error {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return errSourceFileNameEmpty
	}

	runeLen := utf8.RuneCountInString(name)
	if runeLen > MaxSourceFileNameLen {
		display := name
		if runeLen > 40 {
			// 过长时只展示前后各 16 个字符，便于用户识别文件
			runes := []rune(name)
			display = string(runes[:16]) + "..." + string(runes[len(runes)-16:])
		}

		return fmt.Errorf(
			"源文件名过长（最多 %d 个字符，当前 %d 个）：%s。请缩短文件名后重新选择并上传",
			MaxSourceFileNameLen,
			runeLen,
			display,
		)
	}

	return nil
}

// IsSourceFileNameError 判断错误是否为源文件名相关的业务错误。
func IsSourceFileNameError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "源文件名")
}

// ValidateCompleteUpload 校验合并分片请求（文件名与分片总数）。
//
// fileHash 的合法性由存储端口的 ValidateUploadID 负责（属媒体上下文能力）。
func (UploadPolicy) Complete(fileName string, totalChunks int) error {
	if fileName == "" {
		return errFileNameEmpty
	}

	if strings.ContainsAny(fileName, `/\`) {
		return errFileNameInvalid
	}

	if totalChunks <= 0 {
		return errTotalChunksInvalid
	}

	if totalChunks > maxTotalChunks {
		return errTotalChunksTooLarge
	}

	return nil
}

// ValidateChunkSet 校验已上传分片集合是否覆盖 0..totalChunks-1。
//
// validateIndex 由调用方注入（分片索引格式校验属存储端口能力）。
func (UploadPolicy) ChunkSet(chunks []string, totalChunks int, validateIndex func(string) error) error {

	seen := make(map[string]struct{}, len(chunks))

	for _, chunk := range chunks {
		if validateIndex != nil {
			if err := validateIndex(chunk); err != nil {
				return err
			}
		}

		seen[chunk] = struct{}{}
	}

	for i := range totalChunks {
		index := strconv.Itoa(i)
		if _, ok := seen[index]; !ok {
			return fmt.Errorf("缺少分片 %s，请重新上传后重试", index)
		}
	}

	return nil
}

// DefaultPartTitleFromFileName 由源文件名推导分P默认标题，兜底 "P1"。
func (UploadPolicy) DefaultPartTitle(fileName string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return "P1"
	}

	if ext := (ResourcePolicy{}).fileExt(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "P1"
	}

	return name
}

// PartLimit 拒绝超过单作品分P上限的投稿。
func (UploadPolicy) PartLimit(count, maxParts int) error {
	if count > maxParts {
		return fmt.Errorf("单个作品最多上传 %d 个分P", maxParts)
	}

	return nil
}
