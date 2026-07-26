package ipregion

import (
	"testing"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// xdbTestPath 从本包目录出发指向仓库内的 ip2region 数据文件。
const xdbTestPath = "../../../pkg/xdb/ip2region.xdb"

// TestResolverRegionPublicIP 用真实 xdb 数据校验公网 IP 能解析出非空归属地。
// 对应原 pkg/utils/ipUtils 中依赖 global.IPSearcher 的用例。
func TestResolverRegionPublicIP(t *testing.T) {
	t.Parallel()

	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, xdbTestPath)
	if err != nil {
		t.Fatalf("init ip searcher failed: %v", err)
	}

	t.Cleanup(searcher.Close)

	resolver := NewResolver(searcher)

	addr := resolver.Region("8.8.8.8")
	if addr == "" {
		t.Fatal("expected non-empty addr for public ip")
	}

	if addr == "内网" || addr == "异常地址" {
		t.Fatalf("unexpected addr for public ip: %s", addr)
	}
}

// TestResolverRegionPrivateIP 内网地址不查库，直接返回「内网」。
func TestResolverRegionPrivateIP(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(nil)

	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.0.1", "169.254.1.1"} {
		if got := resolver.Region(ip); got != "内网" {
			t.Fatalf("Region(%q) = %q, want 内网", ip, got)
		}
	}
}

// TestResolverRegionWithoutSearcher searcher 缺失时公网 IP 返回「未知地址」，
// 与原 ipUtils.GetIpAddr 在 global.IPSearcher 为 nil 时的行为一致。
func TestResolverRegionWithoutSearcher(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(nil)

	if got := resolver.Region("8.8.8.8"); got != "未知地址" {
		t.Fatalf("Region without searcher = %q, want 未知地址", got)
	}
}

// TestResolverRegionInvalidIP 非法 IP 交给 xdb 会报错，落到「异常地址」分支。
func TestResolverRegionInvalidIP(t *testing.T) {
	t.Parallel()

	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, xdbTestPath)
	if err != nil {
		t.Fatalf("init ip searcher failed: %v", err)
	}

	t.Cleanup(searcher.Close)

	resolver := NewResolver(searcher)

	if got := resolver.Region("not-an-ip"); got != "异常地址" {
		t.Fatalf("Region(invalid) = %q, want 异常地址", got)
	}
}

// TestResolverNilReceiver nil 解析器不应 panic。
func TestResolverNilReceiver(t *testing.T) {
	t.Parallel()

	var resolver *Resolver

	if got := resolver.Region("8.8.8.8"); got != "未知地址" {
		t.Fatalf("nil resolver Region = %q, want 未知地址", got)
	}
}
