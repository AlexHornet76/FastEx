package marketws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type TradeExecutedEvent struct {
	EventType  string    `json:"event_type"`
	EventTime  time.Time `json:"event_time"`
	Instrument string    `json:"instrument"`

	TradeID string `json:"trade_id"`

	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

type Broadcaster interface {
	BroadcastMarket(instrument string, msg any)
}

type Consumer struct {
	reader      *kafka.Reader
	broadcaster Broadcaster
}

func NewConsumer(brokers []string, topic string, groupID string, broadcaster Broadcaster) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})
	return &Consumer{reader: r, broadcaster: broadcaster}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	slog.Info("gateway market ws consumer started")

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var ev TradeExecutedEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			slog.Error("market ws: failed to unmarshal trade.executed", "error", err, "value", string(msg.Value))
			continue
		}

		// broadcast ticker update event (raw trade, clients can aggregate or UI uses last trade)
		out := map[string]any{
			"type":       "trade",
			"instrument": ev.Instrument,
			"price":      ev.Price,
			"quantity":   ev.Quantity,
			"trade_id":   ev.TradeID,
			"time":       ev.EventTime,
		}
		c.broadcaster.BroadcastMarket(ev.Instrument, out)
	}
}
