package router

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// goldenRoutesPath 原项目路由的金标准清单（从基线 router 包静态提取，见 docs/decisions.md D-014）。
const goldenRoutesPath = "testdata/routes_golden.txt"

// TestRoutesMatchLegacyGolden 路由奇偶校验：
// 用真实 gin 引擎注册全部路由，导出 method+path 清单，与原项目金标准逐条比对，差异必须为零。
//
// 处理器全部传 nil：路由注册只取方法值（method value），不会调用处理器本身，
// 因此无需任何数据库/缓存连接即可完成校验。
func TestRoutesMatchLegacyGolden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := New(
		infraconfig.NewProvider(&config.Config{}),
		nil,           // middlewares：路由注册期只取方法值，不会执行
		nil,           // validator：路由清单测试不执行参数绑定
		nil, nil, nil, // identity / identityAdmin / captcha
		nil,      // contentOps
		nil,      // ai
		nil,      // social
		nil, nil, // video / videoAdmin
		nil, // comment
		nil, // chat
		nil, // notification
		nil, // dynamic
		nil, // rbac
	)

	engine := router.BuildEngine()

	got := make([]string, 0, len(engine.Routes()))
	for _, route := range engine.Routes() {
		got = append(got, fmt.Sprintf("%s %s", route.Method, route.Path))
	}

	sort.Strings(got)

	want := loadGoldenRoutes(t)

	if diff := diffRouteLists(want, got); diff != "" {
		t.Fatalf("路由清单与原项目金标准不一致：\n%s", diff)
	}

	if len(got) != len(want) {
		t.Fatalf("路由条数不一致：新 %d 条，原 %d 条", len(got), len(want))
	}
}

// loadGoldenRoutes 读取金标准清单，规范化为 "METHOD /path" 形式。
func loadGoldenRoutes(t *testing.T) []string {
	t.Helper()

	file, err := os.Open(goldenRoutesPath)
	if err != nil {
		t.Fatalf("打开金标准清单失败: %v", err)
	}

	defer func() { _ = file.Close() }()

	var routes []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)

		const expectedFields = 2
		if len(fields) != expectedFields {
			t.Fatalf("金标准清单格式非法: %q", line)
		}

		routes = append(routes, fields[0]+" "+fields[1])
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("读取金标准清单失败: %v", err)
	}

	sort.Strings(routes)

	return routes
}

// diffRouteLists 返回两份有序路由清单的差异描述；完全一致时返回空串。
func diffRouteLists(want, got []string) string {
	wantSet := make(map[string]struct{}, len(want))
	for _, route := range want {
		wantSet[route] = struct{}{}
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, route := range got {
		gotSet[route] = struct{}{}
	}

	var builder strings.Builder

	for _, route := range want {
		if _, ok := gotSet[route]; !ok {
			builder.WriteString("  - 缺失（原有新无）: " + route + "\n")
		}
	}

	for _, route := range got {
		if _, ok := wantSet[route]; !ok {
			builder.WriteString("  + 多出（新有原无）: " + route + "\n")
		}
	}

	return builder.String()
}
