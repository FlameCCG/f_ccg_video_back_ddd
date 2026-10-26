package config

import "f_ccg_video_back_ddd/internal/config"

// 窄化 provider：把根配置拆成各基础组件真正需要的最小配置结构体，
// 避免组件直接吞整个大配置（见 docs/architecture.md 第 6 节）。
//
// 约定：
//   - 启动期一次性读取的组件（数据库/缓存/搜索/消息队列客户端等）注入窄配置值或指针；
//   - 需要读取运行期热更新配置的组件（站点配置、三方登录、AI、邮件等）注入 *Provider。

// ProvideLoggerConf 返回日志配置。
func ProvideLoggerConf(p *Provider) *config.Logger {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Logger
}

// ProvideMysqlConf 返回 MySQL 配置。
func ProvideMysqlConf(p *Provider) *config.Mysql {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}
	return cfg.Mysql
}

// ProvideRedisConf 返回 Redis 配置。
func ProvideRedisConf(p *Provider) *config.Redis {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}
	return cfg.Redis
}

// ProvideMongoConf 返回 Mongo 配置。
func ProvideMongoConf(p *Provider) *config.Mongo {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Mongo
}

// ProvideESConf 返回 Elasticsearch 配置。
func ProvideESConf(p *Provider) *config.ES {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.ES
}

// ProvideKafkaConf 返回 Kafka 配置。
func ProvideKafkaConf(p *Provider) *config.Kafka {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Kafka
}

// ProvideRiverConf 返回 River（binlog → ES）配置。
func ProvideRiverConf(p *Provider) *config.River {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.River
}
