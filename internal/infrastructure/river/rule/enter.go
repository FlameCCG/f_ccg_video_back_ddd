// Package rule 描述 MySQL 表到 Elasticsearch 索引和字段的映射。
//
// 规则供 River 解释表、主键、字段筛选与索引目标，不是领域实体的另一种定义。
// 变更映射需要同时检查索引 mapping、存量文档和增量转换，单改配置不会自动重建历史文档。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package rule

import ( // 导入所需标准库与第三方库
	"slices"
	"strings" // 字符串处理：大小写、去空格等
	"unicode"

	"github.com/siddontang/go-mysql/schema" // 引入 go-mysql 的表结构描述，用于拿到表/主键/字段信息
)

// Rule 是从 MySQL 同步到 ES 的映射规则抽象。 // 原理：增量同步时需要知道“源表字段 -> 目标索引字段”的映射与过滤等策略
// 如果想把 MySQL 数据写入 ES，必须配置一条规则告诉同步器如何做映射。 // 核心映射：schema+table <-> index+type（老版 ES 带 type 的概念）
// schema/table 属于 MySQL 侧，index/type 属于 Elasticsearch 侧。
type Rule struct { // 定义规则结构体
	Schema string   `mapstructure:"schema" yaml:"schema"` // 源库名；支持从配置中 via mapstructure/yaml 解析
	Table  string   `mapstructure:"table"  yaml:"table"`  // 源表名
	Index  string   `mapstructure:"index"  yaml:"index"`  // 目标 ES 索引名；缺省时默认用表名小写
	Type   string   `mapstructure:"type"  yaml:"type"`    // 目标 ES 文档 type；新版 ES 已弱化/废弃 type，这里仍保留兼容
	Parent string   `yaml:"-"`                            // 父文档 ID（ES 父子文档场景）；不从 yaml 直接反序列化
	ID     []string `yaml:"-"`                            // 组成文档 _id 的字段列表；不从 yaml 直接反序列化

	// 默认：MySQL 字段名直接映射为 ES 字段名。 // 但有时需要改名，比如 MySQL 是 title，在 ES 希望叫 my_title
	FieldMapping map[string]string `mapstructure:"field" yaml:"field,omitempty"` // 字段名映射表：mysql字段 -> es字段

	// MySQL 表的结构信息（由 canal 获取），包括主键、列类型等 // 原理：用于构造 ES 文档以及定位主键
	TableInfo *schema.Table `yaml:"-"` // 不参与 yaml 反序列化

	// Filter：仅同步这些字段；默认 nil 表示同步所有字段。 // 原理：降低 ES 字段冗余与写入成本
	Filter []string `mapstructure:"filter" yaml:"filter,omitempty"`

	// Elasticsearch pipeline 名称 // 原理：写入前通过 Ingest Pipeline 做清洗/转换/提取等预处理
	Pipeline string `mapstructure:"pipeline" yaml:"pipeline,omitempty"`
}

// NewDefaultRule 根据库表生成默认同步规则.
func NewDefaultRule(schema, table string) *Rule { // 工厂方法：根据库表生成默认规则
	r := new(Rule) // 分配 Rule

	r.Schema = schema // 设置源库
	r.Table = table   // 设置源表

	lowerTable := strings.ToLower(table) // 规则：默认索引与类型都用表名小写，以符合 ES 要求
	r.Index = lowerTable                 // 默认索引名 = 表名小写
	r.Type = lowerTable                  // 默认类型 = 表名小写（兼容）

	r.FieldMapping = make(map[string]string) // 初始化空的字段映射表

	return r // 返回默认规则
}

// Prepare 规范化规则字段映射、过滤白名单与索引/类型默认值.
func (r *Rule) Prepare() error { // 规则预处理：规范化大小写/空格、去重、设置默认值等
	if r.FieldMapping == nil { // 若未初始化字段映射
		r.FieldMapping = make(map[string]string) // 建一个空 map，避免后续 nil 访问
	}

	if len(r.FieldMapping) > 0 { // 若配置了字段重命名映射
		normalized := make(map[string]string, len(r.FieldMapping)) // 目标：生成规范化（小写键、去空格）的映射
		for k, v := range r.FieldMapping {                         // 遍历每个映射项
			mysqlField := normalizeConfiguredColumnName(k) // 兼容把 MySQL 字段写成 camelCase 的配置
			if mysqlField == "" {                          // 空键忽略
				continue
			}

			elasticField := strings.TrimSpace(v)                   // 目标字段名去空格
			normalized[strings.ToLower(mysqlField)] = elasticField // 统一将 MySQL 字段名小写化，查找稳定
		}

		r.FieldMapping = normalized // 使用规范化后的映射表
	}

	if len(r.Filter) > 0 { // 若配置了字段过滤白名单
		filter := make([]string, 0, len(r.Filter)) // 新切片：存放规范化后的字段名

		seen := make(map[string]struct{}, len(r.Filter)) // 用 map 去重，避免重复字段
		for _, f := range r.Filter {                     // 遍历过滤字段
			field := normalizeConfiguredColumnName(f) // 兼容 camelCase/snake_case 混用
			if field == "" {                          // 空字符串忽略
				continue
			}

			if _, ok := seen[field]; ok { // 已加入则跳过
				continue
			}

			filter = append(filter, field) // 加入白名单
			seen[field] = struct{}{}       // 标记已见
		}

		if len(filter) == 0 { // 若规范化后为空，视为不生效
			r.Filter = nil // 置为 nil，表示不过滤（同步全部字段）
		} else {
			r.Filter = filter // 否则用规范化后的列表
		}
	}

	if len(r.Index) == 0 { // 若未指定 ES 索引名
		r.Index = r.Table // 用表名作为索引名（随后会小写化）
	}

	if len(r.Type) == 0 { // 若未指定 ES 文档类型
		r.Type = r.Index // 用索引名作为类型名（随后会小写化）
	}

	// ES 对 type（以及 index）要求小写，这里统一小写化 // 原理：避免 ES 端因大小写不合法而报错
	r.Index = strings.ToLower(r.Index) // 索引小写
	r.Type = strings.ToLower(r.Type)   // 类型小写

	return nil // 预处理成功
}

// CheckFilter 判断某字段是否需要被同步（命中过滤白名单）。 // 原理：如果未设置过滤（r.Filter==nil），则全部字段同步.
func (r *Rule) CheckFilter(field string) bool {
	if r.Filter == nil { // 未设置过滤
		return true // 默认不过滤，返回 true 表示“需要同步此字段”
	}

	return slices.Contains(r.Filter, strings.ToLower(field)) // 未命中白名单：不需要同步
}

func normalizeConfiguredColumnName(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return field
	}

	var builder strings.Builder
	builder.Grow(len(field) + 4)

	var prev rune

	for i, r := range field {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
				builder.WriteByte('_')
			}

			builder.WriteRune(unicode.ToLower(r))
		} else {
			builder.WriteRune(unicode.ToLower(r))
		}

		prev = r
	}

	return builder.String()
}
