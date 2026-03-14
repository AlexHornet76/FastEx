package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	brokers []string
	writers map[string]*kafka.Writer
}

func NewProducer(brokers []string) (*Producer, error) {
	if len(brokers) == 0 {
		return nil, nil // No Kafka configured, return nil producer
	}
	p := &Producer{
		brokers: brokers,
		writers: map[string]*kafka.Writer{},
	}
	p.writers[TopicOrderPlaced] = newWriter(brokers, TopicOrderPlaced)
	p.writers[TopicTradeExecuted] = newWriter(brokers, TopicTradeExecuted)
	p.writers[TopicOrderCanceled] = newWriter(brokers, TopicOrderCanceled)
	return p, nil
}

func newWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{}, // key-hash => instrument key keeps ordering per instrument
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		BatchTimeout: 10 * time.Millisecond,
	}
}

func (p *Producer) Close() error {
	var firstErr error
	for _, w := range p.writers {
		if err := w.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *Producer) PublishJSON(ctx context.Context, topic string, key string, payload any) error {
	writer, ok := p.writers[topic]
	if !ok {
		return fmt.Errorf("unknown topic %q", topic)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal kafka payload: %w", err)
	}
	// avoid hanging if broker is slow/unreachable
	publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	msg := kafka.Message{
		Key:   []byte(key), // instrument
		Value: b,
		Time:  time.Now(),
	}
	if err := writer.WriteMessages(publishCtx, msg); err != nil {
		return fmt.Errorf("kafka write (%s): %w", topic, err)
	}
	return nil
}
