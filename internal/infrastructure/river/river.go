// Package river 负责基于 binlog 把 MySQL 的变更抽取、转换并写入 Elasticsearch.
//
// 原实现从 global.Config() / global.DB 直接取配置与数据库句柄；此处改为构造函数注入，
// 同步规则、位点文件路径、ES 文档结构与 canal 参数全部与原实现保持一致。
package river

import ( // 导入依赖
	"context" // 上下文控制：取消、超时等
	"crypto/tls"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"

	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/infrastructure/river/rule" // 引入规则模块

	"github.com/juju/errors"
	elastic7 "github.com/olivere/elastic/v7"
	"github.com/siddontang/go-log/log"
	"github.com/siddontang/go-mysql/canal"
	"gorm.io/gorm"
)

const (
	// defaultRiverServerID 未配置 serverID 时使用的 MySQL 复制 server_id.
	defaultRiverServerID = 1001
	// riverSyncChanSize 事件队列缓冲，避免高峰期阻塞 canal 回调.
	riverSyncChanSize = 4096
	// defaultRiverFlavor 未配置 flavor 时的 binlog 方言.
	defaultRiverFlavor = "mysql"
)

// ErrRuleNotExist 表示目标表未配置同步规则.
var ErrRuleNotExist = errors.New("rule is not exist")

// River 是 MySQL binlog 到 ES 的增量同步引擎.
type River struct {
	canal    *canal.Canal
	rules    map[string]*rule.Rule // key=schema:table 的规则集合，用于决定如何将表映射到 ES
	ctx      context.Context       // 控制定时/关闭的根上下文
	cancel   context.CancelFunc    // 用于取消 ctx 的函数（Close 时调用）
	wg       sync.WaitGroup        // 等待组，确保 goroutine 优雅退出
	es       *elastic7.Client      // ES 客户端：负责写入/更新/删除文档
	esHTTP   *http.Client          // River 独占的 HTTP 客户端，Close 时释放空闲连接
	master   *masterInfo           // binlog 位点的持久化信息（文件+位置），用于断点续传
	syncCh   chan any              // 事件/队列通道：内部同步协程消费（如 insert/update/delete）
	cfg      *config.River         // River 同步配置（源、规则、批量大小等）
	mysqlCfg *config.Mysql         // MySQL 连接配置（canal 复制用）
	db       *gorm.DB              // 业务库句柄：反查标签/关系等冗余字段
}

// NewRiver 根据注入的配置创建 River 实例。
//
// 原理：初始化 canal、规则、ES 客户端、位点信息等；任一步失败都返回错误，由调用方决定是否阻断启动。
func NewRiver(riverCfg *config.River, mysqlCfg *config.Mysql, esCfg *config.ES, db *gorm.DB) (*River, error) {
	r := new(River)

	if riverCfg == nil || !riverCfg.Enable {
		return nil, errors.New("river sync is disabled")
	}

	if mysqlCfg == nil {
		return nil, errors.New("mysql config is required for river")
	}

	if esCfg == nil {
		return nil, errors.New("es config is required for river")
	}

	r.cfg = riverCfg
	r.mysqlCfg = mysqlCfg
	r.db = db

	if err := validateRiverSchemas(riverCfg, mysqlCfg); err != nil {
		return nil, errors.Trace(err)
	}

	r.rules = make(map[string]*rule.Rule)                      // 初始化规则 map
	r.syncCh = make(chan any, riverSyncChanSize)               // 为异步处理准备一个有缓冲的通道，避免高峰期阻塞 canal 回调
	r.ctx, r.cancel = context.WithCancel(context.Background()) // 生成可取消上下文，Close 时触发取消

	var err error                                                     // 收集初始化各步骤可能的错误
	if r.master, err = loadMasterInfo(riverCfg.DataDir); err != nil { // 加载上次持久化的 binlog 位点
		return nil, errors.Trace(err) // 位点信息损坏或无法读取时抛错
	}

	if err = r.newCanal(); err != nil { // 创建/配置 canal 实例：连接 MySQL ，设置拉取策略
		return nil, errors.Trace(err)
	}

	if err = r.prepareRule(); err != nil { // 解析配置源、规则（包括通配表展开）、载入表结构、过滤无主键表
		return nil, errors.Trace(err)
	}

	if err = r.prepareCanal(); err != nil { // 基于规则计算 dump 范围（库/表）并注册事件处理器
		return nil, errors.Trace(err)
	}

	// 强制要求 binlog row image 为 FULL // 原理：更新/删除事件需要看到完整的行数据才能构造 ES 文档
	if err = r.canal.CheckBinlogRowImage("FULL"); err != nil { // canal 会检查 server 配置（binlog_row_image=FULL）
		return nil, errors.Trace(err)
	}

	r.es, r.esHTTP, err = newRiverESClient(esCfg)
	if err != nil {
		r.cancel()
		r.canal.Close()

		if closeErr := r.master.Close(); closeErr != nil {
			log.Errorf("close master info after ES init failure: %v", closeErr)
		}

		return nil, errors.Trace(err)
	}

	return r, nil // 创建完成
}

