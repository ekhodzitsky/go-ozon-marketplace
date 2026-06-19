package kafka

import (
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// DLQEnvelope is the default payload written to the dead-letter topic.
type DLQEnvelope struct {
	OriginalTopic string    `json:"original_topic"`
	Partition     int32     `json:"partition"`
	Offset        int64     `json:"offset"`
	Key           []byte    `json:"key"`
	Value         []byte    `json:"value"`
	Error         string    `json:"error"`
	Attempts      int       `json:"attempts"`
	Timestamp     time.Time `json:"timestamp"`
}

func (h *consumerHandler) sendToDLQ(msg *sarama.ConsumerMessage, err error, attempts int) {
	c := h.consumer
	if c.dlq == nil || c.cfg.DLQTopic == "" {
		return
	}

	envelope := DLQEnvelope{
		OriginalTopic: msg.Topic,
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		Key:           msg.Key,
		Value:         msg.Value,
		Error:         err.Error(),
		Attempts:      attempts,
		Timestamp:     time.Now().UTC(),
	}

	payload, jerr := json.Marshal(envelope)
	if jerr != nil {
		c.log.Error("failed to marshal dlq message", zap.Error(jerr))
		return
	}

	if perr := c.dlq.SendMessage(c.cfg.DLQTopic, msg.Key, payload); perr != nil {
		c.log.Error("failed to produce dlq message", zap.Error(perr))
	}
}
