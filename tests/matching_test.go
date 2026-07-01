// Package tests conține testele unitare și de integrare descrise în Capitolul 3.8
// al lucrării de licență. Fișierul acesta validează algoritmul Price-Time Priority
// (FIFO) utilizat de motorul de matching.
package tests

import (
	"sort"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tipuri minimale — oglindesc engine/internal/models fără import intern
// ---------------------------------------------------------------------------

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

type OrderStatus string

const (
	StatusNew      OrderStatus = "NEW"
	StatusOpen     OrderStatus = "OPEN"
	StatusFilled   OrderStatus = "FILLED"
	StatusPartial  OrderStatus = "PARTIAL"
	StatusCanceled OrderStatus = "CANCELED"
)

type Order struct {
	ID        int
	Side      Side
	Price     int64
	Quantity  int64
	FilledQty int64
	Status    OrderStatus
	Timestamp time.Time
}

func (o *Order) RemainingQty() int64 { return o.Quantity - o.FilledQty }

type Trade struct {
	BuyOrderID  int
	SellOrderID int
	Price       int64
	Quantity    int64
}

type MatchResult struct {
	Trades       []Trade
	FullyFilled  bool
	RemainingQty int64
}

// ---------------------------------------------------------------------------
// SimpleOrderBook — implementare Price-Time Priority (FIFO)
// ---------------------------------------------------------------------------

type SimpleOrderBook struct {
	buySide  []*Order // preț descrescător, timp crescător
	sellSide []*Order // preț crescător, timp crescător
	byID     map[int]*Order
}

func NewSimpleOrderBook() *SimpleOrderBook {
	return &SimpleOrderBook{byID: make(map[int]*Order)}
}

// AddOrder inserează un ordin de tip resting în registru.
func (ob *SimpleOrderBook) AddOrder(o *Order) {
	ob.byID[o.ID] = o
	if o.Side == Buy {
		ob.buySide = append(ob.buySide, o)
		sort.SliceStable(ob.buySide, func(i, j int) bool {
			if ob.buySide[i].Price != ob.buySide[j].Price {
				return ob.buySide[i].Price > ob.buySide[j].Price
			}
			return ob.buySide[i].Timestamp.Before(ob.buySide[j].Timestamp)
		})
	} else {
		ob.sellSide = append(ob.sellSide, o)
		sort.SliceStable(ob.sellSide, func(i, j int) bool {
			if ob.sellSide[i].Price != ob.sellSide[j].Price {
				return ob.sellSide[i].Price < ob.sellSide[j].Price
			}
			return ob.sellSide[i].Timestamp.Before(ob.sellSide[j].Timestamp)
		})
	}
}

// Match execută matching pentru ordinul incoming și returnează tranzacțiile produse.
func (ob *SimpleOrderBook) Match(incoming *Order) MatchResult {
	var trades []Trade
	remaining := incoming.Quantity

	if incoming.Side == Buy {
		var kept []*Order
		for _, resting := range ob.sellSide {
			if remaining <= 0 || resting.Price > incoming.Price {
				kept = append(kept, resting)
				continue
			}
			qty := min64(remaining, resting.RemainingQty())
			resting.FilledQty += qty
			incoming.FilledQty += qty
			remaining -= qty
			trades = append(trades, Trade{
				BuyOrderID: incoming.ID, SellOrderID: resting.ID,
				Price: resting.Price, Quantity: qty,
			})
			if resting.RemainingQty() > 0 {
				kept = append(kept, resting)
			} else {
				resting.Status = StatusFilled
				delete(ob.byID, resting.ID)
			}
		}
		ob.sellSide = kept
	} else {
		var kept []*Order
		for _, resting := range ob.buySide {
			if remaining <= 0 || resting.Price < incoming.Price {
				kept = append(kept, resting)
				continue
			}
			qty := min64(remaining, resting.RemainingQty())
			resting.FilledQty += qty
			incoming.FilledQty += qty
			remaining -= qty
			trades = append(trades, Trade{
				BuyOrderID: resting.ID, SellOrderID: incoming.ID,
				Price: resting.Price, Quantity: qty,
			})
			if resting.RemainingQty() > 0 {
				kept = append(kept, resting)
			} else {
				resting.Status = StatusFilled
				delete(ob.byID, resting.ID)
			}
		}
		ob.buySide = kept
	}

	switch {
	case remaining == 0:
		incoming.Status = StatusFilled
	case incoming.FilledQty > 0:
		incoming.Status = StatusPartial
	default:
		incoming.Status = StatusOpen
	}

	return MatchResult{Trades: trades, FullyFilled: remaining == 0, RemainingQty: remaining}
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Teste — scenariile din Secțiunea 3.8 a lucrării
// ---------------------------------------------------------------------------

// TestMatching_ExactMatch verifică potrivirea exactă: cumpărătorul și vânzătorul
// au aceeași cantitate — ambele ordine devin FILLED printr-o singură tranzacție.
func TestMatching_ExactMatch(t *testing.T) {
	ob := NewSimpleOrderBook()

	sell := &Order{ID: 1, Side: Sell, Price: 8100000, Quantity: 100,
		Status: StatusOpen, Timestamp: time.Now()}
	ob.AddOrder(sell)

	buy := &Order{ID: 2, Side: Buy, Price: 8100000, Quantity: 100,
		Status: StatusNew, Timestamp: time.Now().Add(time.Millisecond)}

	result := ob.Match(buy)

	if !result.FullyFilled {
		t.Fatal("ordinul de cumpărare trebuie să fie FILLED")
	}
	if result.RemainingQty != 0 {
		t.Fatalf("cantitate rămasă: expected 0, got %d", result.RemainingQty)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 tranzacție, got %d", len(result.Trades))
	}
	if result.Trades[0].Price != 8100000 {
		t.Errorf("prețul tranzacției: expected 8100000, got %d", result.Trades[0].Price)
	}
	if sell.Status != StatusFilled {
		t.Errorf("vânzătorul trebuie să fie FILLED, got %s", sell.Status)
	}
	if _, exists := ob.byID[sell.ID]; exists {
		t.Error("ordinul de vânzare trebuie eliminat din registru după umplere completă")
	}
}

// TestMatching_PartialFill verifică umplerea parțială: cumpărătorul vrea 3 BTC,
// vânzătorul are 1 BTC — o tranzacție de 1 BTC, cumpărătorul rămâne cu 2 BTC și
// statusul PARTIAL.
func TestMatching_PartialFill(t *testing.T) {
	ob := NewSimpleOrderBook()

	sell := &Order{ID: 1, Side: Sell, Price: 8100000, Quantity: 100,
		Status: StatusOpen, Timestamp: time.Now()}
	ob.AddOrder(sell)

	buy := &Order{ID: 2, Side: Buy, Price: 8200000, Quantity: 300,
		Status: StatusNew, Timestamp: time.Now()}

	result := ob.Match(buy)

	if result.FullyFilled {
		t.Fatal("ordinul NU trebuie să fie complet umplut")
	}
	if result.RemainingQty != 200 {
		t.Fatalf("expected remaining=200, got %d", result.RemainingQty)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 tranzacție, got %d", len(result.Trades))
	}
	if result.Trades[0].Quantity != 100 {
		t.Errorf("cantitate tranzacție: expected 100, got %d", result.Trades[0].Quantity)
	}
	if buy.Status != StatusPartial {
		t.Errorf("statusul cumpărătorului: expected PARTIAL, got %s", buy.Status)
	}
	if buy.FilledQty != 100 {
		t.Errorf("FilledQty cumpărător: expected 100, got %d", buy.FilledQty)
	}
}

// TestMatching_MultipleFills verifică umpleri multiple cu FIFO strict: cumpărătorul
// vrea 500 unități, trei vânzători la același preț cu cantități 200+200+200 —
// sunt produse exact 2 tranzacții complete și una parțială, în ordinea sosirii.
func TestMatching_MultipleFills(t *testing.T) {
	ob := NewSimpleOrderBook()
	now := time.Now()

	s1 := &Order{ID: 1, Side: Sell, Price: 8100000, Quantity: 200,
		Status: StatusOpen, Timestamp: now}
	s2 := &Order{ID: 2, Side: Sell, Price: 8100000, Quantity: 200,
		Status: StatusOpen, Timestamp: now.Add(time.Millisecond)}
	s3 := &Order{ID: 3, Side: Sell, Price: 8100000, Quantity: 200,
		Status: StatusOpen, Timestamp: now.Add(2 * time.Millisecond)}

	ob.AddOrder(s1)
	ob.AddOrder(s2)
	ob.AddOrder(s3)

	buy := &Order{ID: 4, Side: Buy, Price: 8200000, Quantity: 500,
		Status: StatusNew, Timestamp: now.Add(3 * time.Millisecond)}

	result := ob.Match(buy)

	if !result.FullyFilled {
		t.Fatal("ordinul trebuie să fie complet umplut (500 = 200+200+100)")
	}
	if len(result.Trades) != 3 {
		t.Fatalf("expected 3 tranzacții, got %d", len(result.Trades))
	}
	// FIFO: s1 complet (200), s2 complet (200), s3 parțial (100)
	if result.Trades[0].SellOrderID != s1.ID {
		t.Error("prima tranzacție trebuie să fie cu s1 (FIFO)")
	}
	if result.Trades[1].SellOrderID != s2.ID {
		t.Error("a doua tranzacție trebuie să fie cu s2 (FIFO)")
	}
	if result.Trades[2].SellOrderID != s3.ID {
		t.Error("a treia tranzacție trebuie să fie cu s3 (FIFO)")
	}
	if result.Trades[2].Quantity != 100 {
		t.Errorf("a treia tranzacție: expected qty=100, got %d", result.Trades[2].Quantity)
	}
	if result.RemainingQty != 0 {
		t.Errorf("cumpărătorul trebuie să fie complet umplut (500 = 200+200+100)")
	}
}

// TestMatching_EmptyBook verifică comportamentul la registru gol: un ordin de
// vânzare fără niciun cumpărător — zero tranzacții, statusul devine OPEN.
func TestMatching_EmptyBook(t *testing.T) {
	ob := NewSimpleOrderBook()

	sell := &Order{ID: 1, Side: Sell, Price: 8100000, Quantity: 100,
		Status: StatusNew, Timestamp: time.Now()}

	result := ob.Match(sell)

	if result.FullyFilled {
		t.Fatal("nu trebuie tranzacții pe registru gol")
	}
	if len(result.Trades) != 0 {
		t.Fatalf("expected 0 tranzacții, got %d", len(result.Trades))
	}
	if result.RemainingQty != 100 {
		t.Errorf("cantitatea rămasă: expected 100, got %d", result.RemainingQty)
	}
	if sell.Status != StatusOpen {
		t.Errorf("statusul ordinului: expected OPEN, got %s", sell.Status)
	}
}

// TestMatching_NoPriceMatch verifică lipsa potrivirii de preț: cumpărătorul la
// 80.000 USD, vânzătorul la 81.000 USD — zero tranzacții, ambele ordine rămân.
func TestMatching_NoPriceMatch(t *testing.T) {
	ob := NewSimpleOrderBook()

	sell := &Order{ID: 1, Side: Sell, Price: 8100000, Quantity: 100,
		Status: StatusOpen, Timestamp: time.Now()}
	ob.AddOrder(sell)

	buy := &Order{ID: 2, Side: Buy, Price: 8000000, Quantity: 100,
		Status: StatusNew, Timestamp: time.Now()}

	result := ob.Match(buy)

	if len(result.Trades) != 0 {
		t.Fatalf("expected 0 tranzacții (prețuri incompatibile), got %d", len(result.Trades))
	}
	if result.FullyFilled {
		t.Fatal("ordinul NU trebuie să fie filled fără potrivire")
	}
	if _, exists := ob.byID[sell.ID]; !exists {
		t.Error("ordinul de vânzare trebuie să rămână în registru")
	}
}

// TestMatching_PriceTimePriority verifică că, la prețuri diferite, cel mai bun preț
// este executat primul (priortiatea de preț > prioritatea de timp).
func TestMatching_PriceTimePriority(t *testing.T) {
	ob := NewSimpleOrderBook()
	now := time.Now()

	// s1 a sosit primul dar are prețul mai mare (mai puțin atractiv pentru cumpărător)
	s1 := &Order{ID: 1, Side: Sell, Price: 8200000, Quantity: 100,
		Status: StatusOpen, Timestamp: now}
	// s2 a sosit mai târziu dar are prețul mai mic (cel mai bun ask)
	s2 := &Order{ID: 2, Side: Sell, Price: 8100000, Quantity: 100,
		Status: StatusOpen, Timestamp: now.Add(time.Millisecond)}

	ob.AddOrder(s1)
	ob.AddOrder(s2)

	buy := &Order{ID: 3, Side: Buy, Price: 8300000, Quantity: 50,
		Status: StatusNew, Timestamp: now.Add(2 * time.Millisecond)}

	result := ob.Match(buy)

	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 tranzacție, got %d", len(result.Trades))
	}
	if result.Trades[0].SellOrderID != s2.ID {
		t.Error("priortiatea de preț: s2 (prețul mai mic) trebuie executat primul")
	}
	if result.Trades[0].Price != 8100000 {
		t.Errorf("prețul tranzacției trebuie să fie prețul vânzătorului (8100000), got %d",
			result.Trades[0].Price)
	}
	// s1 (prețul mai mare) rămâne în registru
	if _, exists := ob.byID[s1.ID]; !exists {
		t.Error("s1 trebuie să rămână în registru (preț necompetitiv)")
	}
}