func newRiverESClient(cfg *config.ES) (*elastic7.Client, *http.Client, error) {
	scheme := "http"
	if cfg.IsHTTPS {
		scheme = "https"
	}

	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, nil, errors.New("default HTTP transport is not configurable")
	}

	transport := baseTransport.Clone()
	if cfg.IsHTTPS {
		transport.TLSClientConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // 与显式 ES 开发环境配置保持一致
		}
	}

	httpClient := &http.Client{Transport: transport}
	opts := []elastic7.ClientOptionFunc{
		elastic7.SetURL(fmt.Sprintf("%s://%s:%d", scheme, cfg.Host, cfg.Port)),
		elastic7.SetHttpClient(httpClient),
		elastic7.SetSniff(false),
		elastic7.SetHealthcheck(false),
	}
	if cfg.Username != "" || cfg.Password != "" {
		opts = append(opts, elastic7.SetBasicAuth(cfg.Username, cfg.Password))
	}

	client, err := elastic7.NewClient(opts...)
	if err != nil {
		httpClient.CloseIdleConnections()

		return nil, nil, fmt.Errorf("create ES client for river: %w", err)
	}

	return client, httpClient, nil
}

func (r *River) newCanal() error { // 构建 canal 配置并连接 MySQL
	cfg := canal.NewDefaultConfig()                   // 使用 canal 的默认配置作为基底
	db := r.mysqlCfg                                  // 读取 MySQL 连接配置
	cfg.Addr = fmt.Sprintf("%s:%d", db.Host, db.Port) // MySQL 地址
	cfg.User = db.Username                            // MySQL 用户
	cfg.Password = db.Password                        // MySQL 密码
	cfg.Charset = "utf8mb4"                           // 字符集，保证完整 emoji/多字节字符

	flavor := strings.TrimSpace(r.cfg.Flavor) // flavor：mysql / mariadb 等
	if flavor == "" {                         // 未配置时默认 mysql
		flavor = defaultRiverFlavor
	}

	cfg.Flavor = flavor // 设置 flavor 以适配不同实现的 binlog 差异

	serverID := r.cfg.ServerID // 设置 MySQL 复制的 server_id（必须唯一）
	if serverID == 0 {         // 未配置时给默认值，避免与线上主从冲突
		serverID = defaultRiverServerID

		log.Warnf("river server_id not configured, defaulting to %d", serverID) // 打日志提示
	}

	cfg.ServerID = serverID        // 生效
	cfg.Dump.DiscardErr = false    // 全量 dump 错误不丢弃（遇错要报出来）
	cfg.Dump.TableDB = db.DBName   // 限定 dump 的库（减少不必要数据）
	cfg.Dump.SkipMasterData = true // 跳过 master-data（不锁全库，配合位点自己管理）

	if dumpPath := strings.TrimSpace(r.cfg.MysqldumpPath); dumpPath != "" { // 可选：指定 mysqldump 可执行路径
		cfg.Dump.ExecutionPath = dumpPath // 一些环境没有放 PATH，需要手动指定
	}

	for _, s := range r.cfg.Sources { // 遍历配置中的源（库+表集合）
		for _, t := range s.Tables { // 每张表
			table := normalizeConfiguredTableName(t)                                    // 兼容 camelCase 配置，统一成数据库使用的 snake_case
			cfg.IncludeTableRegex = append(cfg.IncludeTableRegex, s.Schema+"\\."+table) // 构造 “库\.表正则”，支持通配，用于过滤拉取
		}
	}

	var err error // 连接 canal

	r.canal, err = canal.NewCanal(cfg) // 基于配置创建 canal 客户端（含全量dump+增量订阅能力）

	return errors.Trace(err) // 返回可能的错误
}

