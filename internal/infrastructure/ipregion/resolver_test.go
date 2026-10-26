package ipregion

import (
	"sync"
	"testing"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// xdbTestPath 从本包目录出发指向仓库内的 ip2region 数据文件。
const xdbTestPath = "../../../pkg/xdb/ip2region.xdb"

// TestResolverRegionPublicIP 用真实 xdb 数据校验公网 IP 能解析出非空归属地。
// 对应原 pkg/utils/ipUtils 中依赖 global.IPSearcher 的用例。
func TestResolverRegionPublicIP(t *testing.T) {
	t.Parallel()

	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, xdbTestPath)
	require.NoError(t, err, "init ip searcher failed: %v", err)

	t.Cleanup(searcher.Close)

	resolver := NewResolver(searcher, zap.NewNop())

	addr := resolver.Region("8.8.8.8")
	require.NotEmpty(t, addr, "expected non-empty addr for public ip")

	require.NotEqual(t, "内网", addr, "unexpected addr for public ip: %s", addr)
	require.NotEqual(t, "异常地址", addr, "unexpected addr for public ip: %s", addr)
}

// TestResolverRegionPrivateIP 内网地址不查库，直接返回「内网」。
func TestResolverRegionPrivateIP(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(nil, zap.NewNop())

	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.0.1", "169.254.1.1"} {
		{
			got := resolver.Region(ip)
			require.Equal(t, "内网", got, "Region(%q) = %q, want 内网", ip, got)
		}
	}
}

// TestResolverRegionWithoutSearcher searcher 缺失时公网 IP 返回「未知地址」，
// 与原 ipUtils.GetIpAddr 在 global.IPSearcher 为 nil 时的行为一致。
func TestResolverRegionWithoutSearcher(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(nil, zap.NewNop())

	{
		got := resolver.Region("8.8.8.8")
		require.Equal(t, "未知地址", got, "Region without searcher = %q, want 未知地址", got)
	}
}

// TestResolverRegionInvalidIP 非法 IP 交给 xdb 会报错，落到「异常地址」分支。
func TestResolverRegionInvalidIP(t *testing.T) {
	t.Parallel()

	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, xdbTestPath)
	require.NoError(t, err, "init ip searcher failed: %v", err)

	t.Cleanup(searcher.Close)

	resolver := NewResolver(searcher, zap.NewNop())

	{
		got := resolver.Region("not-an-ip")
		require.Equal(t, "异常地址", got, "Region(invalid) = %q, want 异常地址", got)
	}
}

// TestResolverNilReceiver nil 解析器不应 panic。
func TestResolverNilReceiver(t *testing.T) {
	t.Parallel()

	var resolver *Resolver

	{
		got := resolver.Region("8.8.8.8")
		require.Equal(t, "未知地址", got, "nil resolver Region = %q, want 未知地址", got)
	}
}

func TestResolverConcurrentSearchesRemainConsistent(t *testing.T) {
	t.Parallel()

	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, xdbTestPath)
	require.NoError(t, err)

	defer searcher.Close()

	resolver := NewResolver(searcher, zap.NewNop())
	ips := []string{"8.8.8.8", "1.1.1.1", "114.114.114.114"}

	want := make(map[string]string)
	for _, ip := range ips {
		want[ip] = resolver.Region(ip)
	}

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for range 20 {
				for _, ip := range ips {
					{
						got := resolver.Region(ip)
						assert.Equal(t, got, want[ip], "%s: %q != %q", ip, got, want[ip])
					}
				}
			}
		}()
	}

	wg.Wait()
}
