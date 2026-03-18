package consumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type TradeConsumer struct {
	reader *kafka.Reader
}

func NewTradeConsumer(brokers []string, topic string, groupID string) *TradeConsumer {
	return &TradeConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 1,
			MaxBytes: 10e6,
			MaxWait:  500 * time.Millisecond,
		}),
	}
}

func (c *TradeConsumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	slog.Info("settlement consumer started")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		slog.Info("received trade event",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"value", string(msg.Value),
			"time", msg.Time)
	}
}
