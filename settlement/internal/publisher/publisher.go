package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexHornet76/FastEx/settlement/internal/events"
	"github.com/segmentio/kafka-go"
)

type TradePublisher struct {
	writer *kafka.Writer
	topic  string
}

func NewTradePublisher(brokers []string, topic string) *TradePublisher {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		BatchTimeout: 50 * time.Millisecond,
	}
	return &TradePublisher{writer: w, topic: topic}
}

func (p *TradePublisher) Close() error {
	return p.writer.Close()
}

// PublishSettled publishes a settled trade event.
// Key = instrument to preserve ordering per instrument like engine writer.
func (p *TradePublisher) PublishSettled(ctx context.Context, ev *events.TradeExecutedEvent) error {
	// clone payload, change event_type to trade.settled.v1
	out := *ev
	out.EventType = "trade.settled"
	if out.EventTime.IsZero() {
		out.EventTime = time.Now().UTC()
	}

	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal trade.settled: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(out.Instrument),
		Value: b,
		Time:  time.Now().UTC(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write kafka trade.settled topic=%s: %w", p.topic, err)
	}

	slog.Info("published trade.settled",
		"topic", p.topic,
		"instrument", out.Instrument,
		"trade_id", out.TradeID,
		"correlation_id", out.CorrelationID,
	)
	return nil
}
