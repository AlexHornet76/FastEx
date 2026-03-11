package orderbook

import (
	"sort"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

// OrderBookSide represents one side of the order book (buy or sell)
type OrderBookSide struct {
	side        models.OrderSide
	priceLevels map[int64]*PriceLevel // price → PriceLevel
	prices      []int64               // Sorted list of prices
	isSorted    bool                  // Track if prices need re-sorting
}

// NewOrderBookSide creates a new order book side
func NewOrderBookSide(side models.OrderSide) *OrderBookSide {
	return &OrderBookSide{
		side:        side,
		priceLevels: make(map[int64]*PriceLevel),
		prices:      make([]int64, 0),
		isSorted:    true,
	}
}

type PriceLevelInfo struct {
	Price      int64 `json:"price"`
	Quantity   int64 `json:"quantity"`
	OrderCount int   `json:"order_count"`
}

// AddOrder adds an order to the appropriate price level
func (obs *OrderBookSide) AddOrder(order *models.Order) {

	price := order.Price

	// Get or create price level
	priceLevel, exists := obs.priceLevels[price]
	if !exists {
		priceLevel = NewPriceLevel(price)
		obs.priceLevels[price] = priceLevel
		obs.prices = append(obs.prices, price)
		obs.isSorted = false
	}

	priceLevel.AddOrder(order)
}

// RemoveOrder removes an order from its price level
func (obs *OrderBookSide) RemoveOrder(orderID uuid.UUID, price int64) *models.Order {

	priceLevel, exists := obs.priceLevels[price]
	if !exists {
		return nil
	}

	order := priceLevel.RemoveOrder(orderID)

	// Clean up empty price level
	if priceLevel.IsEmpty() {
		delete(obs.priceLevels, price)
		obs.removePrice(price)
	}

	return order
}

// removePrice removes a price from the sorted list
func (obs *OrderBookSide) removePrice(price int64) {
	for i, p := range obs.prices {
		if p == price {
			obs.prices = append(obs.prices[:i], obs.prices[i+1:]...)
			break
		}
	}
}

// BestPrice returns the best price (highest for buy, lowest for sell)
func (obs *OrderBookSide) BestPrice() (int64, bool) {
	if len(obs.prices) == 0 {
		return 0, false
	}

	obs.ensureSorted()

	if obs.side == models.Buy {
		// For buy side, best price is highest
		return obs.prices[len(obs.prices)-1], true
	}

	// For sell side, best price is lowest
	return obs.prices[0], true
}

// GetPriceLevel returns the price level at a given price
func (obs *OrderBookSide) GetPriceLevel(price int64) *PriceLevel {

	return obs.priceLevels[price]
}

// ensureSorted sorts prices if needed
func (obs *OrderBookSide) ensureSorted() {

	if obs.isSorted {
		return
	}

	sort.Slice(obs.prices, func(i, j int) bool {
		return obs.prices[i] < obs.prices[j]
	})

	obs.isSorted = true
}

// IsEmpty returns true if no orders on this side
func (obs *OrderBookSide) IsEmpty() bool {

	return len(obs.priceLevels) == 0
}

// Depth returns the number of price levels
func (obs *OrderBookSide) Depth() int {
	return len(obs.priceLevels)
}

// TotalQuantity returns sum of all quantities on this side
func (obs *OrderBookSide) TotalQuantity() int64 {

	var total int64
	for _, pl := range obs.priceLevels {
		total += pl.TotalQuantity()
	}
	return total
}

// GetTopPriceLevels returns top N price levels with aggregated info
func (obs *OrderBookSide) GetTopPriceLevels(limit int) []PriceLevelInfo {

	obs.ensureSorted()

	result := make([]PriceLevelInfo, 0, limit)
	count := 0

	// Get prices in correct order
	var prices []int64
	if obs.side == models.Buy {
		// Buy side: highest first (reverse iteration)
		for i := len(obs.prices) - 1; i >= 0 && count < limit; i-- {
			prices = append(prices, obs.prices[i])
		}
	} else {
		// Sell side: lowest first (forward iteration)
		for i := 0; i < len(obs.prices) && count < limit; i++ {
			prices = append(prices, obs.prices[i])
		}
	}

	// Build price level info
	for _, price := range prices {
		priceLevel := obs.priceLevels[price]
		if priceLevel != nil && !priceLevel.IsEmpty() {
			result = append(result, PriceLevelInfo{
				Price:      price,
				Quantity:   priceLevel.TotalQuantity(),
				OrderCount: priceLevel.Len(),
			})
			count++
		}
	}

	return result
}