func (r *River) prepareCanal() error { // 将规则转化为 canal 的 dump 范围并注册事件回调
	var db string // 记录单库场景下的库名

	dbs := map[string]struct{}{} // 参与的库集合（用于判断是否单库）

	tables := make([]string, 0, len(r.rules)) // 单库场景下要 dump 的表名列表
	for _, rule := range r.rules {            // 遍历有效规则
		db = rule.Schema                    // 记录库名（若最终只有一个则可按表优化）
		dbs[rule.Schema] = struct{}{}       // 加入库集合
		tables = append(tables, rule.Table) // 收集表名
	}

	if len(dbs) == 1 { // 只有一个库
		// 单库可直接按表名缩小 dump 范围，效率更高
		r.canal.AddDumpTables(db, tables...) // 只 dump 指定表
	} else {
		// 多库时只能设定库级别（表集合可能跨库）
		keys := make([]string, 0, len(dbs)) // 把库名 map 转切片
		for key := range dbs {
			keys = append(keys, key)
		}

		r.canal.AddDumpDatabases(keys...) // 全量 dump 这些库
	}

	r.canal.SetEventHandler(&eventHandler{r}) // 注册事件处理器（实现 canal.EventHandler 接口），把行变更转发到 r.syncCh

	return nil // 完成
}

func (r *River) newRule(schema, table string) error { // 为某个 schema.table 新增一条默认规则
	key := ruleKey(schema, table) // 生成 key= "schema:table"

	if _, ok := r.rules[key]; ok { // 如果已存在规则
		return errors.Errorf("duplicate source %s, %s defined in config", schema, table) // 配置重复
	}

	r.rules[key] = rule.NewDefaultRule(schema, table) // 创建默认规则（index/type=表名小写）

	return nil // 成功
}

func (r *River) updateRule(schema, table string) error { // 把 canal 获取到的表结构信息填充进规则
	rule, ok := r.rules[ruleKey(schema, table)] // 取到该表的规则
	if !ok {                                    // 不存在则报预定义错
		return ErrRuleNotExist
	}

	tableInfo, err := r.canal.GetTable(schema, table) // 调 canal，从信息_schema 拉取表结构（列、类型、主键等）
	if err != nil {
		return errors.Trace(err)
	}

	rule.TableInfo = tableInfo // 赋值给规则，后续构造 ES 文档需要它（尤其是主键列）

	return nil // OK
}

func (r *River) parseSource() (map[string][]string, error) { // 解析 Sources：支持通配表名的展开
	wildTables := make(map[string][]string, len(r.cfg.Sources)) // 保存通配规则 -> 实际匹配到的表名列表

	// 第一步，遍历定义的 sources（数据源）
	for _, s := range r.cfg.Sources {
		if !isValidTables(s.Tables) { // 校验规则：如果配置了多张表，不能用 "*" 全量通配（避免危险范围过大）
			return nil, errors.Errorf("wildcard * is not allowed for multiple tables")
		}

		for _, table := range s.Tables { // 遍历每一个表（可能是通配）
			table = normalizeConfiguredTableName(table) // 兼容配置层写成 camelCase 的表名

			if len(s.Schema) == 0 { // schema 必填
				return nil, errors.Errorf("empty schema not allowed for source")
			}

			if regexp.QuoteMeta(table) != table { // 如果 table 含正则特殊字符 => 是通配/正则表
				if _, ok := wildTables[ruleKey(s.Schema, table)]; ok { // 同一个通配规则不可重复定义
					return nil, errors.Errorf("duplicate wildcard table defined for %s.%s", s.Schema, table)
				}

				tables := []string{} // 存放该通配展开出的实际表名

				// 发 SQL 到 information_schema，按 RLIKE 扩展出所有匹配的表名 // 原理：让 DB 端做匹配，权威且高效
				sql := fmt.Sprintf(`SELECT table_name FROM information_schema.tables WHERE
					table_name RLIKE "%s" AND table_schema = "%s";`, buildTable(table), s.Schema)

				res, err := r.canal.Execute(sql) // 通过 canal 的连接执行
				if err != nil {
					return nil, errors.Trace(err)
				}

				for i := range res.RowNumber() { // 逐行读取返回的表名
					f, _ := res.GetString(i, 0) // 第 0 列是 table_name

					err := r.newRule(s.Schema, f) // 为每个具体表注册一条默认规则
					if err != nil {
						return nil, errors.Trace(err)
					}

					tables = append(tables, f) // 收集展开结果
				}

				wildTables[ruleKey(s.Schema, table)] = tables // 记录“通配键 -> 具体表名集”
			} else { // 普通表名（非通配）
				err := r.newRule(s.Schema, table) // 直接注册规则
				if err != nil {
					return nil, errors.Trace(err)
				}
			}
		}
	}

	if len(r.rules) == 0 { // 如果最终没有任何规则
		return nil, errors.Errorf("no source data defined") // 说明 sources 配置为空
	}

	return wildTables, nil // 返回通配展开映射表，供后续规则绑定使用
}

