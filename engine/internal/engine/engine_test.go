package engine

import (
	"testing"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

func TestEngine_ProcessOrder(t *testing.T) {
	tmpDir := t.TempDir()

	engine, err := NewEngine("BTC-USD", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Place sell order
	sellOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Sell,
		Price:      5010000,
		Quantity:   500,
		Timestamp:  time.Now(),
	}

	result, err := engine.ProcessOrder(sellOrder)
	if err != nil {
		t.Fatalf("Failed to process sell order: %v", err)
	}

	if len(result.Trades) != 0 {
		t.Errorf("Sell order should not match (empty book)")
	}

	// Place buy order (matches)
	buyOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5020000,
		Quantity:   500,
		Timestamp:  time.Now(),
	}

	result, err = engine.ProcessOrder(buyOrder)
	if err != nil {
		t.Fatalf("Failed to process buy order: %v", err)
	}

	// Should match
	if len(result.Trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(result.Trades))
	}

	trade := result.Trades[0]
	if trade.Quantity != 500 {
		t.Errorf("Trade quantity should be 500, got %d", trade.Quantity)
	}
}

func TestEngine_Recovery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create engine and place orders
	{
		engine, _ := NewEngine("BTC-USD", tmpDir)

		order := &models.Order{
			OrderID:    uuid.New(),
			UserID:     uuid.New(),
			Instrument: "BTC-USD",
			Side:       models.Buy,
			Price:      5000000,
			Quantity:   1000,
			Timestamp:  time.Now(),
		}

		engine.ProcessOrder(order)
		engine.Close()
	}

	// Restart and verify recovery
	{
		engine, err := NewEngine("BTC-USD", tmpDir)
		if err != nil {
			t.Fatalf("Failed to recover: %v", err)
		}
		defer engine.Close()

		// Check order book has the order
		bestBid, exists := engine.GetOrderBook().BestBid()
		if !exists {
			t.Fatal("Order should be in book after recovery")
		}

		if bestBid != 5000000 {
			t.Errorf("Best bid should be 5000000, got %d", bestBid)
		}
	}
}

func TestEngine_CancelOrder(t *testing.T) {
	tmpDir := t.TempDir()
	engine, _ := NewEngine("BTC-USD", tmpDir)
	defer engine.Close()

	order := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5000000,
		Quantity:   1000,
		Timestamp:  time.Now(),
	}

	engine.ProcessOrder(order)

	// Cancel order
	err := engine.CancelOrder(order.OrderID, order.Price)
	if err != nil {
		t.Fatalf("Failed to cancel order: %v", err)
	}

	// Order should be removed
	_, exists := engine.GetOrderBook().GetOrder(order.OrderID)
	if exists {
		t.Errorf("Canceled order should not be in book")
	}
}
