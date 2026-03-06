package config

import "f_ccg_video_back_ddd/internal/infrastructure/river/rule"

// River 配置 binlog 同步开关、复制身份、位点目录、全量数据源与索引映射。
// ServerID 用于 MySQL 复制连接，多实例部署需核对身份和位点文件的归属。
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

// RiverSource 指定全量/增量同步关注的 schema 与表；字段投影和目标索引由 Rules 决定。
type RiverSource struct {
	Schema string   `mapstructure:"schema" json:"schema"`
	Tables []string `mapstructure:"tables" json:"tables"`
}
