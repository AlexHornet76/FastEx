package orderbook

import (
	"testing"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

func TestOrderBookAddAndRetrieve(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	order := &models.Order{
		OrderID:    uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5000000,
		Quantity:   100,
	}

	err := ob.AddOrder(order)
	if err != nil {
		t.Fatalf("Failed to add order: %v", err)
	}

	retrieved, exists := ob.GetOrder(order.OrderID)
	if !exists {
		t.Fatal("Order should exist")
	}

	if retrieved.OrderID != order.OrderID {
		t.Errorf("Retrieved wrong order")
	}
}

func TestOrderBookBestPrices(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add buy orders
	ob.AddOrder(&models.Order{
		OrderID: uuid.New(), Instrument: "BTC-USD",
		Side: models.Buy, Price: 5000000, Quantity: 100,
	})
	ob.AddOrder(&models.Order{
		OrderID: uuid.New(), Instrument: "BTC-USD",
		Side: models.Buy, Price: 4990000, Quantity: 200,
	})

	// Add sell orders
	ob.AddOrder(&models.Order{
		OrderID: uuid.New(), Instrument: "BTC-USD",
		Side: models.Sell, Price: 5010000, Quantity: 150,
	})
	ob.AddOrder(&models.Order{
		OrderID: uuid.New(), Instrument: "BTC-USD",
		Side: models.Sell, Price: 5020000, Quantity: 250,
	})

	// Check best bid
	bestBid, exists := ob.BestBid()
	if !exists || bestBid != 5000000 {
		t.Errorf("Best bid should be 5000000, got %d", bestBid)
	}

	// Check best ask
	bestAsk, exists := ob.BestAsk()
	if !exists || bestAsk != 5010000 {
		t.Errorf("Best ask should be 5010000, got %d", bestAsk)
	}

	// Check spread
	spread, exists := ob.Spread()
	if !exists || spread != 10000 {
		t.Errorf("Spread should be 10000, got %d", spread)
	}
}

func TestOrderBookRemove(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	order := &models.Order{
		OrderID: uuid.New(), Instrument: "BTC-USD",
		Side: models.Buy, Price: 5000000, Quantity: 100,
	}

	ob.AddOrder(order)

	removed, err := ob.RemoveOrder(order.OrderID)
	if err != nil {
		t.Fatalf("Failed to remove order: %v", err)
	}

	if removed.OrderID != order.OrderID {
		t.Errorf("Removed wrong order")
	}

	_, exists := ob.GetOrder(order.OrderID)
	if exists {
		t.Errorf("Order should not exist after removal")
	}

	if !ob.IsEmpty() {
		t.Errorf("Order book should be empty")
	}
}

func TestOrderBookInstrumentValidation(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	order := &models.Order{
		OrderID:    uuid.New(),
		Instrument: "ETH-USD", // Wrong instrument!
		Side:       models.Buy,
		Price:      5000000,
		Quantity:   100,
	}

	err := ob.AddOrder(order)
	if err == nil {
		t.Errorf("Should reject order with wrong instrument")
	}
}

func TestOrderBookDepth(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add 3 buy price levels
	ob.AddOrder(&models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Side: models.Buy, Price: 5000000, Quantity: 100})
	ob.AddOrder(&models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Side: models.Buy, Price: 4990000, Quantity: 200})
	ob.AddOrder(&models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Side: models.Buy, Price: 4980000, Quantity: 150})

	// Add 2 sell price levels
	ob.AddOrder(&models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Side: models.Sell, Price: 5010000, Quantity: 100})
	ob.AddOrder(&models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Side: models.Sell, Price: 5020000, Quantity: 200})

	buyDepth, sellDepth := ob.Depth()
	if buyDepth != 3 {
		t.Errorf("Expected buy depth 3, got %d", buyDepth)
	}
	if sellDepth != 2 {
		t.Errorf("Expected sell depth 2, got %d", sellDepth)
	}
}
