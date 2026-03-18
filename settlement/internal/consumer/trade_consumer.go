package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AlexHornet76/FastEx/settlement/internal/events"
	"github.com/AlexHornet76/FastEx/settlement/internal/settle"
	"github.com/segmentio/kafka-go"
)

type TradeConsumer struct {
	reader  *kafka.Reader
	settler *settle.Settler
}

func NewTradeConsumer(brokers []string, topic string, groupID string, settler *settle.Settler) *TradeConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})

	return &TradeConsumer{reader: r, settler: settler}
}

func (c *TradeConsumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	slog.Info("settlement consumer started")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var ev events.TradeExecutedEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			slog.Error("failed to unmarshal trade event", "error", err, "value", string(msg.Value))
			// For now: skip bad messages (don't crash loop)
			continue
		}

		applied, err := c.settler.ApplyTrade(ctx, &ev)
		if err != nil {
			slog.Error("failed to apply trade", "trade_id", ev.TradeID, "error", err)
			// Important: returning error will cause consumer restart and retry (at-least-once).
			return err
		}

		if applied {
			slog.Info("trade applied",
				"trade_id", ev.TradeID,
				"instrument", ev.Instrument,
				"qty", ev.Quantity,
				"price", ev.Price,
				"kafka_partition", msg.Partition,
				"kafka_offset", msg.Offset)
		} else {
			slog.Info("trade skipped (already processed)",
				"trade_id", ev.TradeID,
				"kafka_partition", msg.Partition,
				"kafka_offset", msg.Offset)
		}
	}
}
