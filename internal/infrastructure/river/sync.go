package river

import ( // 依赖导入：二进制序列化、反射、时间、ES/canal/mysql/schema 等
	"bytes"            // 构建字符串缓冲（生成文档ID）
	"encoding/json"    // 解析 JSON 列类型
	stderrors "errors" // errors.Is：识别被 Trace 包装的哨兵错误
	"fmt"              // 格式化输出
	"reflect"          // 反射（判断整型、比较变更）
	"strconv"          // 字符串转数字
	"strings"          // 字符串处理
	"time"             // 时间处理

	"f_ccg_video_back_ddd/internal/domain/shared/enum"        // 业务枚举
	"f_ccg_video_back_ddd/internal/infrastructure/river/rule" // 规则模块（字段映射、过滤、主键等）
	"f_ccg_video_back_ddd/pkg/constants"                      // 统一索引名常量等

	"github.com/juju/errors"                               // 错误包装：附带堆栈
	"github.com/siddontang/go-log/log"                     // 日志
	"github.com/siddontang/go-mysql-elasticsearch/elastic" // ES 客户端封装：BulkRequest/Client
	"github.com/siddontang/go-mysql/canal"                 // canal：订阅 MySQL binlog
	"github.com/siddontang/go-mysql/mysql"                 // MySQL 工具：位点、时间格式等
	"github.com/siddontang/go-mysql/replication"           // 复制层事件定义（Rotate/DDl 等）
	"github.com/siddontang/go-mysql/schema"                // 表结构描述（列类型、主键、取值）
)

const ( // 字段映射时的“目标类型”标识：在配置 field 映射中通过 ",date"/",list" 指定
	fieldTypeList = "list" // 字符串按逗号拆分成字符串数组，用于 ES 多值字段
	// 对 MySQL int（Unix 秒级时间戳）转 ES 日期
	// 配置写法示例： [rule.field] created_time = ",date" 表示“同名字段 + 按 date 转换”.
	fieldTypeDate = "date" // 将整型时间戳转为 RFC3339 日期字符串（ES 可解析）

	tableVideoTags = "video_tags"
)

const mysqlDateFormat = "2006-01-02" // 仅日期（不含时分秒）的解析/输出格式

const (
	// defaultBulkSize 未配置 bulkSize 时的批量写入阈值.
	defaultBulkSize = 128
	// syncFlushInterval 定时 flush 周期：保障低吞吐时也能定期落数据.
	syncFlushInterval = 200 * time.Millisecond
	// posSaveThrottle 非强制位点保存的最小间隔.
	posSaveThrottle = 3 * time.Second
	// syncReqsInitCap 批量请求聚合切片的初始容量.
	syncReqsInitCap = 1024
)

type posSaver struct { // 通过 channel 传递的“位点保存”指令
	pos   mysql.Position // 目标位点（binlog 文件名 + 偏移）
	force bool           // 是否强制：Rotate/DDL 等场景，立刻 flush 并保存位点
}

type eventHandler struct { // canal 的回调实现：把 binlog 事件翻译为 ES 批量请求or位点保存请求
	r *River // 持有 River，以便访问规则、ES 客户端、上下文、队列等
}

func (h *eventHandler) OnRotate(e *replication.RotateEvent) error { // binlog 切换文件事件
	pos := mysql.Position{
		Name: string(e.NextLogName), // 下一份 binlog 文件名
		Pos:  uint32(e.Position),    // 下一写入位置
	}

	h.r.syncCh <- posSaver{pos, true} // 强制保存：发送 posSaver(force=true)，确保在切换时立即刷位点

	return h.r.ctx.Err() // 返回 ctx 错误可触发上游停止（当 Close/cancel 时）
}

func (h *eventHandler) OnTableChanged(schema, table string) error { // 表结构/元数据变化
	err := h.r.updateRule(schema, table)                   // 重新拉取表结构（columns/PK），保证后续映射正确
	if err != nil && !stderrors.Is(err, ErrRuleNotExist) { // 若该表并未配置规则，忽略；其他错误则抛出
		return errors.Trace(err)
	}

	return nil // 正常
}

func (h *eventHandler) OnDDL(nextPos mysql.Position, _ *replication.QueryEvent) error { // 执行 DDL 事件
	h.r.syncCh <- posSaver{nextPos, true} // 强制保存位点：结构变更必须快速保证一致

	return h.r.ctx.Err() // 支持优雅退出
}

func (h *eventHandler) OnXID(nextPos mysql.Position) error { // 事务提交（commit）事件
	h.r.syncCh <- posSaver{nextPos, false} // 非强制保存：通常结合 3s 限流在 syncLoop 中落盘

	return h.r.ctx.Err() // 支持优雅退出
}

