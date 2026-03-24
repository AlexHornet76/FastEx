// package marketws

// import (
// 	"context"
// 	"encoding/json"
// 	"log/slog"
// 	"time"

// 	"github.com/segmentio/kafka-go"
// )

// // TradeExecutedEvent is the JSON payload we expect on topic trade.executed
// type TradeExecutedEvent struct {
// 	EventType  string    `json:"event_type"`
// 	EventTime  time.Time `json:"event_time"`
// 	Instrument string    `json:"instrument"`

// 	TradeID string `json:"trade_id"`

// 	Price    int64 `json:"price"`
// 	Quantity int64 `json:"quantity"`
// }

// type Broadcaster interface {
// 	BroadcastMarketTrade(instrument string, msg any)
// }

// type Consumer struct {
// 	reader      *kafka.Reader
// 	broadcaster Broadcaster
// }

// func NewConsumer(brokers []string, topic string, groupID string, broadcaster Broadcaster) *Consumer {
// 	r := kafka.NewReader(kafka.ReaderConfig{
// 		Brokers:  brokers,
// 		Topic:    topic,
// 		GroupID:  groupID,
// 		MinBytes: 1,
// 		MaxBytes: 10e6,
// 		MaxWait:  500 * time.Millisecond,
// 	})
// 	return &Consumer{reader: r, broadcaster: broadcaster}
// }

// func (c *Consumer) Run(ctx context.Context) error {
// 	defer c.reader.Close()
// 	slog.Info("gateway market ws consumer started")

// 	for {
// 		msg, err := c.reader.ReadMessage(ctx)
// 		if err != nil {
// 			return err
// 		}

// 		var ev TradeExecutedEvent
// 		if err := json.Unmarshal(msg.Value, &ev); err != nil {
// 			slog.Error("market ws: failed to unmarshal trade.executed", "error", err, "value", string(msg.Value))
// 			continue
// 		}

// 		// broadcast ticker update event (raw trade, clients can aggregate or UI uses last trade)
// 		out := map[string]any{
// 			"type":       "trade",
// 			"instrument": ev.Instrument,
// 			"price":      ev.Price,
// 			"quantity":   ev.Quantity,
// 			"trade_id":   ev.TradeID,
// 			"time":       ev.EventTime,
// 		}
// 		c.broadcaster.BroadcastMarketTrade(ev.Instrument, out)
// 	}
// }

package marketws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type Broadcaster interface {
	Broadcast(message any)
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

	return &Consumer{
		reader:      r,
		broadcaster: broadcaster,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()

	slog.Info("gateway marketfeed consumer started")

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		// We keep the payload as map to avoid strict coupling.
		var payload map[string]any
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			slog.Error("gateway marketfeed: invalid json", "error", err, "value", string(msg.Value))
			continue
		}

		// Wrap into gateway WS message shape
		out := map[string]any{
			"type":       "trade",
			"instrument": payload["instrument"],
			"trade_id":   payload["trade_id"],
			"price":      payload["price"],
			"quantity":   payload["quantity"],
			"event_time": payload["event_time"],
			"event_type": payload["event_type"],
		}

		c.broadcaster.Broadcast(out)

		slog.Debug("gateway marketfeed broadcasted trade",
			"topic", c.reader.Config().Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)
	}
}
