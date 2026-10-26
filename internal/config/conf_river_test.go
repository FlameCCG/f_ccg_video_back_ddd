package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadRiverCamelCaseKeys(t *testing.T) {
	t.Parallel()

	cfg := loadConfigForTest(t, `
river:
  enable: true
  serverID: 3012
  dataDir: ./runtime/river
  bulkSize: 256
  mysqlDump: ""
`)

	require.NotNil(t, cfg.River, "River config should not be nil")

	require.EqualValues(t, 3012, cfg.River.ServerID, "expected ServerID=3012, got %d", cfg.River.ServerID)

	require.Equal(t, "./runtime/river", cfg.River.DataDir, "expected DataDir=./runtime/river, got %q", cfg.River.DataDir)

	require.Equal(t, 256, cfg.River.BulkSize, "expected BulkSize=256, got %d", cfg.River.BulkSize)
}

func TestConfigLoadRiverCaseInsensitiveLegacyKeys(t *testing.T) {
	t.Parallel()

	// mapstructure 默认大小写不敏感，全小写历史键仍可解析。
	cfg := loadConfigForTest(t, `
river:
  enable: true
  serverid: 3012
  datadir: ./runtime/river
  bulksize: 256
`)

	require.NotNil(t, cfg.River, "River config should not be nil")

	require.EqualValues(t, 3012, cfg.River.ServerID, "expected ServerID=3012, got %d", cfg.River.ServerID)

	require.Equal(t, "./runtime/river", cfg.River.DataDir, "expected DataDir=./runtime/river, got %q", cfg.River.DataDir)

	require.Equal(t, 256, cfg.River.BulkSize, "expected BulkSize=256, got %d", cfg.River.BulkSize)
}

func TestConfigLoadRiverIgnoresSnakeCaseKeys(t *testing.T) {
	t.Parallel()

	cfg := loadConfigForTest(t, `
river:
  enable: true
  server_id: 4012
  data_dir: ./runtime/canonical
  bulk_size: 512
`)

	require.NotNil(t, cfg.River, "River config should not be nil")

	require.EqualValues(t, 0, cfg.River.ServerID, "expected ServerID=0, got %d", cfg.River.ServerID)

	require.Empty(t, cfg.River.DataDir, "expected DataDir to be empty, got %q", cfg.River.DataDir)

	require.Equal(t, 0, cfg.River.BulkSize, "expected BulkSize=0, got %d", cfg.River.BulkSize)
}

func TestConfigLoadTranscodeCamelCaseKeys(t *testing.T) {
	t.Parallel()

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

	require.NotNil(t, cfg.Transcode, "Transcode config should not be nil")

	require.Equal(t, 4, cfg.Transcode.MaxWorkers, "MaxWorkers=%d, want 4", cfg.Transcode.MaxWorkers)

	require.False(t, cfg.Transcode.Mp4Enable, "want both pipelines disabled, got mp4=%v dash=%v", cfg.Transcode.Mp4Enable, cfg.Transcode.DashEnable)
	require.False(t, cfg.Transcode.DashEnable, "want both pipelines disabled, got mp4=%v dash=%v", cfg.Transcode.Mp4Enable, cfg.Transcode.DashEnable)

	require.Equal(t, 4, cfg.Transcode.DashSegDuration, "DashSegDuration=%d, want 4", cfg.Transcode.DashSegDuration)
}

func TestConfigLoadLocalTranscodeConsumer(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			cfg := loadConfigForTest(t, tt.raw)
			require.NotNil(t, cfg.Transcode, "unexpected local consumer config: %+v", cfg.Transcode)
			require.Equal(t, tt.want, cfg.Transcode.LocalConsumerDisabled, "unexpected local consumer config: %+v", cfg.Transcode)
		})
	}
}

func loadConfigForTest(t *testing.T, raw string) *Config {
	t.Helper()

	v := viper.New()
	v.SetConfigType("yaml")

	{
		err := v.ReadConfig(strings.NewReader(raw))
		require.NoError(t, err, "ReadConfig failed: %v", err)
	}

	cfg := new(Config)
	{
		err := v.Unmarshal(cfg)
		require.NoError(t, err, "Unmarshal failed: %v", err)
	}

	return cfg
}