func (h *eventHandler) OnRow(e *canal.RowsEvent) error { // 核心：行变更事件（INSERT/UPDATE/DELETE）
	tableName := strings.ToLower(e.Table.Name) // 统一小写以便匹配

	switch tableName {
	case tableVideoTags: // 维护 videos 与 tags 的反范式：标签变更要“回写视频文档”
		ids := h.r.collectIDsFromRows(e.Rows, e.Table, "video_id") // 收集受影响的视频ID
		h.r.reindexVideosByIDs(ids)                                // 直接重建这些视频的 ES 文档（同步调用 doBulk）

		return nil // 不走默认规则（这张表只是桥表）
	case "tags":
		tagIDs := h.r.collectIDsFromRows(e.Rows, e.Table, "id") // 收集变更的 tag IDs
		videoIDs := h.r.videoIDsByTagIDs(tagIDs)                // 反查受影响的视频
		h.r.reindexVideosByIDs(videoIDs)                        // 重建视频文档

		return nil
	}

	rule, ok := h.r.rules[ruleKey(e.Table.Schema, e.Table.Name)] // 从规则表找当前表的映射规则
	if !ok {
		return nil // 未配置规则的表，忽略（避免无关表的写 ES）
	}

	var reqs []*elastic.BulkRequest // 将要推送给 syncLoop 的批量请求集合

	var err error
	if strings.EqualFold(rule.TableInfo.Name, "user_focus") { // 特例：粉丝关系表变更触发被关注用户 follower_count 的更新
		reqs, err = h.r.makeUserFocusFollowerRequests(rule, e.Rows) // 生成“受影响用户”的更新请求
	} else {
		switch e.Action { // 普通表按动作分流
		case canal.InsertAction:
			reqs, err = h.r.makeInsertRequest(rule, e.Rows) // 为每一行构建 index 请求
		case canal.DeleteAction:
			reqs, err = h.r.makeDeleteRequest(rule, e.Rows) // 为每一行构建 delete 请求
		case canal.UpdateAction:
			reqs, err = h.r.makeUpdateRequest(rule, e.Rows) // update 需要成对行（before/after）
		default:
			err = errors.Errorf("invalid rows action %s", e.Action) // 未知动作
		}
	}

	if err != nil {
		h.r.cancel() // 构建请求失败（一般是致命配置/结构问题），立即取消上下文，停止同步，保护一致性

		return errors.Errorf("make %s ES request err %v, close sync", e.Action, err)
	}

	h.r.syncCh <- reqs // 将本批次请求推送到主循环的批处理队列

	return h.r.ctx.Err() // 返回 ctx 错误支持优雅退出
}

func (h *eventHandler) OnGTID(_ mysql.GTIDSet) error { // GTID 事件（本实现未使用 GTID 位点）
	return nil // 留空：如果改用 GTID，可在此更新/保存 GTIDSet
}

func (h *eventHandler) OnPosSynced(_ mysql.Position, _ mysql.GTIDSet, _ bool) error { // canal 的位置同步回调
	return nil // 未用：本实现通过 posSaver + syncLoop 自行控制“flush→save pos”
}

func (h *eventHandler) String() string { // 标识 handler 名称（日志/调试）
	return "ESRiverEventHandler"
}

func (r *River) syncLoop() { // 核心批处理协程：聚合 BulkRequest，控制 flush 与位点落盘
	bulkSize := defaultBulkSize             // 默认批量大小
	if r.cfg != nil && r.cfg.BulkSize > 0 { // 可来自配置
		bulkSize = r.cfg.BulkSize
	}

	ticker := time.NewTicker(syncFlushInterval) // 周期触发器
	defer ticker.Stop()                         // 函数退出时停止
	defer r.wg.Done()                           // 协程结束，通知 WaitGroup

	lastSavedTime := time.Now()                              // 上次保存位点的时间（节流）
	reqs := make([]*elastic.BulkRequest, 0, syncReqsInitCap) // 聚合待写 ES 的请求数组

	var pos mysql.Position // 待保存的位点

	for { // 主循环：选择处理三种输入（channel、tick、ctx.Done）
		needFlush := false   // 本轮是否需要 flush ES
		needSavePos := false // 本轮 flush 后是否需要保存位点

		select {
		case v := <-r.syncCh: // 从事件队列拿数据：可能是 posSaver 或 []*BulkRequest
			switch v := v.(type) {
			case posSaver: // 位点保存请求
				now := time.Now()
				if v.force || now.Sub(lastSavedTime) > posSaveThrottle { // 强制或“距离上次保存>3s”
					lastSavedTime = now
					needFlush = true // 保存位点前必须先 flush，确保“位点晚于数据”
					needSavePos = true
					pos = v.pos // 记录要持久化的位点
				}
			case []*elastic.BulkRequest: // 来自 OnRow 的批量写请求
				reqs = append(reqs, v...)         // 累加
				needFlush = len(reqs) >= bulkSize // 达到批量阈值则触发 flush
			}
		case <-ticker.C: // 定时触发：避免长期攒批
			needFlush = true
		case <-r.ctx.Done(): // 收到取消信号：退出
			return
		}

		if needFlush {
			if err := r.doBulk(reqs); err != nil { // 执行 ES 批量写
				log.Errorf("do ES bulk err %v, close sync", err)
				r.cancel() // 一旦 Bulk 失败，立刻停止（防止位点继续前进造成丢数）

				return
			}

			reqs = reqs[0:0] // 清空聚合
		}

		if needSavePos { // 只有在本轮 flush 成功后才会进入保存位点
			if err := r.master.Save(pos); err != nil { // 持久化位点（文件/DB）
				log.Errorf("save sync position %s err %v, close sync", pos, err)
				r.cancel() // 位点无法保存 → 停止，避免“认定已写成功但位点没记住”的不一致

				return
			}
		}
	}
}

