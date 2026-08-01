package ipregion

import (
	"errors"
	"fmt"
	"strings"

	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"go.uber.org/zap"
)

// regionFieldCount ip2region 记录固定为「国家|区域|省份|城市|运营商」五段。
const regionFieldCount = 5

// Resolver 基于 ip2region 的归属地解析器，实现 ipUtils.RegionResolver。
// 替代原 pkg/utils/ipUtils.GetIpAddr 对 global.IPSearcher 的全局引用。
type Resolver struct {
	searcher *xdb.Searcher
	logger   *zap.Logger
}

// 编译期断言：Resolver 满足 pkg 侧声明的归属地解析端口。
var _ ipUtils.RegionResolver = (*Resolver)(nil)

// NewResolver 构造归属地解析器；searcher 为 nil 时所有公网 IP 返回「未知地址」。
func NewResolver(searcher *xdb.Searcher, logger *zap.Logger) *Resolver {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Resolver{searcher: searcher, logger: logger}
}

// Region 返回 IP 归属地，语义与原 ipUtils.GetIpAddr 完全一致。
func (r *Resolver) Region(ip string) string {
	if ipUtils.HasLocalIPAddr(ip) {
		return "内网"
	}

	if r == nil || r.searcher == nil {
		return "未知地址"
	}

	region, err := r.searcher.SearchByStr(ip)
	if err != nil {
		r.logger.Warn("ipUtils.GetIpAddr failed", zap.Error(err))

		return "异常地址"
	}

	addrList := strings.Split(region, "|")
	if len(addrList) != regionFieldCount {
		r.logger.Warn("ipUtils.GetIpAddr failed", zap.Error(errors.New("addrList len != 5")))

		return "未知地址"
	}
	// addrList 五个部分
	// 国家 0 省份 市 运营商
	country := addrList[0]
	province := addrList[2]

	city := addrList[3]
	if province != "0" && city != "0" {
		return fmt.Sprintf("%s·%s", province, city)
	}

	if country != "0" && province != "0" {
		return fmt.Sprintf("%s·%s", country, province)
	}

	if country != "0" {
		return country
	}

	return region
}
