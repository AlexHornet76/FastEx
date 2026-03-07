package orderbook

import (
	"testing"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

func TestMatchOrder_FullMatch(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add resting SELL order
	sellOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Sell,
		Price:      5010000, // $50,100
		Quantity:   500,
		Status:     models.Open,
		Timestamp:  time.Now(),
	}
	ob.AddOrder(sellOrder)

	// Incoming BUY order (matches completely)
	buyOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5020000, // $50,200 (willing to pay more)
		Quantity:   500,
		Status:     models.New,
		Timestamp:  time.Now(),
	}

	result := ob.MatchOrder(buyOrder)

	// Verify result
	if !result.FullyFilled {
		t.Errorf("Order should be fully filled")
	}

	if result.RemainingQty != 0 {
		t.Errorf("Expected 0 remaining, got %d", result.RemainingQty)
	}

	if len(result.Trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(result.Trades))
	}

	// Verify trade
	trade := result.Trades[0]
	if trade.Price != 5010000 {
		t.Errorf("Trade price should be seller's price (5010000), got %d", trade.Price)
	}

	if trade.Quantity != 500 {
		t.Errorf("Trade quantity should be 500, got %d", trade.Quantity)
	}

	if trade.BuyOrderID != buyOrder.OrderID {
		t.Errorf("Trade buy order ID mismatch")
	}

	if trade.SellOrderID != sellOrder.OrderID {
		t.Errorf("Trade sell order ID mismatch")
	}

	// Verify sell order removed from book
	_, exists := ob.GetOrder(sellOrder.OrderID)
	if exists {
		t.Errorf("Filled sell order should be removed from book")
	}

	// Verify order book is empty
	if !ob.sellSide.IsEmpty() {
		t.Errorf("Sell side should be empty after full match")
	}
}

func TestMatchOrder_PartialMatch(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add resting SELL order (smaller quantity)
	sellOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Sell,
		Price:      5010000,
		Quantity:   300, // Only 300 available
		Status:     models.Open,
		Timestamp:  time.Now(),
	}
	ob.AddOrder(sellOrder)

	// Incoming BUY order (wants 500)
	buyOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5020000,
		Quantity:   500, // Wants 500
		Status:     models.New,
		Timestamp:  time.Now(),
	}

	result := ob.MatchOrder(buyOrder)

	// Verify partial fill
	if result.FullyFilled {
		t.Errorf("Order should NOT be fully filled")
	}

	if result.RemainingQty != 200 {
		t.Errorf("Expected 200 remaining, got %d", result.RemainingQty)
	}

	if len(result.Trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(result.Trades))
	}

	// Verify trade quantity
	trade := result.Trades[0]
	if trade.Quantity != 300 {
		t.Errorf("Trade quantity should be 300, got %d", trade.Quantity)
	}

	// Verify buy order filled quantity
	if buyOrder.FilledQty != 300 {
		t.Errorf("Buy order filled qty should be 300, got %d", buyOrder.FilledQty)
	}
}

func TestMatchOrder_NoMatch(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add resting SELL order at high price
	sellOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Sell,
		Price:      5050000, // $50,500
		Quantity:   500,
		Status:     models.Open,
		Timestamp:  time.Now(),
	}
	ob.AddOrder(sellOrder)

	// Incoming BUY order at lower price (can't match)
	buyOrder := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5020000, // $50,200 (too low)
		Quantity:   500,
		Status:     models.New,
		Timestamp:  time.Now(),
	}

	result := ob.MatchOrder(buyOrder)

	// Verify no match
	if result.FullyFilled {
		t.Errorf("Order should NOT be filled")
	}

	if result.RemainingQty != 500 {
		t.Errorf("Expected 500 remaining (no trades), got %d", result.RemainingQty)
	}

	if len(result.Trades) != 0 {
		t.Errorf("Expected 0 trades, got %d", len(result.Trades))
	}

	// Verify sell order still in book
	_, exists := ob.GetOrder(sellOrder.OrderID)
	if !exists {
		t.Errorf("Sell order should still be in book")
	}
}

func TestMatchOrder_MultipleMatches(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add multiple SELL orders at same price
	sell1 := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Sell,
		Price: 5010000, Quantity: 200, Status: models.Open,
		Timestamp: time.Now(), SequenceNum: 1,
	}
	sell2 := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Sell,
		Price: 5010000, Quantity: 300, Status: models.Open,
		Timestamp: time.Now().Add(1 * time.Millisecond), SequenceNum: 2,
	}

	ob.AddOrder(sell1)
	ob.AddOrder(sell2)

	// Incoming BUY order (matches both)
	buyOrder := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Buy,
		Price: 5020000, Quantity: 400, // Matches all of sell1 + part of sell2
		Status: models.New, Timestamp: time.Now(),
	}

	result := ob.MatchOrder(buyOrder)

	// Verify 2 trades
	if len(result.Trades) != 2 {
		t.Fatalf("Expected 2 trades, got %d", len(result.Trades))
	}

	// Verify first trade (FIFO: sell1 first)
	if result.Trades[0].SellOrderID != sell1.OrderID {
		t.Errorf("First trade should match sell1")
	}
	if result.Trades[0].Quantity != 200 {
		t.Errorf("First trade should be 200, got %d", result.Trades[0].Quantity)
	}

	// Verify second trade (sell2 partial)
	if result.Trades[1].SellOrderID != sell2.OrderID {
		t.Errorf("Second trade should match sell2")
	}
	if result.Trades[1].Quantity != 200 {
		t.Errorf("Second trade should be 200, got %d", result.Trades[1].Quantity)
	}

	// Verify buy order fully filled
	if !result.FullyFilled {
		t.Errorf("Buy order should be fully filled")
	}

	// Verify sell1 removed, sell2 still in book (partial)
	_, exists := ob.GetOrder(sell1.OrderID)
	if exists {
		t.Errorf("sell1 should be removed (fully filled)")
	}

	sell2InBook, exists := ob.GetOrder(sell2.OrderID)
	if !exists {
		t.Fatal("sell2 should still be in book")
	}
	if sell2InBook.FilledQty != 200 {
		t.Errorf("sell2 should have 200 filled, got %d", sell2InBook.FilledQty)
	}
	if sell2InBook.RemainingQuantity() != 100 {
		t.Errorf("sell2 should have 100 remaining, got %d", sell2InBook.RemainingQuantity())
	}
}