// makeRequest 为 INSERT/DELETE 构造批量请求（共用构造器）。
func (r *River) makeRequest(rule *rule.Rule, action string, rows [][]any) ([]*elastic.BulkRequest, error) {
	reqs := make([]*elastic.BulkRequest, 0, len(rows)) // 为每一行准备一条 BulkRequest

	for _, values := range rows { // 遍历每一行变更
		id, err := r.getDocID(rule, values) // 生成 ES 文档 _id（主键组合作为稳定幂等键）
		if err != nil {
			return nil, errors.Trace(err)
		}

		parentID := ""            // 处理父子文档场景
		if len(rule.Parent) > 0 { // 若设置了 Parent 字段名
			if parentID, err = r.getParentID(rule, values, rule.Parent); err != nil { // 从行中取父ID
				return nil, errors.Trace(err)
			}
		}

		req := &elastic.BulkRequest{Index: rule.Index, Type: rule.Type, ID: id, Parent: parentID, Pipeline: rule.Pipeline} // 预填公共字段

		if action == canal.DeleteAction { // 删除场景
			req.Action = elastic.ActionDelete // 生成 delete 操作
		} else { // 插入场景
			r.makeInsertReqData(req, rule, values) // 构建 index 数据体（含字段映射/过滤/类型转换/富化）
		}

		reqs = append(reqs, req) // 收集
	}

	return reqs, nil // 返回本批构造
}

func (r *River) makeInsertRequest(rule *rule.Rule, rows [][]any) ([]*elastic.BulkRequest, error) { // INSERT 专用封装
	return r.makeRequest(rule, canal.InsertAction, rows)
}

func (r *River) makeDeleteRequest(rule *rule.Rule, rows [][]any) ([]*elastic.BulkRequest, error) { // DELETE 专用封装
	return r.makeRequest(rule, canal.DeleteAction, rows)
}

func (r *River) makeUpdateRequest(rule *rule.Rule, rows [][]any) ([]*elastic.BulkRequest, error) { // UPDATE 构造：行对（before/after）
	if len(rows)%2 != 0 { // canal 对 UPDATE 提供成对行：偶数条
		return nil, errors.Errorf("invalid update rows event, must have 2x rows, but %d", len(rows))
	}

	reqs := make([]*elastic.BulkRequest, 0, len(rows)) // 容量上界

	for i := 0; i < len(rows); i += 2 { // 每次处理一对
		beforeID, err := r.getDocID(rule, rows[i]) // 变更前的文档ID
		if err != nil {
			return nil, errors.Trace(err)
		}

		afterID, err := r.getDocID(rule, rows[i+1]) // 变更后的文档ID（主键变更时会不同）
		if err != nil {
			return nil, errors.Trace(err)
		}

		beforeParentID, afterParentID := "", "" // 父子文档ID可能也会随变更不同
		if len(rule.Parent) > 0 {
			if beforeParentID, err = r.getParentID(rule, rows[i], rule.Parent); err != nil {
				return nil, errors.Trace(err)
			}

			if afterParentID, err = r.getParentID(rule, rows[i+1], rule.Parent); err != nil {
				return nil, errors.Trace(err)
			}
		}

		req := &elastic.BulkRequest{Index: rule.Index, Type: rule.Type, ID: beforeID, Parent: beforeParentID} // 先以 before 作为目标

		if beforeID != afterID || beforeParentID != afterParentID { // 主键或父子关系变动
			req.Action = elastic.ActionDelete // 先删旧文档，防止悬挂
			reqs = append(reqs, req)

			req = &elastic.BulkRequest{Index: rule.Index, Type: rule.Type, ID: afterID, Parent: afterParentID, Pipeline: rule.Pipeline} // 再插新文档
			r.makeInsertReqData(req, rule, rows[i+1])                                                                                   // 重建完整文档（可走 pipeline）
		} else { // ID 与 Parent 未变：做部分更新或“强制 index”
			if len(rule.Pipeline) > 0 { // ES pipeline 只能绑定在 index 操作
				// Pipelines can only be specified on index action
				r.makeInsertReqData(req, rule, rows[i+1]) // 构造完整数据（等价于 upsert）
				// Make sure action is index, not create
				req.Action = elastic.ActionIndex // 设置为 index（不是 create）
				req.Pipeline = rule.Pipeline     // 挂上 pipeline 做预处理
			} else {
				r.makeUpdateReqData(req, rule, rows[i], rows[i+1]) // 对比前后值，仅更新变化字段，节省写放大
			}
		}

		reqs = append(reqs, req) // 收集本次的 update（或 index 新文档）
	}

	return reqs, nil // 返回所有构造请求
}

