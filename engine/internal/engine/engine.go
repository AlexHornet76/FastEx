package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/kafka"
	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/AlexHornet76/FastEx/engine/internal/orderbook"
	"github.com/AlexHornet76/FastEx/engine/internal/wal"
	"github.com/google/uuid"
)

type Engine struct {
	instrument string
	orderbook  *orderbook.OrderBook
	wal        *wal.WAL
	producer   *kafka.Producer
}

func NewEngine(instrument string, walDir string, producer *kafka.Producer) (*Engine, error) {
	// Open WAL
	w, err := wal.Open(walDir, fmt.Sprintf("%s.wal", instrument))
	if err != nil {
		return nil, fmt.Errorf("open WAL: %w", err)
	}

	engine := &Engine{
		instrument: instrument,
		orderbook:  orderbook.NewOrderBook(instrument),
		wal:        w,
		producer:   producer,
	}

	// Recover from WAL if exists
	if err := engine.recoverFromWAL(); err != nil {
		return nil, fmt.Errorf("recover from WAL: %w", err)
	}
	return engine, nil
}

// recoverFromWAL rebuilds order book state from WAL
func (e *Engine) recoverFromWAL() error {
	slog.Info("starting WAL recovery", "instrument", e.instrument)

	// Track all orders by ID
	orderMap := make(map[uuid.UUID]*models.Order)

	var ordersPlaced int
	var tradesApplied int
	var ordersCancelled int

	err := e.wal.Replay(func(entry *wal.Entry) error {
		switch entry.Type {
		case wal.TypeOrderPlaced:
			data, err := entry.ParseOrderPlacedData()
			if err != nil {
				return err
			}
			if data.Order.Instrument != e.instrument {
				return nil
			}
			orderMap[data.Order.OrderID] = data.Order
			ordersPlaced++
		case wal.TypeTradeExecuted:
			data, err := entry.ParseTradeExecutedData()
			if err != nil {
				return err
			}

			if data.Trade.Instrument != e.instrument {
				return nil
			}

			//update fiiled quantities
			if buyOrder, exists := orderMap[data.Trade.BuyOrderID]; exists {
				buyOrder.FilledQty += data.Trade.Quantity
			}

			if sellOrder, exists := orderMap[data.Trade.SellOrderID]; exists {
				sellOrder.FilledQty += data.Trade.Quantity
			}
			tradesApplied++

		case wal.TypeOrderCanceled:
			data, err := entry.ParseOrderCanceledData()
			if err != nil {
				return err
			}

			if data.Instrument != e.instrument {
				return nil
			}

			delete(orderMap, data.OrderID)
			ordersCancelled++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("WAL replay failed: %w", err)
	}

	// Add orders with remaining quantity to book
	var ordersInBook int
	for _, order := range orderMap {
		if order.RemainingQuantity() > 0 {
			if err := e.orderbook.AddOrder(order); err != nil {
				slog.Warn("failed to add order during recovery",
					"order_id", order.OrderID,
					"error", err)
			} else {
				ordersInBook++
			}
		}
	}
	slog.Info("WAL recovery completed",
		"orders_placed", ordersPlaced,
		"trades_applied", tradesApplied,
		"orders_cancelled", ordersCancelled,
		"orders_in_book", ordersInBook)

	return nil

}

// // ProcessOrder processes an order (WAL → Match → WAL)
func (e *Engine) ProcessOrder(order *models.Order) (*orderbook.MatchResult, error) {
	// Step 1: WAL - Log order placement FIRST
	entry, err := wal.NewOrderPlacedEntry(0, order)
	if err != nil {
		return nil, fmt.Errorf("create WAL entry: %w", err)
	}
	if err := e.wal.Append(entry); err != nil {
		return nil, fmt.Errorf("append to WAL: %w", err)
	}
	slog.Debug("order logged to WAL",
		"order_id", order.OrderID,
		"instrument", order.Instrument,
		"seq", entry.SequenceNum)

	e.publishOrderPlaced(order)

	// Step 2: Match in-memory
	result := e.orderbook.MatchOrder(order)

	// Step 3: WAL - Log trades and cancellations
	for _, trade := range result.Trades {
		tradeEntry, err := wal.NewTradeExecutedEntry(0, trade)
		if err != nil {
			return nil, fmt.Errorf("create trade WAL entry: %w", err)
		}
		if err := e.wal.Append(tradeEntry); err != nil {
			return nil, fmt.Errorf("append trade to WAL: %w", err)
		}
		slog.Debug("trade logged to WAL",
			"trade_id", trade.TradeID,
			"qty", trade.Quantity,
			"price", trade.Price,
			"seq", tradeEntry.SequenceNum)
		e.publishTradeExecuted(trade)
	}

	// Step 4: Add remaining to book (if any)
	if !result.FullyFilled && result.RemainingQty > 0 {
		order.Status = models.Open
		if order.FilledQty > 0 {
			order.Status = models.Partial
		}
		if err := e.orderbook.AddOrder(order); err != nil {
			return nil, fmt.Errorf("add order to book: %w", err)
		}
	} else {
		order.Status = models.Filled
	}

	return result, nil
}

// CancelOrder cancels an order (WAL → Remove)
func (e *Engine) CancelOrder(orderID uuid.UUID, price int64) error {
	// Step 1: WAL - Log cancellation FIRST
	entry, err := wal.NewOrderCanceledEntry(0, orderID, e.instrument, price)
	if err != nil {
		return fmt.Errorf("create WAL entry: %w", err)
	}

	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("append to WAL: %w", err)
	}

	// Step 2: Remove from order book
	_, err = e.orderbook.RemoveOrder(orderID)
	if err != nil {
		return fmt.Errorf("remove order: %w", err)
	}

	e.publishOrderCanceled(orderID, price)
	slog.Info("order canceled", "order_id", orderID)
	return nil
}

func (e *Engine) publishOrderPlaced(order *models.Order) {
	if e.producer == nil {
		return
	}

	ev := kafka.OrderPlacedEvent{
		EventType:  kafka.TopicOrderPlaced,
		EventTime:  time.Now().UTC(),
		Instrument: order.Instrument,
		OrderID:    order.OrderID,
		UserID:     order.UserID,
		Side:       string(order.Side),
		Type:       string(order.Type),
		Price:      order.Price,
		Quantity:   order.Quantity,
		FilledQty:  order.FilledQty,
		Status:     string(order.Status),
	}

	if err := e.producer.PublishJSON(context.Background(), kafka.TopicOrderPlaced, order.Instrument, ev); err != nil {
		slog.Warn("kafka publish failed", "topic", kafka.TopicOrderPlaced, "instrument", order.Instrument, "error", err)
	}
}

func (e *Engine) publishTradeExecuted(trade *models.Trade) {
	if e.producer == nil {
		return
	}

	ev := kafka.TradeExecutedEvent{
		EventType:    kafka.TopicTradeExecuted,
		EventTime:    time.Now().UTC(),
		Instrument:   trade.Instrument,
		TradeID:      trade.TradeID,
		BuyOrderID:   trade.BuyOrderID,
		SellOrderID:  trade.SellOrderID,
		BuyerUserID:  trade.BuyerUserID,
		SellerUserID: trade.SellerUserID,
		Price:        trade.Price,
		Quantity:     trade.Quantity,
	}

	if err := e.producer.PublishJSON(context.Background(), kafka.TopicTradeExecuted, trade.Instrument, ev); err != nil {
		slog.Warn("kafka publish failed", "topic", kafka.TopicTradeExecuted, "instrument", trade.Instrument, "error", err)
	}
}

func (e *Engine) publishOrderCanceled(orderID uuid.UUID, price int64) {
	if e.producer == nil {
		return
	}

	ev := kafka.OrderCanceledEvent{
		EventType:  kafka.TopicOrderCanceled,
		EventTime:  time.Now().UTC(),
		Instrument: e.instrument,
		OrderID:    orderID,
		Price:      price,
	}

	if err := e.producer.PublishJSON(context.Background(), kafka.TopicOrderCanceled, e.instrument, ev); err != nil {
		slog.Warn("kafka publish failed", "topic", kafka.TopicOrderCanceled, "instrument", e.instrument, "error", err)
	}
}

// GetOrderBook returns the order book (for queries)
func (e *Engine) GetOrderBook() *orderbook.OrderBook {
	return e.orderbook
}

// Close closes the WAL
func (e *Engine) Close() error {
	return e.wal.Close()
}
