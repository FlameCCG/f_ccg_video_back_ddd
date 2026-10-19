package video

import (
	"database/sql"
	"path"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/media"

	"gorm.io/gorm"
)

// legacyStoragePathReferenced 兼容旧资源行中的完整 URL 和 bucket 前缀。
// 不能直接用 LIKE 的结果决定保留：查询参数和同名不同目录的文件不构成引用。
func legacyStoragePathReferenced(db *gorm.DB, storedPath string) (bool, error) {
	normalized := media.NormalizeStoredPath(storedPath)
	if normalized == "" || normalized == "/" {
		return false, nil
	}
	// 源文件名含哈希，先按文件名缩小候选，再按完整对象路径核对。
	// 显式转义 LIKE 通配符，避免 _source_ 中的下划线被当作任意字符。
	escape := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	pattern := "%" + escape.Replace(path.Base(normalized)) + "%"
	for _, target := range []struct{ table, column string }{
		{"videos", "cover"},
		{"video_resources", "file_url"},
		{"video_part_resources", "file_url"},
	} {
		found, err := legacyStorageReferenceInTable(db, target.table, target.column, pattern, normalized)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func legacyStorageReferenceInTable(db *gorm.DB, table, column, pattern, normalized string) (bool, error) {
	// 表名和列名只来自上面的固定列表；路径始终作为查询参数传入。
	rows, err := db.Table(table).Select(column).Where(column+" LIKE ? ESCAPE '!'", pattern).Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var stored sql.NullString
		if err := rows.Scan(&stored); err != nil {
			return false, err
		}
		if stored.Valid && media.StoredPathMatchesRelative(stored.String, normalized) {
			return true, nil
		}
	}
	return false, rows.Err()
}
