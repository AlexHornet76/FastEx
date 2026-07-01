package orderbook

import (
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

// MatchResult contains the outcome of matching an order
type MatchResult struct {
	Trades       []*models.Trade // generated trades
	RemainingQty int64           // quantity not matched
	FullyFilled  bool            // whether the order was fully filled
	FilledOrders []*models.Order // resting orders that became FILLED during this match
}

// MatchOrder attempts to match an incoming order against the order book
// Returns trades and remaining quantity
func (ob *OrderBook) MatchOrder(incomingOrder *models.Order) *MatchResult {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	result := &MatchResult{
		Trades:       make([]*models.Trade, 0),
		RemainingQty: incomingOrder.Quantity,
		FullyFilled:  false,
		FilledOrders: make([]*models.Order, 0),
	}

	// Determine which side to match against
	var oppositeSide *OrderBookSide
	if incomingOrder.Side == models.Buy {
		oppositeSide = ob.sellSide
	} else {
		oppositeSide = ob.buySide
	}

	// Keep matching while there is remaining quantity
	for result.RemainingQty > 0 {

		// Get best price from opposite side
		bestPrice, exists := oppositeSide.BestPrice()
		if !exists {
			// No more orders to match against
			break
		}

		// Check if we can match at this price
		if !canMatch(incomingOrder, bestPrice) {
			// Best price is not matchable
			break
		}

		// Get price level
		priceLevel := oppositeSide.GetPriceLevel(bestPrice)
		if priceLevel == nil || priceLevel.IsEmpty() {
			break
		}

		// Match against orders at this price level
		matchedTrades, filledResting := ob.matchAtPriceLevel(incomingOrder, priceLevel, &result.RemainingQty)
		result.Trades = append(result.Trades, matchedTrades...)
		result.FilledOrders = append(result.FilledOrders, filledResting...) 

		if priceLevel.IsEmpty() {
			delete(oppositeSide.priceLevels, bestPrice)
			oppositeSide.removePrice(bestPrice)
		}
	}

	// Check if fully filled
	result.FullyFilled = (result.RemainingQty == 0)

	return result
}

// canMatch checks if incoming order can match at the given price
func canMatch(incomingOrder *models.Order, restingPrice int64) bool {
	if incomingOrder.Type == models.Market {
		return true // market orders accept any available price
	}
	if incomingOrder.Side == models.Buy {
		return incomingOrder.Price >= restingPrice
	}
	return incomingOrder.Price <= restingPrice
}

// matchAtPriceLevel matches the incoming order against orders at a specific price level
func (ob *OrderBook) matchAtPriceLevel(
	incomingOrder *models.Order,
	priceLevel *PriceLevel,
	remainingQty *int64,
) ([]*models.Trade, []*models.Order) { // return filled resting orders too

	trades := make([]*models.Trade, 0)
	filledOrders := make([]*models.Order, 0) 

	// Iterate through orders at this price level (FIFO)
	for *remainingQty > 0 && !priceLevel.IsEmpty() {
		restingOrder := priceLevel.Front()
		if restingOrder == nil {
			break
		}

		// Determine trade quantity
		tradeQty := min(*remainingQty, restingOrder.RemainingQuantity())

		// Create trade
		trade := ob.createTrade(incomingOrder, restingOrder, tradeQty, priceLevel.Price)
		trades = append(trades, trade)

		// Update quantities
		incomingOrder.FilledQty += tradeQty
		restingOrder.FilledQty += tradeQty
		*remainingQty -= tradeQty

		// Update order statuses
		if restingOrder.IsFilled() {
			restingOrder.Status = models.Filled

			// track resting orders that became filled so Engine can WAL-log them
			filledOrders = append(filledOrders, restingOrder)

			// Remove from price level
			priceLevel.RemoveOrder(restingOrder.OrderID)
			delete(ob.orders, restingOrder.OrderID)
		} else {
			restingOrder.Status = models.Partial
		}
	}

	return trades, filledOrders
}

// createTrade creates a trade record
func (ob *OrderBook) createTrade(
	incomingOrder *models.Order,
	restingOrder *models.Order,
	quantity int64,
	price int64,
) *models.Trade {
	trade := &models.Trade{
		TradeID:    uuid.New(),
		Instrument: ob.instrument,
		Price:      price,
		Quantity:   quantity,
		Timestamp:  time.Now(),
	}

	// Set buy/sell order IDs
	if incomingOrder.Side == models.Buy {
		trade.BuyOrderID = incomingOrder.OrderID
		trade.SellOrderID = restingOrder.OrderID
		trade.BuyerUserID = incomingOrder.UserID
		trade.SellerUserID = restingOrder.UserID
	} else {
		trade.BuyOrderID = restingOrder.OrderID
		trade.SellOrderID = incomingOrder.OrderID
		trade.BuyerUserID = restingOrder.UserID
		trade.SellerUserID = incomingOrder.UserID
	}

	return trade
}
