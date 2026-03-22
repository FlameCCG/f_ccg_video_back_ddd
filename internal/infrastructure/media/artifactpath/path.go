package artifactpath

import (
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// NormalizeStoredPath 把存储路径 / 完整 URL 归一为 path.Clean 后的对象路径。
// 无法解析或只指向根目录时返回空串。
type Policy struct{}

func NewPolicy() *Policy { return &Policy{} }

func (Policy) NormalizeStoredPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" && u.Host != "" {
		trimmed = u.Path
		if unescaped, err := url.PathUnescape(trimmed); err == nil {
			trimmed = unescaped
		}
	}

	cleaned := path.Clean(strings.TrimSpace(trimmed))
	if cleaned == "." {
		return ""
	}

	return cleaned
}

// StoredPathMatchesRelative 判断资源行 file_url（可能是完整 URL / 带 bucket 前缀路径）
// 是否指向期望的相对产物路径。产物路径由 FileHash 派生，源内容变化会自然失配。
func (Policy) StoredPathMatchesRelative(storedURL, relativePath string) bool {
	stored := (Policy{}).NormalizeStoredPath(storedURL)
	rel := (Policy{}).NormalizeStoredPath(relativePath)

	if stored == "" || rel == "" {
		return false
	}

	stored = "/" + strings.TrimPrefix(stored, "/")
	rel = "/" + strings.TrimPrefix(rel, "/")

	return strings.HasSuffix(stored, rel)
}

// pathBase 返回存储路径/URL 的最后一段文件名（兼容 Windows 反斜杠）。
func (Policy) pathBase(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" {
		trimmed = u.Path
	}

	trimmed = strings.ReplaceAll(trimmed, `\`, "/")

	return path.Base(strings.TrimRight(trimmed, "/"))
}

// ShouldDeleteOriginalSource 判断任务收尾时是否应删除原始源对象：
// 源路径为空不删；规范化后 URL 为空视为源已被替换（删）；两者不同则删。
func (Policy) ShouldDeleteOriginalSource(initialSourceURL, currentSourceURL string) bool {
	initial := (Policy{}).NormalizeStoredPath(initialSourceURL)
	current := (Policy{}).NormalizeStoredPath(currentSourceURL)

	if initial == "" {
		return false
	}

	if current == "" {
		return true
	}

	return initial != current
}

// SourceTargetMatchesInput 判断规范化目标是否与输入源同名（会覆盖输入，必须拒绝）。
func (Policy) SourceTargetMatchesInput(srcPath, relativePath string) bool {
	sourceBase := (Policy{}).pathBase(srcPath)
	targetBase := (Policy{}).pathBase(relativePath)

	return sourceBase != "" && targetBase != "" && strings.EqualFold(sourceBase, targetBase)
}

// ValidateSourceTargetIsolation 拒绝会覆盖输入源对象的规范化目标。
func (p Policy) ValidateSourceTargetIsolation(srcPath string, targets []mediadomain.TranscodeTarget) error {
	for _, target := range targets {
		if !target.IsSource {
			continue
		}

		if p.SourceTargetMatchesInput(srcPath, target.RelativePath) {
			return fmt.Errorf("source normalization target must not overwrite input: %s", target.RelativePath)
		}
	}

	return nil
}
