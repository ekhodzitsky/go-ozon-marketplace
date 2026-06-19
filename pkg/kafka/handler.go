package kafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type consumerHandler struct {
	consumer *Consumer
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.process(session, msg); err != nil {
			h.consumer.log.Error("failed to process message",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (h *consumerHandler) process(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {
	c := h.consumer

	ctx, cancel := context.WithTimeout(session.Context(), c.cfg.ProcessTimeout)
	defer cancel()

	var err error
	attempts := 0
	backoff := c.cfg.InitialBackoff

	for attempts = 0; attempts <= c.cfg.MaxRetries; attempts++ {
		err = c.processor.Process(ctx, msg)
		if err == nil {
			session.MarkMessage(msg, "")
			return nil
		}

		if c.cfg.IsPermanent(err) {
			break
		}

		if attempts < c.cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	c.log.Error("message processing failed",
		zap.String("topic", msg.Topic),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
		zap.Int("attempts", attempts),
		zap.Error(err),
	)

	if c.cfg.IsPermanent(err) {
		h.sendToDLQ(msg, err, attempts)
		session.MarkMessage(msg, "")
		return nil
	}

	if attempts > c.cfg.MaxRetries {
		h.sendToDLQ(msg, err, attempts)
		if c.cfg.DLQTopic != "" {
			session.MarkMessage(msg, "")
		}
		return nil
	}

	return err
}
