package river

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/siddontang/go-mysql/mysql"
)

// 迁移自原 service/river_service/master_test.go。
// 位点文件路径（dataDir/master.info）与 toml 字段名（bin_name / bin_pos）属于外部契约。

func TestMasterInfo_CloseForceFlushDespiteThrottle(t *testing.T) {
	dir := t.TempDir()

	m, err := loadMasterInfo(dir)
	if err != nil {
		t.Fatalf("loadMasterInfo: %v", err)
	}

	// 第一次落盘：建立 lastSaveTime。
	pos1 := mysql.Position{Name: "mysql-bin.000001", Pos: 100}
	if err := m.Save(pos1); err != nil {
		t.Fatalf("Save pos1: %v", err)
	}

	// 1s 节流窗口内再次 Save 应被忽略。
	pos2 := mysql.Position{Name: "mysql-bin.000001", Pos: 200}
	if err := m.Save(pos2); err != nil {
		t.Fatalf("Save pos2: %v", err)
	}

	// 内存已更新，但文件仍应是 pos1（节流）。
	if got := m.Position(); got != pos2 {
		t.Fatalf("in-memory position = %+v, want %+v", got, pos2)
	}

	filePos, err := readMasterInfoFile(filepath.Join(dir, masterInfoFileName))
	if err != nil {
		t.Fatalf("read after throttled save: %v", err)
	}

	if filePos != pos1 {
		t.Fatalf("file position after throttled save = %+v, want %+v (pos1)", filePos, pos1)
	}

	// Close 必须强制刷盘最终位点，不能被 1s 节流吞掉。
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	filePos, err = readMasterInfoFile(filepath.Join(dir, masterInfoFileName))
	if err != nil {
		t.Fatalf("read after Close: %v", err)
	}

	if filePos != pos2 {
		t.Fatalf("file position after Close = %+v, want %+v", filePos, pos2)
	}
}

func TestMasterInfo_SaveThrottleAndReload(t *testing.T) {
	dir := t.TempDir()

	m, err := loadMasterInfo(dir)
	if err != nil {
		t.Fatalf("loadMasterInfo: %v", err)
	}

	pos := mysql.Position{Name: "mysql-bin.000009", Pos: 4096}
	if err := m.Save(pos); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 等待超过节流窗口后再次落盘。
	time.Sleep(1100 * time.Millisecond)

	pos2 := mysql.Position{Name: "mysql-bin.000009", Pos: 8192}
	if err := m.Save(pos2); err != nil {
		t.Fatalf("Save pos2: %v", err)
	}

	reloaded, err := loadMasterInfo(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if got := reloaded.Position(); got != pos2 {
		t.Fatalf("reloaded position = %+v, want %+v", got, pos2)
	}
}

// TestMasterInfo_MemoryOnlyWithoutDataDir dataDir 为空时只更新内存、不落盘。
func TestMasterInfo_MemoryOnlyWithoutDataDir(t *testing.T) {
	m, err := loadMasterInfo("")
	if err != nil {
		t.Fatalf("loadMasterInfo: %v", err)
	}

	pos := mysql.Position{Name: "mysql-bin.000003", Pos: 512}
	if err := m.Save(pos); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := m.Position(); got != pos {
		t.Fatalf("in-memory position = %+v, want %+v", got, pos)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func readMasterInfoFile(path string) (mysql.Position, error) {
	f, err := os.Open(path)
	if err != nil {
		return mysql.Position{}, err
	}

	defer func() { _ = f.Close() }()

	var raw struct {
		Name string `toml:"bin_name"`
		Pos  uint32 `toml:"bin_pos"`
	}

	if _, err := toml.NewDecoder(f).Decode(&raw); err != nil {
		return mysql.Position{}, err
	}

	return mysql.Position{Name: raw.Name, Pos: raw.Pos}, nil
}
