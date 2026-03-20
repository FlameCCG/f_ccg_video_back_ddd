package mq

import "go.uber.org/zap"

// newProducerPoolWithFactory 只用于在测试中替换 Kafka 工厂，不进入生产包。
func newProducerPoolWithFactory(factory ProducerFactory, logger *zap.Logger) *ProducerPool {
	return &ProducerPool{factory: factory, logger: logger}
}