func (r *River) makeReqColumnData(col *schema.TableColumn, value any) any { // 将 MySQL 列值转换为 ES 友好值（按列类型）
	switch col.Type {
	case schema.TYPE_ENUM: // ENUM 存 binlog 时可能是 int64，下标从 1 开始
		switch value := value.(type) {
		case int64:
			// for binlog, ENUM may be int64, but for dump, enum is string
			eNum := value - 1 // 下标转为 0 开始
			if eNum < 0 || eNum >= int64(len(col.EnumValues)) {
				// 如果历史插入过非法值，则返回空字符串以避免映射异常
				log.Warnf("invalid binlog enum index %d, for enum %v", eNum, col.EnumValues)

				return ""
			}

			return col.EnumValues[eNum] // 取枚举对应字符串
		}
	case schema.TYPE_SET: // SET 以 bitset 编码
		switch value := value.(type) {
		case int64:
			// for binlog, SET may be int64, but for dump, SET is string
			bitmask := value

			sets := make([]string, 0, len(col.SetValues))
			for i, s := range col.SetValues {
				if bitmask&int64(1<<uint(i)) > 0 { // 位上为1表示该取值被选择
					sets = append(sets, s)
				}
			}

			return strings.Join(sets, ",") // 返回逗号分隔字符串
		}
	case schema.TYPE_BIT: // BIT 在 dump 时是字符串 "\x01"/"\x00"
		switch value := value.(type) {
		case string:
			// for binlog, BIT is int64, but for dump, BIT is string
			// for dump 0x01 is for 1, \0 is for 0
			if value == "\x01" {
				return int64(1)
			}

			return int64(0)
		}
	case schema.TYPE_STRING: // BLOB/TEXT 可能是 []byte，需要转 string
		switch value := value.(type) {
		case []byte:
			return string(value[:])
		}
	case schema.TYPE_JSON: // JSON 存储时可能是 string 或 []byte，需要反序列化为 interface{}
		var (
			f   any
			err error
		)

		switch v := value.(type) {
		case string:
			err = json.Unmarshal([]byte(v), &f)
		case []byte:
			err = json.Unmarshal(v, &f)
		}

		if err == nil && f != nil {
			return f // 返回结构化对象，ES 能按 nested/object 解析
		}
	case schema.TYPE_DATETIME, schema.TYPE_TIMESTAMP: // 日期时间，字符串解析为 RFC3339 输出
		switch v := value.(type) {
		case string:
			vt, err := time.ParseInLocation(mysql.TimeFormat, v, time.Local) // 按本地时区解析
			if err != nil || vt.IsZero() {                                   // 解析失败或零日期
				return nil
			}

			return vt.Format(time.RFC3339) // 输出标准 RFC3339，ES 内建解析
		}
	case schema.TYPE_DATE: // 纯日期
		switch v := value.(type) {
		case string:
			vt, err := time.Parse(mysqlDateFormat, v)
			if err != nil || vt.IsZero() { // 解析失败或零日期
				return nil
			}

			return vt.Format(mysqlDateFormat) // 按 YYYY-MM-DD 输出
		}
	}

	return value // 默认原样返回（数字/布尔等）
}

func (r *River) getFieldParts(k, v string) (string, string, string) { // 解析字段映射项：k=MySQL列名，v="esName[,type]"
	composedField := strings.Split(v, ",")         // 允许 v 里携带 "目标名,类型"（类型可为 list/date）
	mysql := strings.ToLower(strings.TrimSpace(k)) // 源字段名：统一小写去空格
	elastic := ""                                  // 目标字段名（为空则退回同名）
	fieldType := ""                                // 可选的转换类型（list/date）

	if len(composedField) > 0 {
		elastic = strings.TrimSpace(composedField[0]) // 取目标名
	}

	if len(elastic) == 0 {
		elastic = mysql // 未指定目标名 → 保持同名
	}

	if len(composedField) > 1 {
		fieldType = strings.ToLower(strings.TrimSpace(composedField[1])) // 解析类型后缀
	}

	return mysql, elastic, fieldType // 返回三元组
}

func (r *River) makeInsertReqData(req *elastic.BulkRequest, rule *rule.Rule, values []any) { // 构造 index 操作的数据体
	req.Data = make(map[string]any, len(values)) // 初始化文档数据
	req.Action = elastic.ActionIndex             // 插入/覆盖（幂等）

	for i, c := range rule.TableInfo.Columns { // 遍历列定义（有序，能对应行值索引）
		columnName := strings.ToLower(c.Name) // 源列名小写
		if !rule.CheckFilter(columnName) {    // 若配置了过滤白名单且未命中，则不输出该字段
			continue
		}

		mapped := false // 标记是否被 FieldMapping 命中

		for k, v := range rule.FieldMapping { // 遍历映射配置
			mysql, elastic, fieldType := r.getFieldParts(k, v) // 解析映射条目
			if mysql == columnName {                           // 命中该列
				mapped = true
				req.Data[elastic] = r.getFieldValue(&c, fieldType, values[i]) // 按类型转换后写入目标字段名

				break
			}
		}

		if !mapped { // 未在映射中指定 → 默认“同名 + 类型感知转换”
			req.Data[c.Name] = r.makeReqColumnData(&c, values[i])
		}
	}

	r.enrichVideoDocument(req, rule, values) // 针对 videos：补充 tags、suggester、计数字段
	r.enrichUserDocument(req, rule, values)  // 针对 users：补充 follower_count、username_suggest
	r.enrichTagDocument(req, rule, values)   // 针对 tags：补充 name_suggest
}

