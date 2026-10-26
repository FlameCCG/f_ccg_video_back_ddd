package mq

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRetriesRetainTaskAndGenerateNewAttemptWithDLQHeaders(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	original := logmeta.Values{RequestID: "request-id", UserID: "7", TaskID: testStableTask}
	message := &sarama.ConsumerMessage{Topic: "original-topic", Partition: 2, Offset: 31, Value: []byte(`{"videoId":1}`)}
	setCorrelation(message, original)

	var attempts []logmeta.Values

	handler := consumerGroupHandler{logger: zap.New(core), fn: func(session sarama.ConsumerGroupSession, _ *sarama.ConsumerMessage) error {
		attempts = append(attempts, logmeta.From(session.Context()))
		if len(attempts) == 1 {
			return errors.New("temporary")
		}

		return nil
	}}

	session := stubConsumerGroupSession{ctx: context.Background()}
	{
		err := handler.handleMessage(session, message)
		require.NoError(t, err)
	}

	require.Len(t, attempts, 2, "bad retry correlation: %+v", attempts)
	require.Equal(t, testStableTask, attempts[0].TaskID, "bad retry correlation: %+v", attempts)
	require.Equal(t, testStableTask, attempts[1].TaskID, "bad retry correlation: %+v", attempts)
	require.NotEqual(t, attempts[0].AttemptID, attempts[1].AttemptID, "bad retry correlation: %+v", attempts)
	require.NotEmpty(t, attempts[0].AttemptID, "bad retry correlation: %+v", attempts)

	require.False(t, logs.FilterLevelExact(zap.ErrorLevel).Len() != 0 || logs.FilterMessage("kafka.message.retry").Len() != 1, "retry emitted terminal failure: %#v", logs.All())

	producer := &trackingDLQProducer{}
	handler.producer = producer

	handler.dlqTopic = "dlq"
	{
		err := handler.publishDLQ(message, errors.New("failed"))
		require.NoError(t, err)
	}

	require.Len(t, producer.messages, 1, "DLQ not published")

	dlq := producer.messages[0]

	headers := make([]*sarama.RecordHeader, 0, len(dlq.Headers))
	for _, h := range dlq.Headers {
		header := h
		headers = append(headers, &header)
	}

	restored := logmeta.From(messageContext(context.Background(), &sarama.ConsumerMessage{Headers: headers}))
	require.Equal(t, "request-id", restored.RequestID, "DLQ lost correlation: %+v", restored)
	require.Equal(t, testStableTask, restored.TaskID, "DLQ lost correlation: %+v", restored)
	require.Equal(t, restored.AttemptID, attempts[1].AttemptID, "DLQ lost correlation: %+v", restored)
}

func TestLegacyCorrelationIsStableAndIncludesOriginalPosition(t *testing.T) {
	t.Parallel()

	message := &sarama.ConsumerMessage{Topic: "old-topic", Partition: 1, Offset: 13}
	first := logmeta.From(messageContext(context.Background(), message))

	second := logmeta.From(messageContext(context.Background(), message))

	require.NotEmpty(t, first.TaskID, "legacy message identity is unstable")
	require.Equal(t, first.TaskID, second.TaskID, "legacy message identity is unstable")

	message.Offset++

	other := logmeta.From(messageContext(context.Background(), message))
	require.NotEqual(t, first.TaskID, other.TaskID, "distinct offsets share identity")
}

type errorChannelGroup struct {
	sarama.ConsumerGroup
	errors chan error
}

func (g *errorChannelGroup) Errors() <-chan error { return g.errors }
func TestConsumerErrorListenersFollowGroupLifetime(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	consumer := &KafkaConsumer{logger: zap.New(core)}
	old := &errorChannelGroup{errors: make(chan error)}
	next := &errorChannelGroup{errors: make(chan error)}

	stopOld := consumer.listenErrors(old)
	defer stopOld()

	old.errors <- errors.New("old group error")

	stopNext := consumer.listenErrors(next)
	defer stopNext()

	stopOld()

	next.errors <- errors.New("new group error")

	close(next.errors)
	stopNext()

	require.Equal(t, 2, logs.FilterMessage("kafka.consumer.failed").Len(), "lost group errors: %#v", logs.All())
}

func TestMessagePanicRetainsCauseAndSingleTerminalDetail(t *testing.T) {
	t.Parallel()

	cause := errors.New("panic cause")
	handler := consumerGroupHandler{fn: func(sarama.ConsumerGroupSession, *sarama.ConsumerMessage) error { panic(cause) }}

	err := handler.handleMessage(stubConsumerGroupSession{ctx: context.Background()}, &sarama.ConsumerMessage{Topic: "task"})
	require.ErrorIs(t, err, cause, "panic cause lost: %v", err)

	core, logs := observer.New(zap.DebugLevel)
	recordMessageFailure(zap.New(core), "transcode.task.failed", err)

	entries := logs.FilterLevelExact(zap.ErrorLevel).All()
	require.False(t, len(entries) != 1 || entries[0].ContextMap()["stack"] == nil, "panic boundary=%#v", logs.All())
}
