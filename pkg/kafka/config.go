package kafka

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

// Config — всё, что нужно для запуска consumer group.
type Config struct {
	Brokers           []string
	GroupID           string
	Topics            []string
	DLQTopic          string
	RebalanceStrategy string // "roundrobin" (default), "range", "sticky"
	InitialOffset     int64  // sarama.OffsetOldest or sarama.OffsetNewest
	MaxRetries        int
	InitialBackoff    time.Duration
	ProcessTimeout    time.Duration
	IsPermanent       IsPermanentError
}

func (c *Config) setDefaults() {
	if c.RebalanceStrategy == "" {
		c.RebalanceStrategy = "roundrobin"
	}
	if c.InitialOffset == 0 {
		c.InitialOffset = sarama.OffsetOldest
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = 50 * time.Millisecond
	}
	if c.ProcessTimeout <= 0 {
		c.ProcessTimeout = 10 * time.Second
	}
	if c.IsPermanent == nil {
		c.IsPermanent = func(error) bool { return false }
	}
}

func (c *Config) saramaConfig() (*sarama.Config, error) {
	sc := sarama.NewConfig()

	switch c.RebalanceStrategy {
	case "roundrobin":
		sc.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	case "range":
		sc.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()
	case "sticky":
		sc.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()
	default:
		return nil, fmt.Errorf("unsupported rebalance strategy: %q", c.RebalanceStrategy)
	}

	sc.Consumer.Offsets.Initial = c.InitialOffset
	return sc, nil
}
