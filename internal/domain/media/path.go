package media

import (
	"net/url"
	"path"
	"strings"
)

// NormalizeStoredPath 把存储路径 / 完整 URL 归一为 path.Clean 后的对象路径。
// 无法解析或只指向根目录时返回空串。
func NormalizeStoredPath(raw string) string {
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
func StoredPathMatchesRelative(storedURL, relativePath string) bool {
	stored := NormalizeStoredPath(storedURL)
	rel := NormalizeStoredPath(relativePath)

	if stored == "" || rel == "" {
		return false
	}

	stored = "/" + strings.TrimPrefix(stored, "/")
	rel = "/" + strings.TrimPrefix(rel, "/")

	return strings.HasSuffix(stored, rel)
}

// StoredPathBase 返回存储路径/URL 的最后一段文件名（兼容 Windows 反斜杠）。
func StoredPathBase(raw string) string {
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
func ShouldDeleteOriginalSource(initialSrcPath, currentSourceURL string) bool {
	initial := NormalizeStoredPath(initialSrcPath)
	current := NormalizeStoredPath(currentSourceURL)

	if initial == "" {
		return false
	}

	if current == "" {
		return true
	}

	return initial != current
}

// SourceTargetMatchesInput 判断规范化目标是否与输入源同名（会覆盖输入，必须拒绝）。
func SourceTargetMatchesInput(srcPath, targetPath string) bool {
	sourceBase := StoredPathBase(srcPath)
	targetBase := StoredPathBase(targetPath)

	return sourceBase != "" && targetBase != "" && strings.EqualFold(sourceBase, targetBase)
}