func TestMatchOrder_PriceTimePriority(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add SELL orders at different prices
	sell1 := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Sell,
		Price: 5020000, Quantity: 100, Status: models.Open, // Higher price
		Timestamp: time.Now(),
	}
	sell2 := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Sell,
		Price: 5010000, Quantity: 100, Status: models.Open, // Lower price (best)
		Timestamp: time.Now().Add(1 * time.Millisecond),
	}

	ob.AddOrder(sell1)
	ob.AddOrder(sell2)

	// Incoming BUY order
	buyOrder := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Buy,
		Price: 5030000, Quantity: 50, // Only matches part of best price
		Status: models.New, Timestamp: time.Now(),
	}

	result := ob.MatchOrder(buyOrder)

	// Should match sell2 (lower price) first
	if len(result.Trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(result.Trades))
	}

	if result.Trades[0].SellOrderID != sell2.OrderID {
		t.Errorf("Should match lower price (sell2) first")
	}

	if result.Trades[0].Price != 5010000 {
		t.Errorf("Trade price should be 5010000, got %d", result.Trades[0].Price)
	}

	// sell1 should still be in book
	_, exists := ob.GetOrder(sell1.OrderID)
	if !exists {
		t.Errorf("sell1 should still be in book (not matched)")
	}
}

func TestMatchOrder_SellOrderMatchesBuyBook(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add resting BUY orders
	buy1 := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Buy,
		Price: 5010000, Quantity: 200, Status: models.Open,
		Timestamp: time.Now(),
	}
	buy2 := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Buy,
		Price: 5000000, Quantity: 300, Status: models.Open,
		Timestamp: time.Now(),
	}

	ob.AddOrder(buy1)
	ob.AddOrder(buy2)

	// Incoming SELL order (matches buy1 only)
	sellOrder := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Sell,
		Price: 5000000, Quantity: 150, // Willing to sell at $50,000
		Status: models.New, Timestamp: time.Now(),
	}

	result := ob.MatchOrder(sellOrder)

	// Should match buy1 (highest price)
	if len(result.Trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(result.Trades))
	}

	if result.Trades[0].BuyOrderID != buy1.OrderID {
		t.Errorf("Should match highest buy price (buy1)")
	}

	if result.Trades[0].Price != 5010000 {
		t.Errorf("Trade should execute at buy1's price (5010000), got %d", result.Trades[0].Price)
	}

	if !result.FullyFilled {
		t.Errorf("Sell order should be fully filled")
	}
}

func TestProcessOrder_PartialMatchThenRest(t *testing.T) {
	ob := NewOrderBook("BTC-USD")

	// Add small SELL order
	sellOrder := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Sell,
		Price: 5010000, Quantity: 100, Status: models.Open,
		Timestamp: time.Now(),
	}
	ob.AddOrder(sellOrder)

	// Incoming large BUY order
	buyOrder := &models.Order{
		OrderID: uuid.New(), UserID: uuid.New(),
		Instrument: "BTC-USD", Side: models.Buy,
		Price: 5020000, Quantity: 500, // Much larger
		Status: models.New, Timestamp: time.Now(),
	}

	result, err := ob.ProcessOrder(buyOrder)
	if err != nil {
		t.Fatalf("ProcessOrder failed: %v", err)
	}

	// Should have 1 trade (100 filled)
	if len(result.Trades) != 1 {
		t.Errorf("Expected 1 trade, got %d", len(result.Trades))
	}

	// Should have 400 remaining in book
	if result.RemainingQty != 400 {
		t.Errorf("Expected 400 remaining, got %d", result.RemainingQty)
	}

	// Buy order should be in book now
	orderInBook, exists := ob.GetOrder(buyOrder.OrderID)
	if !exists {
		t.Fatal("Buy order should be in book")
	}

	if orderInBook.FilledQty != 100 {
		t.Errorf("Buy order should have 100 filled, got %d", orderInBook.FilledQty)
	}

	if orderInBook.Status != models.Partial {
		t.Errorf("Buy order status should be PARTIAL, got %s", orderInBook.Status)
	}

	// Should be best bid now
	bestBid, exists := ob.BestBid()
	if !exists || bestBid != 5020000 {
		t.Errorf("Buy order should be best bid at 5020000")
	}
}
