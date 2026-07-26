package config

// Kafka 定义 Kafka 客户端初始化所需的主要配置.
type Kafka struct {
	Enable              bool              `mapstructure:"enable" json:"enable"`
	Brokers             []string          `mapstructure:"brokers" json:"brokers"`
	ClientID            string            `mapstructure:"clientID" json:"clientID"`
	GroupID             string            `mapstructure:"groupID" json:"groupID"`
	Version             string            `mapstructure:"version" json:"version"`
	Topics              map[string]string `mapstructure:"topics" json:"topics"`
	RebalanceTimeoutSec int               `mapstructure:"rebalanceTimeoutSec" json:"rebalanceTimeoutSec"` // rebalance 等待 claim 释放的最长时间(秒)，长任务消费建议 >= 最大单任务耗时
}