func (r *River) makeUpdateReqData(req *elastic.BulkRequest, rule *rule.Rule,
	beforeValues, afterValues []any,
) { // UPDATE 的部分更新：仅包含“有变化”的字段
	req.Data = make(map[string]any, len(beforeValues)) // 初始化更新字段集合

	// maybe dangerous if something wrong delete before?
	req.Action = elastic.ActionUpdate // 使用 ES 的“局部更新”语义

	for i, c := range rule.TableInfo.Columns { // 遍历列
		columnName := strings.ToLower(c.Name)
		mapped := false

		if !rule.CheckFilter(columnName) { // 过滤不需要同步的列
			continue
		}

		if reflect.DeepEqual(beforeValues[i], afterValues[i]) { // 值未变则跳过（节省写放大）
			// nothing changed
			continue
		}

		for k, v := range rule.FieldMapping { // 优先看是否有重命名/类型映射
			mysql, elastic, fieldType := r.getFieldParts(k, v)
			if mysql == columnName {
				mapped = true
				req.Data[elastic] = r.getFieldValue(&c, fieldType, afterValues[i]) // 用“新值”

				break
			}
		}

		if !mapped { // 否则默认“同名 + 类型感知转换”
			req.Data[c.Name] = r.makeReqColumnData(&c, afterValues[i])
		}
	}

	r.enrichVideoDocument(req, rule, afterValues) // UPDATE 同样需要按 after 补充冗余字段
	r.enrichUserDocument(req, rule, afterValues)
	r.enrichTagDocument(req, rule, afterValues)
}

// getDocID 生成 ES 文档 _id（幂等关键）。
//
// 未在规则里指定 ID 字段时取表主键，否则按 rule.ID 列表拼接；统一加「表名:」前缀避免跨表碰撞。
func (r *River) getDocID(rule *rule.Rule, row []any) (string, error) {
	var (
		ids []any // 主键或自定义ID字段值的集合
		err error
	)

	if rule.ID == nil { // 未显式指定 ID 字段 → 使用表主键列
		ids, err = rule.TableInfo.GetPKValues(row) // canal 提供 row，可依表结构取 PK 值
		if err != nil {
			return "", err
		}
	} else { // 指定了 ID 字段列表（可拼接复合ID）
		ids = make([]any, 0, len(rule.ID))
		for _, column := range rule.ID {
			value, err := rule.TableInfo.GetColumnValue(column, row) // 按列名取值
			if err != nil {
				return "", err
			}

			ids = append(ids, value)
		}
	}

	var buf bytes.Buffer // 拼接 id 字符串

	sep := ""

	for i, value := range ids {
		if value == nil { // 任何 ID 片段为 nil 都不合法（幂等键必须非空）
			return "", errors.Errorf("The %ds id or PK value is nil", i)
		}

		fmt.Fprintf(&buf, "%s%v", sep, value) // 用 ":" 拼接多段 ID
		sep = ":"
	}

	id := buf.String() // 基本 ID（不含前缀）

	prefix := strings.ToLower(rule.Table) // 默认前缀用表名
	if prefix == "" {
		prefix = strings.ToLower(rule.Index) // 兜底用索引名
	}

	if prefix != "" { // 加上前缀，避免跨表/索引碰撞
		id = fmt.Sprintf("%s:%s", prefix, id)
	}

	return id, nil // 返回完整 _id
}

func (r *River) getParentID(rule *rule.Rule, row []any, columnName string) (string, error) { // 取父文档ID
	index := rule.TableInfo.FindColumn(columnName) // 找到父ID列下标
	if index < 0 {
		return "", errors.Errorf("parent id not found %s(%s)", rule.TableInfo.Name, columnName)
	}

	return fmt.Sprint(row[index]), nil // 转字符串（与 _id 拼接逻辑保持统一）
}

func (r *River) enrichVideoDocument(req *elastic.BulkRequest, rule *rule.Rule, row []any) { // 针对 videos 表的冗余补充
	if !strings.EqualFold(rule.TableInfo.Name, "videos") {
		return
	}

	videoID, ok := getPrimaryKeyUint(rule, row) // 提取主键（uint）
	if !ok {
		return
	}

	status := r.getColumnData(rule, row, "status")
	title := r.getStringColumn(rule, row, "title") // 取标题

	tags := r.loadVideoTagsByID(videoID) // 通过桥表加载视频标签（保持冗余）
	if len(tags) > 0 {
		req.Data["tags"] = tags
	}

	if isPublishedStatusValue(status) {
		if sugg := buildCompletionPayload([]string{title}); sugg != nil { // 构建 completion suggester
			req.Data["title_suggest"] = sugg
		}

		if tagSugg := buildCompletionPayload(tags); tagSugg != nil {
			req.Data["tag_suggest"] = tagSugg // tag 的补全
		}
	} else {
		req.Data["title_suggest"] = nil
		req.Data["tag_suggest"] = nil
	}

	req.Data["author_username"] = r.getStringColumn(rule, row, "author_username")
	req.Data["created_at"] = r.getColumnData(rule, row, "created_at")
	req.Data["status"] = status
	req.Data["danmu_count"] = r.getNumericColumn(rule, row, "danmu_count") // 直接冗余写入，便于检索/排序
	req.Data["duration"] = r.getNumericColumn(rule, row, "duration")
}

func (r *River) enrichUserDocument(req *elastic.BulkRequest, rule *rule.Rule, row []any) { // 针对 users 表的冗余补充
	if !strings.EqualFold(rule.TableInfo.Name, "users") {
		return
	}

	userID, ok := getPrimaryKeyUint(rule, row)
	if !ok {
		return
	}

	req.Data["follower_count"] = r.loadUserFollowerCount(userID) // 反查粉丝数（聚合写入，避免查询时 join）

	username := r.getStringColumn(rule, row, "username")
	if sugg := buildCompletionPayload([]string{username}); sugg != nil {
		req.Data["username_suggest"] = sugg // 用户名补全字段
	}
}