func (r *River) prepareRule() error { // 结合 sources 与自定义 rules，做规则最终确定与表结构加载
	wildtables, err := r.parseSource() // 先解析 sources（含通配展开+默认规则注册）
	if err != nil {
		return errors.Trace(err)
	}

	if r.cfg.Rules != nil { // 如果配置里提供了“自定义规则”（改 index/type/字段映射/父子等）
		// 第二步：把自定义规则覆盖/补充到前面 newRule 得到的默认规则中
		for _, rule := range r.cfg.Rules { // 遍历每条规则
			if len(rule.Schema) == 0 { // schema 必填
				return errors.Errorf("empty schema not allowed for rule")
			}

			rule.Table = normalizeConfiguredTableName(rule.Table) // 自定义规则与 source 使用同一套表名规范化，避免 key 对不上

			if regexp.QuoteMeta(rule.Table) != rule.Table { // 若自定义规则用的是“通配表”
				// 通配表必须先在 sources 中定义，否则无法知道具体表有哪些
				tables, ok := wildtables[ruleKey(rule.Schema, rule.Table)]
				if !ok {
					return errors.Errorf("wildcard table for %s.%s is not defined in source", rule.Schema, rule.Table)
				}

				if len(rule.Index) == 0 { // 通配规则必须显式指定 index（因为每张表会共享这个 index）
					return errors.Errorf("wildcard table rule %s.%s must have a index, can not empty", rule.Schema, rule.Table)
				}

				if err := rule.Prepare(); err != nil { // 对该条规则做规范化（小写、去重、默认值等）
					return errors.Trace(err)
				}

				rule.Type = "" // 兼容无 type 的 ES（或由上层写入逻辑自行决定 type）

				for _, table := range tables { // 对通配展开出的每一张表，覆盖其默认规则的关键字段
					rr := r.rules[ruleKey(rule.Schema, table)] // 拿到已有默认规则
					rr.Index = rule.Index                      // 统一用指定索引
					rr.Type = ""                               // 清空 type
					rr.Parent = rule.Parent                    // 父子文档设置
					rr.ID = rule.ID                            // _id 组成字段
					rr.FieldMapping = rule.FieldMapping        // 字段映射
				}
			} else { // 自定义规则对应的是“单张具体表”
				key := ruleKey(rule.Schema, rule.Table) // 生成键
				if _, ok := r.rules[key]; !ok {         // 这张表必须先出现在 sources，否则规则无目标
					return errors.Errorf("rule %s, %s not defined in source", rule.Schema, rule.Table)
				}

				if err := rule.Prepare(); err != nil { // 规范化规则
					return errors.Trace(err)
				}

				rule.Type = ""      // 兼容无 type
				r.rules[key] = rule // 覆盖默认规则为自定义规则
			}
		}
	}

	rules := make(map[string]*rule.Rule) // 临时 map：仅保留“有主键”的表规则
	for key, rule := range r.rules {     // 遍历所有规则
		if rule.TableInfo, err = r.canal.GetTable(rule.Schema, rule.Table); err != nil { // 加载表结构信息
			return errors.Trace(err)
		}

		if len(rule.TableInfo.PKColumns) == 0 { // 没有主键的表
			log.Errorf("ignored table without a primary key: %s\n", rule.TableInfo.Name) // 忽略（无法稳定构造文档 _id）
		} else {
			rules[key] = rule // 有主键才保留，作为最终生效规则
		}
	}

	r.rules = rules // 替换回去，仅保留可安全同步的表

	return nil // 规则准备完成
}

