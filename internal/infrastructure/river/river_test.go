package river

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/infrastructure/river/rule"
	"strings"
	"testing"
	"time"

	"github.com/siddontang/go-mysql/schema"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

			{
				got := normalizeConfiguredTableName(tt.input)
				require.Equal(t, tt.want, got, "normalizeConfiguredTableName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateRiverSchemasDefaultsToMysqlDB(t *testing.T) {
	t.Parallel()

	riverCfg := &config.River{
		Sources: []config.RiverSource{
			{Tables: []string{testTableVideos, tableUsers}},
		},
		Rules: []*rule.Rule{
			{Table: testTableVideos},
			{Table: tableUsers},
		},
	}

	{
		err := validateRiverSchemas(riverCfg, &config.Mysql{DBName: testDBName})
		require.NoError(t, err, "validateRiverSchemas failed: %v", err)
	}

	{
		got := riverCfg.Sources[0].Schema
		require.Equal(t, testDBName, got, "source schema = %q, want %q", got, testDBName)
	}

	for i, rr := range riverCfg.Rules {
		require.Equal(t, testDBName, rr.Schema, "rules[%d].Schema = %q, want %q", i, rr.Schema, testDBName)
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
			sources: []config.RiverSource{{Schema: testFCcgVideo, Tables: []string{testTableVideos}}},
			rules:   []*rule.Rule{{Schema: testDBName, Table: testTableVideos}},
			want:    "source[0]",
		},
		{
			name:    "rule schema mismatch",
			sources: []config.RiverSource{{Schema: testDBName, Tables: []string{testTableVideos}}},
			rules:   []*rule.Rule{{Schema: testFCcgVideo, Table: testTableVideos}},
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
			require.Error(t, err, "expected schema mismatch error, got nil")

			require.Contains(t, err.Error(), tt.want, "error = %q, want it to contain %q", err.Error(), tt.want)
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
	require.Error(t, err, "expected error when mysql.db is empty")

	require.Contains(t, err.Error(), "mysql db is required", "error = %q", err.Error())
}

// TestValidateRiverSchemasRejectsNilRule 规则切片里出现 nil 元素时报错并指明下标，
// 避免后续 prepareRule 解引用 nil。
func TestValidateRiverSchemasRejectsNilRule(t *testing.T) {
	t.Parallel()

	err := validateRiverSchemas(
		&config.River{Rules: []*rule.Rule{nil}},
		&config.Mysql{DBName: testDBName},
	)
	require.Error(t, err, "expected error for nil rule")

	require.Contains(t, err.Error(), "rules[0] is nil", "error = %q", err.Error())
}

// TestNewRiverRejectsDisabledOrMissingConfig 未启用或缺少必需配置段时构造必须失败，
// 由 Component.Start 捕获并只记 "init river failed"，不阻断启动。
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

			{
				_, err := NewRiver(tt.riverCfg, tt.mysqlCfg, tt.esCfg, nil, zap.NewNop())
				require.Error(t, err, "expected NewRiver to fail")
			}
		})
	}
}

func TestIsValidTables(t *testing.T) {
	t.Parallel()

	require.True(t, isValidTables([]string{"*"}), "single wildcard should be valid")

	require.True(t, isValidTables([]string{testTableVideos, tableUsers}), "explicit table list should be valid")

	// 多表配置里混入 "*" 会把同步范围放大到整库，必须拒绝。
	require.False(t, isValidTables([]string{testTableVideos, "*"}), "wildcard mixed with explicit tables should be rejected")
}

func TestBuildTable(t *testing.T) {
	t.Parallel()

	{
		got := buildTable("*")
		require.Equal(t, ".*", got, "buildTable(\"*\") = %q, want %q", got, ".*")
	}

	{
		got := buildTable(testTableVideos)
		require.Equal(t, testTableVideos, got, "buildTable(%q) = %q", testTableVideos, got)
	}
}

func TestMysqlUTF8LiteralDoesNotExposeQuotes(t *testing.T) {
	t.Parallel()

	literal := mysqlUTF8Literal(`video".*' OR 1=1 --`)
	require.False(t, strings.ContainsAny(literal, `"'`), "literal contains raw quote: %s", literal)

	require.True(t, strings.HasPrefix(literal, "CONVERT(0x"), "unexpected literal encoding: %s", literal)
	require.True(t, strings.HasSuffix(literal, " USING utf8mb4)"), "unexpected literal encoding: %s", literal)
}

func TestRiverDatetimeUsesMysqlLocation(t *testing.T) {
	t.Parallel()

	r := &River{mysqlLoc: time.FixedZone("database", 9*60*60)}
	column := &schema.TableColumn{Type: schema.TYPE_DATETIME}

	got := r.makeReqColumnData(column, "2024-01-02 03:04:05")
	require.EqualValues(t, "2024-01-02T03:04:05+09:00", got, "datetime = %v, want explicit database offset", got)
}

func TestRuleKeyIsLowerCase(t *testing.T) {
	t.Parallel()

	{
		got := ruleKey("F_CCG", "Videos")
		require.Equal(t, "f_ccg:videos", got, "ruleKey = %q, want %q", got, "f_ccg:videos")
	}
}