func (r *River) enrichTagDocument(req *elastic.BulkRequest, rule *rule.Rule, row []any) { // 针对 tags 表的冗余补充
	if !strings.EqualFold(rule.TableInfo.Name, "tags") {
		return
	}

	name := r.getStringColumn(rule, row, "name")
	if sugg := buildCompletionPayload([]string{name}); sugg != nil {
		req.Data["name_suggest"] = sugg // 标签名补全
	}
}

func getPrimaryKeyUint(rule *rule.Rule, row []any) (uint64, bool) { // 取主键并转为无符号整数
	values, err := rule.TableInfo.GetPKValues(row)
	if err != nil || len(values) == 0 {
		return 0, false
	}

	return convertInterfaceToUint(values[0]) // 多列主键只用第一列（当前使用场景为单列PK）
}

func (r *River) getStringColumn(rule *rule.Rule, row []any, column string) string { // 以列名取字符串
	idx := rule.TableInfo.FindColumn(column)
	if idx < 0 || idx >= len(row) {
		return ""
	}

	if row[idx] == nil {
		return ""
	}

	return fmt.Sprint(row[idx]) // 通用转字符串
}

func buildCompletionPayload(values []string) map[string]any { // ES completion 字段的输入格式 {"input": [...]}
	inputs := make([]string, 0, len(values))
	for _, val := range values {
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			continue
		}

		inputs = append(inputs, trimmed)
	}

	if len(inputs) == 0 {
		return nil
	}

	return map[string]any{"input": inputs} // 交由 ES mapping 定义为 "completion" 类型
}

func (r *River) getNumericColumn(rule *rule.Rule, row []any, column string) any { // 直接返回数值（或0兜底）
	idx := rule.TableInfo.FindColumn(column)
	if idx < 0 || idx >= len(row) {
		return 0
	}

	return row[idx]
}

func (r *River) getColumnData(rule *rule.Rule, row []any, column string) any {
	idx := rule.TableInfo.FindColumn(column)
	if idx < 0 || idx >= len(row) {
		return nil
	}

	if row[idx] == nil {
		return nil
	}

	return r.makeReqColumnData(&rule.TableInfo.Columns[idx], row[idx])
}

// isPublishedStatusValue 判断 binlog/dump 取到的 status 值是否为「已发布」。
//
// binlog 与 mysqldump 对同一列可能给出不同 Go 类型，因此逐类型兜底。
func isPublishedStatusValue(value any) bool {
	switch v := value.(type) {
	case enum.VideoStatus:
		return v == enum.VideoStatusPublished
	case int:
		return v == int(enum.VideoStatusPublished)
	case int8:
		return int(v) == int(enum.VideoStatusPublished)
	case int16:
		return int(v) == int(enum.VideoStatusPublished)
	case int32:
		return int(v) == int(enum.VideoStatusPublished)
	case int64:
		return int(v) == int(enum.VideoStatusPublished)
	case uint:
		return int(v) == int(enum.VideoStatusPublished)
	case uint8:
		return int(v) == int(enum.VideoStatusPublished)
	case uint16:
		return int(v) == int(enum.VideoStatusPublished)
	case uint32:
		return int(v) == int(enum.VideoStatusPublished)
	case uint64:
		return int(v) == int(enum.VideoStatusPublished)
	case string:
		status, err := strconv.Atoi(strings.TrimSpace(v))

		return err == nil && status == int(enum.VideoStatusPublished)
	default:
		return false
	}
}

func (r *River) reindexVideosByIDs(ids map[uint64]struct{}) { // 批量重建视频文档：用于 tags 或 video_tags 变化后的“派生修复”
	if len(ids) == 0 {
		return
	}

	reqs := make([]*elastic.BulkRequest, 0, len(ids))
	for id := range ids {
		if req := r.buildVideoBulkRequest(id); req != nil { // 针对每个视频，拉全量字段并构建 index
			reqs = append(reqs, req)
		}
	}

	if len(reqs) == 0 {
		return
	}

	if err := r.doBulk(reqs); err != nil { // 注意：这里直接调用 doBulk（不走 syncLoop），会阻塞 canal 回调
		log.Errorf("reindex videos failed: %v", err) // 如果失败，仅日志，不会影响位点保存逻辑
	}
}

func (r *River) buildVideoBulkRequest(videoID uint64) *elastic.BulkRequest { // 从 DB 读出视频行并构建完整文档
	var video struct {
		ID             uint64
		Title          string
		Cover          string
		AuthorUsername string
		CreatedAt      time.Time
		Status         int
		Views          int64
		DanmuCount     int64
		Duration       int64
	}
	if err := r.db.WithContext(r.ctx). // 通过 GORM 直连业务库按主键读取
						Table("videos").
						Select("id, title, cover, author_username, created_at, status, views, danmu_count, duration").
						Where("id = ?", videoID).
						First(&video).Error; err != nil {
		log.Errorf("reload video failed (id=%d): %v", videoID, err)

		return nil
	}

	tags := r.loadVideoTagsByID(videoID) // 读标签列表
	data := map[string]any{
		"id":              video.ID,
		"title":           video.Title,
		"cover":           video.Cover,
		"author_username": video.AuthorUsername,
		"created_at":      video.CreatedAt,
		"status":          video.Status,
		"views":           video.Views,
		"danmu_count":     video.DanmuCount,
		"duration":        video.Duration,
	}
	published := isPublishedStatusValue(video.Status)

	if len(tags) > 0 {
		data["tags"] = tags
	}

	if published {
		if tagSugg := buildCompletionPayload(tags); tagSugg != nil {
			data["tag_suggest"] = tagSugg
		}

		if sugg := buildCompletionPayload([]string{video.Title}); sugg != nil {
			data["title_suggest"] = sugg
		}
	}

	return &elastic.BulkRequest{ // 构造“覆盖式 index”，幂等地重建文档
		Index:  constants.VideoSearchIndex,        // 统一的视频索引名
		Type:   "",                                // 空 type 以适配新 ES
		ID:     fmt.Sprintf("videos:%d", videoID), // 与普通逻辑一致的 _id 前缀
		Action: elastic.ActionIndex,
		Data:   data,
	}
}