func ruleKey(schema, table string) string { // 生成规则键，统一小写方便查找
	return strings.ToLower(fmt.Sprintf("%s:%s", schema, table)) // 形如 "db:table"
}

// validateRiverSchemas 校验并就地补全 river 源/规则的 schema：
// 留空时回落到 mysql.db，显式指定但与 mysql.db 不一致时直接报错。
func validateRiverSchemas(riverCfg *config.River, mysqlCfg *config.Mysql) error {
	if riverCfg == nil || mysqlCfg == nil {
		return nil
	}

	mysqlSchema := strings.TrimSpace(mysqlCfg.DBName)
	if mysqlSchema == "" {
		return errors.New("mysql db is required for river schema")
	}

	for i := range riverCfg.Sources {
		schema, err := normalizeRiverSchema(riverCfg.Sources[i].Schema, mysqlSchema, fmt.Sprintf("source[%d]", i))
		if err != nil {
			return err
		}

		riverCfg.Sources[i].Schema = schema
	}

	for i, rr := range riverCfg.Rules {
		if rr == nil {
			return errors.Errorf("river rules[%d] is nil", i)
		}

		schema, err := normalizeRiverSchema(rr.Schema, mysqlSchema, fmt.Sprintf("rules[%d]", i))
		if err != nil {
			return err
		}

		rr.Schema = schema
	}

	return nil
}

func normalizeRiverSchema(schema, mysqlSchema, path string) (string, error) {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return mysqlSchema, nil
	}

	if schema != mysqlSchema {
		return "", errors.Errorf("river %s schema %q does not match mysql.db %q; keep river source/rules in the same business database to avoid syncing stale search indexes", path, schema, mysqlSchema)
	}

	return schema, nil
}

// Run 启动同步：从位点开始跑 canal，并启动内部消费循环把事件写 ES.
func (r *River) Run() error {
	r.wg.Add(1)     // 记录将要启动一个 goroutine（syncLoop）
	go r.syncLoop() // 启动事件消费协程：从 r.syncCh 读取 insert/update/delete 并调用 ES 客户端

	pos := r.master.Position()                   // 读取上次持久化的 binlog 位点（文件名+偏移）
	if err := r.canal.RunFrom(pos); err != nil { // 从该位点启动 canal（会先做一次 dump，再进入增量订阅）
		log.Errorf("start canal err %v", err) // 启动失败日志

		return errors.Trace(err) // 返回错误
	}

	return nil // 启动成功
}

// Ctx 返回内部上下文，供外部感知退出/取消（例如 event handler 里需要）.
func (r *River) Ctx() context.Context {
	return r.ctx // 返回可取消上下文
}

// Close 关闭 River：停止订阅、保存位点、等待协程退出.
func (r *River) Close() {
	log.Infof("closing river") // 记录关闭日志

	r.cancel() // 触发上下文取消，通知 syncLoop 等协程退出

	r.canal.Close() // 关闭 canal，断开与 MySQL 的复制连接

	if err := r.master.Close(); err != nil { // 关闭位点管理（通常会 flush 持久化最后位置）
		log.Errorf("close master info failed: %v", err)
	}

	r.wg.Wait() // 等待所有 goroutine 结束，保证优雅退出

	if r.es != nil {
		r.es.Stop()
	}

	if r.esHTTP != nil {
		r.esHTTP.CloseIdleConnections()
	}
}

func isValidTables(tables []string) bool { // 校验“多表配置是否包含 * 通配”
	if len(tables) > 1 { // 当配置了多张表
		if slices.Contains(tables, "*") { // 其中包含 "*" 则判为不合法（范围过于危险）
			return false
		}
	}

	return true // 合法
}

func buildTable(table string) string { // 为通配构建 RLIKE 正则：把 "*" 变成 ".*"
	if table == "*" { // 如果就是全通配
		return "." + table // 返回 ".*"（RLIKE 语意下匹配任意表名）
	}

	return table // 否则返回原样（可能已经是更复杂的正则）
}

// normalizeConfiguredTableName 把 camelCase 配置的表名统一成数据库使用的 snake_case；
// 含正则特殊字符（通配表）时原样返回。
func normalizeConfiguredTableName(table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return table
	}

	if regexp.QuoteMeta(table) != table {
		return table
	}

	var builder strings.Builder
	builder.Grow(len(table) + 4)

	var prev rune

	for i, r := range table {
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
