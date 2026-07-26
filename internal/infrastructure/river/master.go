package river

import (
	"bytes" // 用于把 toml 编码写入内存缓冲，随后一次性落盘
	"os"    // 文件/目录操作
	"path"  // 拼接 dataDir/master.info 路径
	"sync"  // 并发安全的互斥锁/读写锁
	"time"  // 节流保存、时间戳管理

	"github.com/BurntSushi/toml"           // TOML 编解码（master.info 的存储格式）
	"github.com/juju/errors"               // 错误包装（带 Cause/Stack）
	"github.com/siddontang/go-log/log"     // 日志
	"github.com/siddontang/go-mysql/mysql" // mysql.Position 类型（Name+Pos）
	"github.com/siddontang/go/ioutil2"     // WriteFileAtomic：原子写文件，防止部分写入
)

// masterInfoFileName binlog 位点文件名（路径为 river.dataDir/master.info，属于外部契约）.
const masterInfoFileName = "master.info"

type masterInfo struct {
	sync.RWMutex           // 读写锁：并发读 Position 和写 Save 时保证一致性
	Name         string    `toml:"bin_name"` // 当前消费到的 binlog 文件名（写入 toml 字段 bin_name）
	Pos          uint32    `toml:"bin_pos"`  // 当前消费到的 binlog 偏移量（写入 toml 字段 bin_pos）
	filePath     string    // master.info 的绝对路径（dataDir/master.info）
	lastSaveTime time.Time // 上次成功保存时间，用于“至少间隔 1s 才落盘一次”的节流
}

func loadMasterInfo(dataDir string) (*masterInfo, error) {
	var m masterInfo // 初始化一个零值 masterInfo（Name/Pos 为空；内存态）

	if len(dataDir) == 0 { // 若未配置持久化目录：仅内存保存，不做落盘
		return &m, nil
	}

	m.filePath = path.Join(dataDir, masterInfoFileName) // 约定的位点文件名
	// lastSaveTime 保持零值：保证首次 Save 一定落盘；之后才启用 1s 节流。

	if err := os.MkdirAll(dataDir, 0o755); err != nil { // 确保目录存在（幂等）
		return nil, errors.Trace(err)
	}

	f, err := os.Open(m.filePath)                        // 尝试打开已有位点文件
	if err != nil && !os.IsNotExist(errors.Cause(err)) { // 打开失败且不是“文件不存在”
		return nil, errors.Trace(err)
	} else if os.IsNotExist(errors.Cause(err)) { // 文件不存在：首次启动或未落盘过
		return &m, nil // 返回零值（从 binlog 默认位点开始，由上层 canal 决定）
	}

	defer func() { _ = f.Close() }()

	_, err = toml.NewDecoder(f).Decode(&m) // 从 toml 反序列化 Name/Pos 字段到 m

	return &m, errors.Trace(err) // 返回已加载的位点（若解析失败则返回错误）
}

func (m *masterInfo) Save(pos mysql.Position) error {
	return m.save(pos, false)
}

// save 持久化位点。force=true 时忽略 1s 节流（用于 Close 兜底刷盘）。
func (m *masterInfo) save(pos mysql.Position, force bool) error {
	log.Infof("save position %s", pos) // 打日志：可观测位点前进

	m.Lock()         // 写锁：防止并发读写引起数据竞争
	defer m.Unlock() // 函数返回时释放锁

	m.Name = pos.Name // 覆盖当前记录的 binlog 文件名
	m.Pos = pos.Pos   // 覆盖当前记录的偏移量

	if len(m.filePath) == 0 { // 若未配置 filePath（即 dataDir 为空）
		return nil // 仅更新内存，不落盘
	}

	n := time.Now()
	if !force && n.Sub(m.lastSaveTime) < time.Second { // 小于 1s 的频繁保存直接忽略（节流，避免磁盘抖动）
		return nil
	}

	m.lastSaveTime = n // 更新“上次保存时间”

	var buf bytes.Buffer

	e := toml.NewEncoder(&buf) // 用 toml 编码到内存缓冲（一次性写入，避免半包）

	if err := e.Encode(m); err != nil {
		return errors.Trace(err)
	}

	if err := ioutil2.WriteFileAtomic(m.filePath, buf.Bytes(), 0o644); err != nil { // 原子写：写临时文件后 rename 覆盖，防止崩溃留下半文件
		log.Errorf("canal save master info to file %s err %v", m.filePath, err) // 写盘失败记录错误

		return errors.Trace(err)
	}

	return nil
}

func (m *masterInfo) Position() mysql.Position {
	m.RLock()         // 读锁：允许并发多读，避免和 Save 冲突
	defer m.RUnlock() // 读完释放锁

	return mysql.Position{ // 返回一个值拷贝，避免外部修改内部状态
		Name: m.Name,
		Pos:  m.Pos,
	}
}

func (m *masterInfo) Close() error { // 关闭前强制落盘当前位点（忽略 1s 节流）
	return m.save(m.Position(), true)
}