func (r *River) collectIDsFromRows(rows [][]any, table *schema.Table, column string) map[uint64]struct{} { // 从行集合按列名提取 uint ID 集
	idx := table.FindColumn(column)
	if idx < 0 {
		return nil
	}

	ids := make(map[uint64]struct{})

	for _, row := range rows {
		if id, ok := convertInterfaceToUint(row[idx]); ok { // 兼容多种 binlog 值类型
			ids[id] = struct{}{}
		}
	}

	return ids
}

func (r *River) videoIDsByTagIDs(tagIDs map[uint64]struct{}) map[uint64]struct{} { // 由 tagID 反查视频ID 集合
	if len(tagIDs) == 0 {
		return nil
	}

	ids := make([]uint64, 0, len(tagIDs))
	for id := range tagIDs {
		ids = append(ids, id)
	}

	var videoIDs []uint64
	if err := r.db.WithContext(r.ctx). // 查询桥表获得所有视频ID
						Table(tableVideoTags).
						Select("video_id").
						Where("tag_id IN ?", ids).
						Scan(&videoIDs).Error; err != nil {
		log.Errorf("load video_ids by tag failed: %v", err)

		return nil
	}

	result := make(map[uint64]struct{}, len(videoIDs))
	for _, vid := range videoIDs {
		result[vid] = struct{}{}
	}

	return result
}

func (r *River) loadVideoTagsByID(videoID uint64) []string { // 按视频ID加载标签名（排序后返回）
	var names []string
	if err := r.db.WithContext(r.ctx).
		Table(tableVideoTags+" AS vt").
		Select("t.name").
		Joins("JOIN tags t ON t.id = vt.tag_id").
		Where("vt.video_id = ?", videoID).
		Order("t.name ASC").
		Scan(&names).Error; err != nil {
		log.Errorf("load video tags failed (video_id=%d): %v", videoID, err)

		return nil
	}

	return names
}

// convertInterfaceToUint 将各种可能类型（string/[]byte/intX/uintX）转为 uint64。
func convertInterfaceToUint(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint64:
		return v, true
	case uint32:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}

		return uint64(v), true
	case int32:
		if v < 0 {
			return 0, false
		}

		return uint64(v), true
	case int:
		if v < 0 {
			return 0, false
		}

		return uint64(v), true
	case string:
		if v == "" {
			return 0, false
		}

		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, false
		}

		return id, true
	case []byte:
		if len(v) == 0 {
			return 0, false
		}

		id, err := strconv.ParseUint(string(v), 10, 64)
		if err != nil {
			return 0, false
		}

		return id, true
	default:
		return 0, false
	}
}

func (r *River) loadUserFollowerCount(userID uint64) int64 { // 统计被关注用户的粉丝数（写入 users 文档）
	var count int64
	if err := r.db.WithContext(r.ctx).
		Table("user_focus").
		Where("focus_user_id = ?", userID).
		Count(&count).Error; err != nil {
		log.Errorf("load follower count failed (user_id=%d): %v", userID, err)

		return 0
	}

	return count
}

// makeUserFocusFollowerRequests 处理 user_focus 变更 → 批量重建被影响用户的完整文档。
func (r *River) makeUserFocusFollowerRequests(rule *rule.Rule, rows [][]any) ([]*elastic.BulkRequest, error) {
	userIdx := rule.TableInfo.FindColumn("user_id")

	focusIdx := rule.TableInfo.FindColumn("focus_user_id")
	if focusIdx < 0 {
		return nil, errors.Errorf("focus_user_id column not found in %s", rule.TableInfo.Name)
	}

	affected := make(map[uint64]struct{})

	for _, row := range rows {
		if focusIdx >= len(row) {
			continue
		}

		if id, ok := convertInterfaceToUint(row[focusIdx]); ok {
			affected[id] = struct{}{} // 被关注者
		}

		if userIdx >= 0 && userIdx < len(row) {
			if id, ok := convertInterfaceToUint(row[userIdx]); ok {
				affected[id] = struct{}{} // 关注者：可能进入互关列表
			}
		}
	}

	if len(affected) == 0 {
		return nil, nil
	}

	reqs := make([]*elastic.BulkRequest, 0, len(affected))
	for userID := range affected {
		req := r.buildUserBulkRequest(userID)
		if req == nil {
			continue
		}

		reqs = append(reqs, req)
	}

	return reqs, nil
}

