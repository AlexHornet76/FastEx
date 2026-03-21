package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AlexHornet76/FastEx/marketdata/events"
	"github.com/AlexHornet76/FastEx/marketdata/state"
	"github.com/segmentio/kafka-go"
)

type TradeConsumer struct {
	reader *kafka.Reader
	store  *state.Store
}

func NewTradeConsumer(brokers []string, topic, groupID string, store *state.Store) *TradeConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})

	return &TradeConsumer{reader: r, store: store}
}

func (c *TradeConsumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	slog.Info("marketdata consumer started")

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		var ev events.TradeExecutedEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			slog.Error("marketdata: failed to unmarshal trade event", "error", err, "value", string(msg.Value))
			continue
		}

		// If event_time missing/zero, fallback to Kafka msg time
		t := ev.EventTime
		if t.IsZero() {
			t = msg.Time
		}

		c.store.ApplyTrade(ev.Instrument, ev.Price, ev.Quantity, t)

		slog.Info("marketdata: ticker updated",
			"instrument", ev.Instrument,
			"price", ev.Price,
			"qty", ev.Quantity,
			"trade_id", ev.TradeID,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)
	}
}
