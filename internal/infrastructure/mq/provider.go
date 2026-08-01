package mq

import "github.com/google/wire"

// ProviderSet 汇总 Kafka 与转码消息基础设施构造器。
var ProviderSet = wire.NewSet(
	NewKafkaConsumer,
	NewKafkaDriver,
	NewMQService,
	NewProducerPool,
	NewTranscodeConsumer,
	NewTranscodeConsumerComponent,
	NewTranscodeEngine,
)
