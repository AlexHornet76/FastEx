package orderws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type UserSender interface {
	SendToUser(userID string, message any)
}

type Consumer struct {
	placed   *kafka.Reader
	canceled *kafka.Reader
	filled   *kafka.Reader
	sender   UserSender
}

func NewConsumer(brokers []string, topicPlaced, topicCanceled, topicFilled, groupID string, sender UserSender) *Consumer {
	newR := func(topic string) *kafka.Reader {
		return kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 1,
			MaxBytes: 10e6,
			MaxWait:  500 * time.Millisecond,
		})
	}

	return &Consumer{
		placed:   newR(topicPlaced),
		canceled: newR(topicCanceled),
		filled:   newR(topicFilled),
		sender:   sender,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.placed.Close()
	defer c.canceled.Close()
	defer c.filled.Close()

	errCh := make(chan error, 3)

	go func() { errCh <- c.runOne(ctx, c.placed, "PLACED") }()
	go func() { errCh <- c.runOne(ctx, c.canceled, "CANCELED") }()
	go func() { errCh <- c.runOne(ctx, c.filled, "FILLED") }()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Consumer) runOne(ctx context.Context, r *kafka.Reader, status string) error {
	slog.Info("orderws consumer started", "topic", r.Config().Topic, "status", status)

	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var ev map[string]any
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			slog.Error("orderws: invalid json", "topic", r.Config().Topic, "error", err, "value", string(msg.Value))
			continue
		}

		userID, _ := ev["user_id"].(string)
		if userID == "" {
			userID, _ = ev["userId"].(string)
		}
		if userID == "" {
			slog.Warn("orderws: missing user_id", "topic", r.Config().Topic, "value", string(msg.Value))
			continue
		}

		orderID, _ := ev["order_id"].(string)
		if orderID == "" {
			orderID, _ = ev["orderId"].(string)
		}

		out := map[string]any{
			"type":   "order_update",
			"status": status,

			"order_id": orderID,

			"instrument":     ev["instrument"],
			"side":           ev["side"],
			"price":          ev["price"],
			"quantity":       ev["quantity"],
			"filled_qty":     ev["filled_qty"],
			"event_time":     ev["event_time"],
			"event_type":     ev["event_type"],
			"correlation_id": ev["correlation_id"],
		}

		c.sender.SendToUser(userID, out)
	}
}
