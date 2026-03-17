package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/wal"
)

type WALPublisher struct {
	producer     *Producer
	walDir       string
	instrument   string
	pollInterval time.Duration
}

func NewWALPublisher(producer *Producer, walDir, instrument string) *WALPublisher {
	return &WALPublisher{
		producer:     producer,
		walDir:       walDir,
		instrument:   instrument,
		pollInterval: 500 * time.Millisecond,
	}
}

// Run continuously replays WAL entries after last cursor and publishes them to Kafka.
// It is safe to run in a goroutine.
func (p *WALPublisher) Run(ctx context.Context) {
	if p.producer == nil {
		return
	}
	walPath := filepath.Join(p.walDir, fmt.Sprintf("%s.wal", p.instrument))
	lastSeq, err := LoadCursor(p.walDir, p.instrument)
	if err != nil {
		slog.Error("wal publisher failed to load cursor", "instrument", p.instrument, "error", err)
		return
	}
	slog.Info("wal publisher started", "instrument", p.instrument, "wal", walPath, "cursor", lastSeq)

	t := time.NewTicker(p.pollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("wal publisher stopped", "instrument", p.instrument)
			return
		case <-t.C:
			newSeq, err := p.PublishOnce(ctx, walPath, lastSeq)
			if err != nil {
				// Do not advance cursor. Retry later.
				slog.Warn("wal publisher publishOnce failed", "instrument", p.instrument, "error", err)
				continue
			}
			lastSeq = newSeq
		}
	}
}

func (p *WALPublisher) PublishOnce(ctx context.Context, walPath string, lastSeq uint64) (uint64, error) {
	maxSeq := lastSeq

	err := wal.ReadEntriesFromFile(walPath, lastSeq, func(e *wal.Entry) error {
		// convert WAL entry -> Kafka event
		switch e.Type {
		case wal.TypeOrderPlaced:
			var data wal.OrderPlacedData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return err
			}
			if data.Order == nil {
				return nil
			}
			if data.Order.Instrument != p.instrument {
				return nil
			}

			ev := OrderPlacedEvent{
				EventType:  TopicOrderPlaced,
				EventTime:  e.Timestamp.UTC(),
				Instrument: data.Order.Instrument,
				OrderID:    data.Order.OrderID,
				UserID:     data.Order.UserID,
				Side:       string(data.Order.Side),
				Type:       string(data.Order.Type),
				Price:      data.Order.Price,
				Quantity:   data.Order.Quantity,
				FilledQty:  data.Order.FilledQty,
				Status:     string(data.Order.Status),
			}

			if err := p.producer.PublishJSON(ctx, TopicOrderPlaced, p.instrument, ev); err != nil {
				return err
			}

		case wal.TypeTradeExecuted:
			var data wal.TradeExecutedData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return err
			}
			if data.Trade == nil {
				return nil
			}
			if data.Trade.Instrument != p.instrument {
				return nil
			}

			ev := TradeExecutedEvent{
				EventType:    TopicTradeExecuted,
				EventTime:    e.Timestamp.UTC(),
				Instrument:   data.Trade.Instrument,
				TradeID:      data.Trade.TradeID,
				BuyOrderID:   data.Trade.BuyOrderID,
				SellOrderID:  data.Trade.SellOrderID,
				BuyerUserID:  data.Trade.BuyerUserID,
				SellerUserID: data.Trade.SellerUserID,
				Price:        data.Trade.Price,
				Quantity:     data.Trade.Quantity,
			}

			if err := p.producer.PublishJSON(ctx, TopicTradeExecuted, p.instrument, ev); err != nil {
				return err
			}

		case wal.TypeOrderCanceled:
			var data wal.OrderCanceledData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return err
			}
			if data.Instrument != p.instrument {
				return nil
			}

			ev := OrderCanceledEvent{
				EventType:  TopicOrderCanceled,
				EventTime:  e.Timestamp.UTC(),
				Instrument: data.Instrument,
				OrderID:    data.OrderID,
				Price:      data.Price,
			}

			if err := p.producer.PublishJSON(ctx, TopicOrderCanceled, p.instrument, ev); err != nil {
				return err
			}
		default:
			// ignore unknown types
		}

		// update cursor after each successful publish
		if err := SaveCursor(p.walDir, p.instrument, e.SequenceNum); err != nil {
			return err
		}

		if e.SequenceNum > maxSeq {
			maxSeq = e.SequenceNum
		}
		return nil
	})

	if err != nil {
		return lastSeq, err
	}

	return maxSeq, nil
}
