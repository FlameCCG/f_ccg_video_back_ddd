package mq

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"testing"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestTranscodeConsumerComponentSkipsDisabledLocalConsumer(t *testing.T) {
	t.Parallel()

	provider := infraconfig.NewProvider(&config.Config{
		Transcode: &config.Transcode{LocalConsumerDisabled: true},
	})
	logs, observed := observer.New(zap.InfoLevel)
	// Kafka 保持开启；其余依赖不提供，确保关闭时不构造消费者、不连接 Kafka。
	c := NewTranscodeConsumerComponent(provider, &config.Kafka{Enable: true}, nil, nil, nil, nil, zap.New(logs))
	for range 2 {
		if err := c.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	if c.consumer != nil || c.retryCancel != nil {
		t.Fatal("disabled local consumer must not start a consumer or background retries")
	}

	if observed.FilterMessage("local transcode consumer disabled, skip transcode consumer").Len() != 1 {
		t.Fatal("disabled startup should be logged once")
	}

	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
