package orderbook

import (
	"fmt"
	"sync"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

// OrderBook represents a full order book for one instrument
type OrderBook struct {
	instrument string
	buySide    *OrderBookSide
	sellSide   *OrderBookSide
	orders     map[uuid.UUID]*models.Order // All orders by ID
	mu         sync.RWMutex                // Protect concurrent access
}

// NewOrderBook creates a new order book for an instrument
func NewOrderBook(instrument string) *OrderBook {
	return &OrderBook{
		instrument: instrument,
		buySide:    NewOrderBookSide(models.Buy),
		sellSide:   NewOrderBookSide(models.Sell),
		orders:     make(map[uuid.UUID]*models.Order),
	}
}

// AddOrder adds an order to the book
func (ob *OrderBook) AddOrder(order *models.Order) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// Validate order
	if order.Instrument != ob.instrument {
		return fmt.Errorf("order instrument %s does not match book instrument %s",
			order.Instrument, ob.instrument)
	}

	// Store order
	ob.orders[order.OrderID] = order

	// Add to appropriate side
	if order.Side == models.Buy {
		ob.buySide.AddOrder(order)
	} else {
		ob.sellSide.AddOrder(order)
	}

	return nil
}

// RemoveOrder removes an order from the book
func (ob *OrderBook) RemoveOrder(orderID uuid.UUID) (*models.Order, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	order, exists := ob.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found", orderID)
	}

	// Remove from side
	var removed *models.Order
	if order.Side == models.Buy {
		removed = ob.buySide.RemoveOrder(orderID, order.Price)
	} else {
		removed = ob.sellSide.RemoveOrder(orderID, order.Price)
	}

	if removed != nil {
		delete(ob.orders, orderID)
	}

	return removed, nil
}

// GetOrder retrieves an order by ID
func (ob *OrderBook) GetOrder(orderID uuid.UUID) (*models.Order, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	order, exists := ob.orders[orderID]
	return order, exists
}

// BestBid returns the highest buy price
func (ob *OrderBook) BestBid() (int64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.buySide.BestPrice()
}

// BestAsk returns the lowest sell price
func (ob *OrderBook) BestAsk() (int64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.sellSide.BestPrice()
}

// Spread returns the bid-ask spread
func (ob *OrderBook) Spread() (int64, bool) {
	bid, bidExists := ob.BestBid()
	ask, askExists := ob.BestAsk()

	if !bidExists || !askExists {
		return 0, false
	}

	return ask - bid, true
}

// Depth returns total depth (buy + sell price levels)
func (ob *OrderBook) Depth() (int, int) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.buySide.Depth(), ob.sellSide.Depth()
}

// IsEmpty returns true if no orders in the book
func (ob *OrderBook) IsEmpty() bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.buySide.IsEmpty() && ob.sellSide.IsEmpty()
}

// ProcessOrder matches an order and adds any remaining quantity to the book
// Returns the match result
func (ob *OrderBook) ProcessOrder(order *models.Order) (*MatchResult, error) {
	if order.Instrument != ob.instrument {
		return nil, fmt.Errorf("instrument mismatch")
	}

	result := ob.MatchOrder(order)

	// If not fully filled, add remaining to book
	if !result.FullyFilled && result.RemainingQty > 0 {
		order.Status = models.Open
		if order.FilledQty > 0 {
			order.Status = models.Partial
		}

		if err := ob.AddOrder(order); err != nil {
			return result, fmt.Errorf("failed to add order to book: %w", err)
		}
	} else {
		order.Status = models.Filled
	}

	return result, nil
}

// GetTopBids returns top N bid levels
func (ob *OrderBook) GetTopBids(limit int) []PriceLevelInfo {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.buySide.GetTopPriceLevels(limit)
}

// GetTopAsks returns top N ask levels
func (ob *OrderBook) GetTopAsks(limit int) []PriceLevelInfo {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.sellSide.GetTopPriceLevels(limit)
}
