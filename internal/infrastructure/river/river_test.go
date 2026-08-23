package river

import (
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/infrastructure/river/rule"
)

// 迁移自原 service/river_service/river_test.go。
//
// 原测试通过 global.SetConfig 注入配置；新实现的 validateRiverSchemas 直接接收
// *config.River 与 *config.Mysql 形参，因此改为构造入参并断言就地补全后的字段。

const (
	testTableVideoTags = "video_tags"
	testTableVideos    = "videos"
	testDBName         = "f_ccg_video_back"
)

func TestNormalizeConfiguredTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "camelCase join table", input: "videoTags", want: testTableVideoTags},
		{name: "camelCase user focus", input: "userFocus", want: "user_focus"},
		{name: "snake case unchanged", input: testTableVideoTags, want: testTableVideoTags},
		{name: "lowercase unchanged", input: testTableVideos, want: testTableVideos},
		{name: "regex preserved", input: "video.*", want: "video.*"},
		{name: "wildcard preserved", input: "*", want: "*"},
		{name: "trim spaces", input: " userFocus ", want: "user_focus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeConfiguredTableName(tt.input); got != tt.want {
				t.Fatalf("normalizeConfiguredTableName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateRiverSchemasDefaultsToMysqlDB(t *testing.T) {
	t.Parallel()

	riverCfg := &config.River{
		Sources: []config.RiverSource{
			{Tables: []string{testTableVideos, "users"}},
		},
		Rules: []*rule.Rule{
			{Table: testTableVideos},
			{Table: "users"},
		},
	}

	if err := validateRiverSchemas(riverCfg, &config.Mysql{DBName: testDBName}); err != nil {
		t.Fatalf("validateRiverSchemas failed: %v", err)
	}

	if got := riverCfg.Sources[0].Schema; got != testDBName {
		t.Fatalf("source schema = %q, want %q", got, testDBName)
	}

	for i, rr := range riverCfg.Rules {
		if rr.Schema != testDBName {
			t.Fatalf("rules[%d].Schema = %q, want %q", i, rr.Schema, testDBName)
		}
	}
}

func TestValidateRiverSchemasRejectsMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sources []config.RiverSource
		rules   []*rule.Rule
		want    string
	}{
		{
			name:    "source schema mismatch",
			sources: []config.RiverSource{{Schema: "f_ccg_video", Tables: []string{testTableVideos}}},
			rules:   []*rule.Rule{{Schema: testDBName, Table: testTableVideos}},
			want:    "source[0]",
		},
		{
			name:    "rule schema mismatch",
			sources: []config.RiverSource{{Schema: testDBName, Tables: []string{testTableVideos}}},
			rules:   []*rule.Rule{{Schema: "f_ccg_video", Table: testTableVideos}},
			want:    "rules[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateRiverSchemas(
				&config.River{Sources: tt.sources, Rules: tt.rules},
				&config.Mysql{DBName: testDBName},
			)
			if err == nil {
				t.Fatal("expected schema mismatch error, got nil")
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}

// TestValidateRiverSchemasRequiresMysqlDB mysql.db 缺失时必须直接报错，
// 否则 river 会把规则同步到一个未定义的 schema。
func TestValidateRiverSchemasRequiresMysqlDB(t *testing.T) {
	t.Parallel()

	err := validateRiverSchemas(
		&config.River{Sources: []config.RiverSource{{Tables: []string{testTableVideos}}}},
		&config.Mysql{},
	)
	if err == nil {
		t.Fatal("expected error when mysql.db is empty")
	}

	if !strings.Contains(err.Error(), "mysql db is required") {
		t.Fatalf("error = %q", err.Error())
	}
}

// TestValidateRiverSchemasRejectsNilRule 规则切片里出现 nil 元素时报错并指明下标，
// 避免后续 prepareRule 解引用 nil。
func TestValidateRiverSchemasRejectsNilRule(t *testing.T) {
	t.Parallel()

	err := validateRiverSchemas(
		&config.River{Rules: []*rule.Rule{nil}},
		&config.Mysql{DBName: testDBName},
	)
	if err == nil {
		t.Fatal("expected error for nil rule")
	}

	if !strings.Contains(err.Error(), "rules[0] is nil") {
		t.Fatalf("error = %q", err.Error())
	}
}

// TestNewRiverRejectsDisabledOrMissingConfig 未启用或缺少必需配置段时构造必须失败，
// 由 RiverComponent.Start 捕获并只记 "init river failed"，不阻断启动。
func TestNewRiverRejectsDisabledOrMissingConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		riverCfg *config.River
		mysqlCfg *config.Mysql
		esCfg    *config.ES
	}{
		{name: "nil river config", riverCfg: nil, mysqlCfg: &config.Mysql{}, esCfg: &config.ES{}},
		{name: "river disabled", riverCfg: &config.River{Enable: false}, mysqlCfg: &config.Mysql{}, esCfg: &config.ES{}},
		{name: "missing mysql config", riverCfg: &config.River{Enable: true}, mysqlCfg: nil, esCfg: &config.ES{}},
		{name: "missing es config", riverCfg: &config.River{Enable: true}, mysqlCfg: &config.Mysql{}, esCfg: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewRiver(tt.riverCfg, tt.mysqlCfg, tt.esCfg, nil); err == nil {
				t.Fatal("expected NewRiver to fail")
			}
		})
	}
}

func TestIsValidTables(t *testing.T) {
	t.Parallel()

	if !isValidTables([]string{"*"}) {
		t.Fatal("single wildcard should be valid")
	}

	if !isValidTables([]string{testTableVideos, "users"}) {
		t.Fatal("explicit table list should be valid")
	}

	// 多表配置里混入 "*" 会把同步范围放大到整库，必须拒绝。
	if isValidTables([]string{testTableVideos, "*"}) {
		t.Fatal("wildcard mixed with explicit tables should be rejected")
	}
}

func TestBuildTable(t *testing.T) {
	t.Parallel()

	if got := buildTable("*"); got != ".*" {
		t.Fatalf("buildTable(\"*\") = %q, want %q", got, ".*")
	}

	if got := buildTable(testTableVideos); got != testTableVideos {
		t.Fatalf("buildTable(%q) = %q", testTableVideos, got)
	}
}

func TestRuleKeyIsLowerCase(t *testing.T) {
	t.Parallel()

	if got := ruleKey("F_CCG", "Videos"); got != "f_ccg:videos" {
		t.Fatalf("ruleKey = %q, want %q", got, "f_ccg:videos")
	}
}
