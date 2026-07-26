package config

import "f_ccg_video_back_ddd/internal/infrastructure/river/rule"

// River 类型.
type River struct {
	Enable        bool          `mapstructure:"enable" json:"enable"`
	ServerID      uint32        `mapstructure:"serverID" json:"serverID"`
	Flavor        string        `mapstructure:"flavor" json:"flavor"`
	DataDir       string        `mapstructure:"dataDir" json:"dataDir"`
	MysqldumpPath string        `mapstructure:"mysqlDump" json:"mysqlDump"`
	Sources       []RiverSource `mapstructure:"source" json:"source"`
	Rules         []*rule.Rule  `mapstructure:"rules" json:"rules"`
	BulkSize      int           `mapstructure:"bulkSize" json:"bulkSize"`
}

// RiverSource 类型.
type RiverSource struct {
	Schema string   `mapstructure:"schema" json:"schema"`
	Tables []string `mapstructure:"tables" json:"tables"`
}
