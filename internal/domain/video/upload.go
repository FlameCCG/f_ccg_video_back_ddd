package video

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxSourceFileNameLen 与 video_resources / video_part_resources.source_file_name 列长一致。
const MaxSourceFileNameLen = 255

// maxTotalChunks 单次合并允许的最大分片数（与原 validateCompleteUpload 一致）。
const maxTotalChunks = 100000

// ValidateSourceFileName 校验投稿源文件名长度，避免写入 DB 时触发 Data too long。
//
// 与原 api/video_api/publish.go 的 validateSourceFileName 逐字一致（含展示截断规则）。
func ValidateSourceFileName(fileName string) error {
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

// MapSourceFileNameDBError 将源文件名列超长等 DB 错误转为可读业务文案。
func MapSourceFileNameDBError(err error, sourceFileName string) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	if strings.Contains(msg, "source_file_name") &&
		(strings.Contains(msg, "Data too long") || strings.Contains(msg, "1406")) {
		if vErr := ValidateSourceFileName(sourceFileName); vErr != nil {
			return vErr
		}

		return fmt.Errorf(
			"源文件名过长（最多 %d 个字符）：请缩短文件名后重新选择并上传",
			MaxSourceFileNameLen,
		)
	}

	return err
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
func ValidateCompleteUpload(fileName string, totalChunks int) error {
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
func ValidateChunkSet(chunks []string, totalChunks int, validateIndex func(string) error) error {

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
func DefaultPartTitleFromFileName(fileName string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return "P1"
	}

	if ext := pathExtOf(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "P1"
	}

	return name
}

// IsContentFileHash 判断字符串是否为 32/64 位十六进制内容哈希。
func IsContentFileHash(s string) bool {
	if n := len(s); n != 32 && n != 64 {
		return false
	}

	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}

	return true
}
