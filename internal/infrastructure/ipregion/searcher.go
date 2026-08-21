// Package ipregion 提供 ip2region 归属地查询器的构建与关闭（原 core/init_ip_db.go）。
package ipregion

import (
	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"go.uber.org/zap"
)

// dbPath ip2region 数据文件路径（与原实现一致，相对工作目录）。
const dbPath = "pkg/xdb/ip2region.xdb"

// Searcher 包装 ip2region 查询器，负责文件句柄生命周期。
//
// 初始化失败时 searcher 为 nil（与原实现一致：仅打错误日志，不阻断启动）。
type Searcher struct {
	searcher *xdb.Searcher
	logger   *zap.Logger
}

// XDB 返回底层查询器（可能为 nil）。
func (s *Searcher) XDB() *xdb.Searcher {
	if s == nil {
		return nil
	}

	return s.searcher
}

// New 打开 ip2region 数据文件。
func New(logger *zap.Logger) *Searcher {
	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, dbPath)
	if err != nil {
		logger.Error("初始化ip数据库失败", zap.Error(err))

		return &Searcher{logger: logger}
	}

	return &Searcher{searcher: searcher, logger: logger}
}

// Stop 关闭 ip2region 文件句柄。
func (s *Searcher) Stop() {
	if s == nil || s.searcher == nil {
		return
	}

	s.searcher.Close()
	s.searcher = nil
}

// ProvideXDB 取出底层查询器供归属地解析器注入（可能为 nil）。
func ProvideXDB(s *Searcher) *xdb.Searcher {
	return s.XDB()
}

// Provide 供 wire 使用的 provider：返回 ip2region 查询器与其资源关闭函数。
func Provide(logger *zap.Logger) (*Searcher, func()) {
	s := New(logger)

	return s, s.Stop
}