// buildUserBulkRequest 从 DB 读出用户行并构建完整文档，避免 update 在文档缺失时丢失关系字段。
func (r *River) buildUserBulkRequest(userID uint64) *elastic.BulkRequest {
	var user struct {
		ID          uint64
		Username    string
		Description string
		Avatar      string
		Level       int
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}
	if err := r.db.WithContext(r.ctx).
		Table("users").
		Select("id, username, description, avatar, level, created_at, updated_at").
		Where("id = ?", userID).
		First(&user).Error; err != nil {
		log.Errorf("reload user failed (id=%d): %v", userID, err)

		return nil
	}

	count, followNames, friendNames := r.loadUserRelationStats(userID)

	data := map[string]any{
		"id":               user.ID,
		"username":         user.Username,
		"description":      user.Description,
		"avatar":           user.Avatar,
		"level":            user.Level,
		"created_at":       user.CreatedAt,
		"updated_at":       user.UpdatedAt,
		"follower_count":   count,
		"follow_usernames": followNames,
		"friend_usernames": friendNames,
	}
	if sugg := buildCompletionPayload([]string{user.Username}); sugg != nil {
		data["username_suggest"] = sugg
	}

	return &elastic.BulkRequest{
		Index:  constants.UserSearchIndex,
		Type:   "",
		ID:     fmt.Sprintf("users:%d", userID),
		Action: elastic.ActionIndex,
		Data:   data,
	}
}

// loadUserRelationStats 统计粉丝数、单向关注用户名、互关好友用户名。
func (r *River) loadUserRelationStats(userID uint64) (int64, []string, []string) {
	var followerCount int64
	if err := r.db.WithContext(r.ctx).
		Table("user_focus").
		Where("focus_user_id = ?", userID).
		Count(&followerCount).Error; err != nil {
		log.Errorf("load follower count failed (user_id=%d): %v", userID, err)
	}

	// 互相关注（好友）：当前用户关注别人，且对方也关注当前用户
	friendNames := make([]string, 0)
	if err := r.db.WithContext(r.ctx).Raw(`
		SELECT u.username
		FROM user_focus f
		JOIN user_focus r ON r.user_id = f.focus_user_id AND r.focus_user_id = f.user_id
		JOIN users u ON u.id = f.focus_user_id
		WHERE f.user_id = ?
		ORDER BY u.username ASC`, userID).Pluck("username", &friendNames).Error; err != nil {
		log.Errorf("load friend usernames failed (user_id=%d): %v", userID, err)

		friendNames = nil
	}

	// 单向关注（未互关）：当前用户关注的列表里排除已互关的
	followingNames := make([]string, 0)
	if err := r.db.WithContext(r.ctx).Raw(`
		SELECT u.username
		FROM user_focus f
		JOIN users u ON u.id = f.focus_user_id
		WHERE f.user_id = ?
		  AND NOT EXISTS (
		    SELECT 1 FROM user_focus r WHERE r.user_id = f.focus_user_id AND r.focus_user_id = f.user_id
		  )
		ORDER BY u.username ASC`, userID).Pluck("username", &followingNames).Error; err != nil {
		log.Errorf("load following usernames failed (user_id=%d): %v", userID, err)

		followingNames = nil
	}

	return followerCount, followingNames, friendNames
}

// doBulk 执行 ES 批量请求。
//
// 整体失败返回错误（上层会 cancel 停止同步）；部分条目失败只逐条打日志，与原实现一致。
func (r *River) doBulk(reqs []*elastic.BulkRequest) error {
	if len(reqs) == 0 {
		return nil
	}

	if resp, err := r.es.Bulk(reqs); err != nil { // 网络/集群不可达等“总体失败”
		log.Errorf("sync docs err %v after binlog %s", err, r.canal.SyncedPosition())

		return errors.Trace(err) // 返回错误，上层会 cancel 停止同步
	} else if resp.Code/100 == 2 || resp.Errors { // 2xx 或部分错误
		for i := range len(resp.Items) {
			for action, item := range resp.Items[i] {
				if len(item.Error) > 0 { // 逐条日志输出失败项（例如 mapping 冲突、版本冲突等）
					log.Errorf("%s index: %s, type: %s, id: %s, status: %d, error: %s",
						action, item.Index, item.Type, item.ID, item.Status, item.Error)
				}
			}
		}
	}

	return nil
}

// getFieldValue 施加映射配置里声明的类型转换（",list" / ",date"）。
func (r *River) getFieldValue(col *schema.TableColumn, fieldType string, value any) any {
	var fieldValue any

	switch fieldType {
	case fieldTypeList: // ",list"：将字符串按逗号拆分为数组
		v := r.makeReqColumnData(col, value) // 先做列类型感知转换
		if str, ok := v.(string); ok {
			fieldValue = strings.Split(str, ",") // 逗号切分
		} else {
			fieldValue = v // 非字符串则不切分，原样返回
		}

	case fieldTypeDate: // ",date"：整型秒级时间戳 → RFC3339 字符串（ES 可按 date 解析）
		if col.Type == schema.TYPE_NUMBER { // 仅当原列为 number
			col.Type = schema.TYPE_DATETIME // 暂改类型以复用 DATETIME 的处理分支

			v := reflect.ValueOf(value)
			switch v.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fieldValue = r.makeReqColumnData(col, time.Unix(v.Int(), 0).Format(mysql.TimeFormat)) // 转本地时间字符串，再交给 DATETIME 流程输出 RFC3339
			}
		}
	}

	if fieldValue == nil {
		fieldValue = r.makeReqColumnData(col, value) // 默认回落到列类型感知转换
	}

	return fieldValue
}
