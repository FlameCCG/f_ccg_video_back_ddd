package config

// 窄化 provider：把根配置拆成各基础组件真正需要的最小配置结构体，
// 避免组件直接吞整个大配置（见 docs/refactor-spec.md 第 2 节）。
//
// 约定：
//   - 启动期一次性读取的组件（数据库/缓存/搜索/消息队列客户端等）注入窄配置值或指针；
//   - 需要读取运行期热更新配置的组件（站点配置、三方登录、AI、邮件等）注入 *Provider。

// ProvideServerConf 返回 HTTP 服务配置。
func ProvideServerConf(p *Provider) *Server {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Server
}

// ProvideLoggerConf 返回日志配置。
func ProvideLoggerConf(p *Provider) *Logger {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Logger
}

// ProvideMysqlConf 返回 MySQL 配置。
func ProvideMysqlConf(p *Provider) *Mysql {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}
	return cfg.Mysql
}

// ProvideRedisConf 返回 Redis 配置。
func ProvideRedisConf(p *Provider) *Redis {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}
	return cfg.Redis
}

// ProvideMongoConf 返回 Mongo 配置。
func ProvideMongoConf(p *Provider) *Mongo {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Mongo
}

// ProvideESConf 返回 Elasticsearch 配置。
func ProvideESConf(p *Provider) *ES {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.ES
}

// ProvideKafkaConf 返回 Kafka 配置。
func ProvideKafkaConf(p *Provider) *Kafka {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Kafka
}

// ProvideRiverConf 返回 River（binlog → ES）配置。
func ProvideRiverConf(p *Provider) *River {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.River
}

// ProvideTranscodeConf 返回转码配置。
func ProvideTranscodeConf(p *Provider) *Transcode {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Transcode
}

// ProvideJwtConf 返回 JWT 配置。
func ProvideJwtConf(p *Provider) *Jwt {
	cfg := p.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Jwt
}
