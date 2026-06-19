package kafka

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var errPermanent = errors.New("permanent error")

func TestConsumer_SuccessMarksMessage(t *testing.T) {
	t.Parallel()

	msg := &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 1, Value: []byte(`{}`)}
	claim := &fakeClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- msg
	close(claim.messages)

	session := &fakeSession{}
	group := &fakeGroup{session: session, claim: claim}

	processor := ProcessorFunc(func(ctx context.Context, m *sarama.ConsumerMessage) error {
		assert.Equal(t, msg, m)
		return nil
	})

	cfg := Config{Topics: []string{"events"}, MaxRetries: 1, InitialBackoff: time.Millisecond, ProcessTimeout: time.Second}
	c := NewConsumerFromGroup(group, cfg, processor, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	require.Eventually(t, func() bool { return session.MarkedLen() == 1 }, time.Second, 10*time.Millisecond)
	require.NoError(t, c.Close())
}

func TestConsumer_TransientErrorDoesNotMark(t *testing.T) {
	t.Parallel()

	msg := &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 2, Value: []byte(`{}`)}
	claim := &fakeClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- msg
	close(claim.messages)

	session := &fakeSession{}
	group := &fakeGroup{session: session, claim: claim}

	processor := ProcessorFunc(func(ctx context.Context, m *sarama.ConsumerMessage) error {
		return errors.New("transient")
	})

	cfg := Config{Topics: []string{"events"}, MaxRetries: 2, InitialBackoff: time.Millisecond, ProcessTimeout: time.Second}
	c := NewConsumerFromGroup(group, cfg, processor, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	require.Eventually(t, func() bool { return group.consumed.Load() }, time.Second, 10*time.Millisecond)
	require.NoError(t, c.Close())
	assert.Zero(t, session.MarkedLen())
}

func TestConsumer_PermanentErrorSendsToDLQ(t *testing.T) {
	t.Parallel()

	msg := &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 3, Value: []byte(`{}`)}
	claim := &fakeClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- msg
	close(claim.messages)

	session := &fakeSession{}
	group := &fakeGroup{session: session, claim: claim}
	producer := &fakeProducer{}

	processor := ProcessorFunc(func(ctx context.Context, m *sarama.ConsumerMessage) error {
		return errPermanent
	})

	cfg := Config{
		Topics:         []string{"events"},
		DLQTopic:       "dlq",
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
		ProcessTimeout: time.Second,
		IsPermanent:    func(err error) bool { return errors.Is(err, errPermanent) },
	}
	c := NewConsumerFromGroup(group, cfg, processor, zap.NewNop())
	c.SetDLQProducer(producer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	require.Eventually(t, func() bool {
		return session.MarkedLen() == 1 && producer.SentLen() == 1
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, c.Close())

	assert.Equal(t, "dlq", producer.First().topic)
}

func TestConsumer_RetryThenSuccess(t *testing.T) {
	t.Parallel()

	msg := &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 4, Value: []byte(`{}`)}
	claim := &fakeClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- msg
	close(claim.messages)

	session := &fakeSession{}
	group := &fakeGroup{session: session, claim: claim}

	var calls atomic.Int32
	processor := ProcessorFunc(func(ctx context.Context, m *sarama.ConsumerMessage) error {
		if calls.Add(1) < 2 {
			return errors.New("transient")
		}
		return nil
	})

	cfg := Config{Topics: []string{"events"}, MaxRetries: 3, InitialBackoff: time.Millisecond, ProcessTimeout: time.Second}
	c := NewConsumerFromGroup(group, cfg, processor, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	require.Eventually(t, func() bool { return session.MarkedLen() == 1 }, time.Second, 10*time.Millisecond)
	require.NoError(t, c.Close())
	assert.Equal(t, int32(2), calls.Load())
}

type fakeGroup struct {
	session  sarama.ConsumerGroupSession
	claim    sarama.ConsumerGroupClaim
	consumed atomic.Bool
}

func (f *fakeGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	if err := handler.Setup(f.session); err != nil {
		return err
	}
	f.consumed.Store(true)
	err := handler.ConsumeClaim(f.session, f.claim)
	_ = handler.Cleanup(f.session)
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return err
	}
}

func (f *fakeGroup) Errors() <-chan error                       { return nil }
func (f *fakeGroup) Close() error                               { return nil }
func (f *fakeGroup) Pause(map[string][]int32)                   {}
func (f *fakeGroup) Resume(map[string][]int32)                  {}
func (f *fakeGroup) PauseAll()                                  {}
func (f *fakeGroup) ResumeAll()                                 {}

type fakeSession struct {
	mu     sync.Mutex
	marked []*sarama.ConsumerMessage
}

func (f *fakeSession) Claims() map[string][]int32               { return nil }
func (f *fakeSession) MemberID() string                         { return "" }
func (f *fakeSession) GenerationID() int32                      { return 0 }
func (f *fakeSession) MarkOffset(string, int32, int64, string)  {}
func (f *fakeSession) Commit()                                  {}
func (f *fakeSession) ResetOffset(string, int32, int64, string) {}
func (f *fakeSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marked = append(f.marked, msg)
}
func (f *fakeSession) Context() context.Context { return context.Background() }

func (f *fakeSession) MarkedLen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.marked)
}

type fakeClaim struct {
	messages chan *sarama.ConsumerMessage
}

func (f *fakeClaim) Topic() string              { return "events" }
func (f *fakeClaim) Partition() int32           { return 0 }
func (f *fakeClaim) InitialOffset() int64       { return 0 }
func (f *fakeClaim) HighWaterMarkOffset() int64 { return 1 }
func (f *fakeClaim) Messages() <-chan *sarama.ConsumerMessage {
	return f.messages
}

type fakeProducer struct {
	mu   sync.Mutex
	sent []fakeProducerMsg
}

type fakeProducerMsg struct {
	topic string
	key   []byte
	value []byte
}

func (f *fakeProducer) SendMessage(topic string, key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, fakeProducerMsg{topic: topic, key: key, value: value})
	return nil
}

func (f *fakeProducer) SentLen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func (f *fakeProducer) First() fakeProducerMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return fakeProducerMsg{}
	}
	return f.sent[0]
}

func (f *fakeProducer) Close() error { return nil }
