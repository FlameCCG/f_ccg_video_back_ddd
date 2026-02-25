package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigLoadRiverCamelCaseKeys(t *testing.T) {
	cfg := loadConfigForTest(t, `
river:
  enable: true
  serverID: 3012
  dataDir: ./runtime/river
  bulkSize: 256
  mysqlDump: ""
`)

	if cfg.River == nil {
		t.Fatal("River config should not be nil")
	}

	if cfg.River.ServerID != 3012 {
		t.Fatalf("expected ServerID=3012, got %d", cfg.River.ServerID)
	}

	if cfg.River.DataDir != "./runtime/river" {
		t.Fatalf("expected DataDir=./runtime/river, got %q", cfg.River.DataDir)
	}

	if cfg.River.BulkSize != 256 {
		t.Fatalf("expected BulkSize=256, got %d", cfg.River.BulkSize)
	}
}

func TestConfigLoadRiverCaseInsensitiveLegacyKeys(t *testing.T) {
	// mapstructure 默认大小写不敏感，全小写历史键仍可解析。
	cfg := loadConfigForTest(t, `
river:
  enable: true
  serverid: 3012
  datadir: ./runtime/river
  bulksize: 256
`)

	if cfg.River == nil {
		t.Fatal("River config should not be nil")
	}

	if cfg.River.ServerID != 3012 {
		t.Fatalf("expected ServerID=3012, got %d", cfg.River.ServerID)
	}

	if cfg.River.DataDir != "./runtime/river" {
		t.Fatalf("expected DataDir=./runtime/river, got %q", cfg.River.DataDir)
	}

	if cfg.River.BulkSize != 256 {
		t.Fatalf("expected BulkSize=256, got %d", cfg.River.BulkSize)
	}
}

func TestConfigLoadRiverIgnoresSnakeCaseKeys(t *testing.T) {
	cfg := loadConfigForTest(t, `
river:
  enable: true
  server_id: 4012
  data_dir: ./runtime/canonical
  bulk_size: 512
`)

	if cfg.River == nil {
		t.Fatal("River config should not be nil")
	}

	if cfg.River.ServerID != 0 {
		t.Fatalf("expected ServerID=0, got %d", cfg.River.ServerID)
	}

	if cfg.River.DataDir != "" {
		t.Fatalf("expected DataDir to be empty, got %q", cfg.River.DataDir)
	}

	if cfg.River.BulkSize != 0 {
		t.Fatalf("expected BulkSize=0, got %d", cfg.River.BulkSize)
	}
}

func TestConfigLoadTranscodeCamelCaseKeys(t *testing.T) {
	cfg := loadConfigForTest(t, `
transcode:
  maxWorkers: 4
  mp4Enable: false
  dashEnable: false
  dashSegDuration: 4
  cpuMode: true
  gpuMode: false
  highBitrateThreshold: 8000
`)

	if cfg.Transcode == nil {
		t.Fatal("Transcode config should not be nil")
	}
	if cfg.Transcode.MaxWorkers != 4 {
		t.Fatalf("MaxWorkers=%d, want 4", cfg.Transcode.MaxWorkers)
	}
	if cfg.Transcode.Mp4Enable || cfg.Transcode.DashEnable {
		t.Fatalf("want both pipelines disabled, got mp4=%v dash=%v",
			cfg.Transcode.Mp4Enable, cfg.Transcode.DashEnable)
	}
	if cfg.Transcode.DashSegDuration != 4 {
		t.Fatalf("DashSegDuration=%d, want 4", cfg.Transcode.DashSegDuration)
	}
}

func TestConfigLoadLocalTranscodeConsumer(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want bool
	}{
		{"legacy config participates", "transcode:\n  cpuMode: true\n", false},
		{"explicitly participates", "transcode:\n  localConsumerDisabled: false\n", false},
		{"disabled locally", "transcode:\n  localConsumerDisabled: true\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadConfigForTest(t, tt.raw)
			if cfg.Transcode == nil || cfg.Transcode.LocalConsumerDisabled != tt.want {
				t.Fatalf("unexpected local consumer config: %+v", cfg.Transcode)
			}
		})
	}
}

func loadConfigForTest(t *testing.T, raw string) *Config {
	t.Helper()

	v := viper.New()
	v.SetConfigType("yaml")

	if err := v.ReadConfig(strings.NewReader(raw)); err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	cfg := new(Config)
	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	return cfg
}
