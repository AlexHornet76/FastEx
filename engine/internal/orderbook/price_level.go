package orderbook

import (
	"container/list"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

// PriceLevel represents all orders at a specific price
// Orders are stored in FIFO order (first in, first out)
type PriceLevel struct {
	Price    int64
	orders   *list.List
	orderMap map[uuid.UUID]*list.Element
}

// NewPriceLevel creates a new price level
func NewPriceLevel(price int64) *PriceLevel {
	return &PriceLevel{
		Price:    price,
		orders:   list.New(),
		orderMap: make(map[uuid.UUID]*list.Element),
	}
}

// AddOrder adds an order to the back of the queue (FIFO)
func (pl *PriceLevel) AddOrder(order *models.Order) {
	elem := pl.orders.PushBack(order)
	pl.orderMap[order.OrderID] = elem
}

// RemoveOrder removes an order by ID
// Returns the removed order or nil if not found
func (pl *PriceLevel) RemoveOrder(orderID uuid.UUID) *models.Order {
	elem, exists := pl.orderMap[orderID]
	if !exists {
		return nil
	}

	order := pl.orders.Remove(elem).(*models.Order)
	delete(pl.orderMap, orderID)
	return order
}

// Front returns the first order (next to be matched)
func (pl *PriceLevel) Front() *models.Order {
	if pl.orders.Len() == 0 {
		return nil
	}
	return pl.orders.Front().Value.(*models.Order)
}

// IsEmpty returns true if no orders at this price level
func (pl *PriceLevel) IsEmpty() bool {
	return pl.orders.Len() == 0
}

// TotalQuantity returns sum of remaining quantity at this level
func (pl *PriceLevel) TotalQuantity() int64 {
	var total int64
	for elem := pl.orders.Front(); elem != nil; elem = elem.Next() {
		order := elem.Value.(*models.Order)
		total += order.RemainingQuantity()
	}
	return total
}

// Len returns number of orders at this price level
func (pl *PriceLevel) Len() int {
	return pl.orders.Len()
}

// Go's doubly linked list provides:
// - PushBack(order)   → O(1) add to end
// - Remove(element)   → O(1) remove specific order
// - Front()           → O(1) get first order

// Combined with map for O(1) lookup by order ID:
// orderMap[orderID] → *list.Element → O(1) access
