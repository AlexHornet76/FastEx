// Fișierul testează idempotența mecanismului de settlement, scenariul descris în
// Secțiunea 3.8: același eveniment trade.executed sosește de două ori (livrare
// at-least-once din Kafka) — a doua procesare trebuie să fie un no-op complet.
package tests

import (
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Settler în memorie — logică identică cu settlement/internal/settle/settler.go,
// fără dependență de PostgreSQL (adecvat pentru test unitar izolat)
// ---------------------------------------------------------------------------

type Balance struct {
	Amount int64
}

type LedgerEntry struct {
	TradeID string
	UserID  string
	Asset   string
	Amount  int64
}

// InMemorySettler replică logica din Settler.ApplyTrade folosind structuri în memorie.
type InMemorySettler struct {
	mu              sync.Mutex
	processedTrades map[string]bool           // SET processed_trades
	balances        map[string]map[string]int64 // user -> asset -> amount
	ledger          []LedgerEntry             // jurnal contabil
}

func NewInMemorySettler() *InMemorySettler {
	return &InMemorySettler{
		processedTrades: make(map[string]bool),
		balances:        make(map[string]map[string]int64),
	}
}

func (s *InMemorySettler) getBalance(userID, asset string) int64 {
	if s.balances[userID] == nil {
		return 0
	}
	return s.balances[userID][asset]
}

func (s *InMemorySettler) addBalance(userID, asset string, amount int64) {
	if s.balances[userID] == nil {
		s.balances[userID] = make(map[string]int64)
	}
	s.balances[userID][asset] += amount
}

type TradeEvent struct {
	TradeID      string
	BuyerUserID  string
	SellerUserID string
	Instrument   string // ex. "BTC"
	Quantity     int64  // cantitate instrument (în cenți de BTC: 100 = 1 BTC)
	Price        int64  // preț per unitate în cenți USD
}

// ApplyTrade procesează un trade: verifică idempotența, aplică cele 4 înregistrări
// contabile și marchează trade-ul ca procesat — totul atomic (sub mutex, analog
// tranzacției PostgreSQL din implementarea reală).
//
// Returnează (true, nil) la prima procesare și (false, nil) dacă trade-ul există deja.
func (s *InMemorySettler) ApplyTrade(ev TradeEvent) (applied bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Verificare idempotență — SELECT EXISTS FROM processed_trades
	if s.processedTrades[ev.TradeID] {
		return false, nil // eveniment duplicat — skip
	}

	usdValue := ev.Price * ev.Quantity / 100

	// 2. Patru înregistrări contabile (double-entry bookkeeping)
	entries := []LedgerEntry{
		{ev.TradeID, ev.BuyerUserID, ev.Instrument, +ev.Quantity}, // cumpărător +instrument
		{ev.TradeID, ev.SellerUserID, ev.Instrument, -ev.Quantity}, // vânzător   -instrument
		{ev.TradeID, ev.BuyerUserID, "USD", -usdValue},             // cumpărător -USD
		{ev.TradeID, ev.SellerUserID, "USD", +usdValue},            // vânzător   +USD
	}

	// 3. Verificare zero-sum (suma algebrică = 0, altfel rollback)
	var sumInstrument, sumUSD int64
	for _, e := range entries {
		if e.Asset == ev.Instrument {
			sumInstrument += e.Amount
		} else {
			sumUSD += e.Amount
		}
	}
	if sumInstrument != 0 || sumUSD != 0 {
		// Echivalentul rollback din PostgreSQL
		return false, &zeroSumError{instrument: sumInstrument, usd: sumUSD}
	}

	// 4. Aplicare solduri + înregistrare în jurnal
	for _, e := range entries {
		s.ledger = append(s.ledger, e)
		s.addBalance(e.UserID, e.Asset, e.Amount)
	}

	// 5. Marcare ca procesat — INSERT INTO processed_trades
	s.processedTrades[ev.TradeID] = true

	return true, nil
}

type zeroSumError struct{ instrument, usd int64 }

func (e *zeroSumError) Error() string {
	return "zero-sum violated"
}

// ---------------------------------------------------------------------------
// Teste idempotență — scenariul din Secțiunea 3.8
// ---------------------------------------------------------------------------

// TestIdempotency_DuplicateEvent este testul central: același eveniment trade.executed
// este trimis de două ori (simulând livrarea at-least-once din Kafka).
// Prima procesare actualizează soldurile; a doua trebuie să fie un no-op complet.
func TestIdempotency_DuplicateEvent(t *testing.T) {
	s := NewInMemorySettler()

	// Sold inițial: cumpărătorul are 10.000 USD, vânzătorul are 1 BTC (100 unități).
	s.addBalance("buyer", "USD", 1_000_000)  // 10.000 USD (în cenți)
	s.addBalance("seller", "BTC", 100)        // 1 BTC (100 unități)

	ev := TradeEvent{
		TradeID:      "trade-abc123",
		BuyerUserID:  "buyer",
		SellerUserID: "seller",
		Instrument:   "BTC",
		Quantity:     50,      // 0.5 BTC
		Price:        8100_00, // 8100 USD/BTC (în cenți)
	}

	// --- Prima procesare ---
	applied, err := s.ApplyTrade(ev)
	if err != nil {
		t.Fatalf("prima procesare a eșuat: %v", err)
	}
	if !applied {
		t.Fatal("prima procesare trebuie să returneze applied=true")
	}

	balBuyerBTC := s.getBalance("buyer", "BTC")
	balBuyerUSD := s.getBalance("buyer", "USD")
	balSellerBTC := s.getBalance("seller", "BTC")
	balSellerUSD := s.getBalance("seller", "USD")

	// --- A doua procesare (eveniment duplicat din Kafka) ---
	applied2, err2 := s.ApplyTrade(ev)
	if err2 != nil {
		t.Fatalf("a doua procesare a returnat eroare neașteptată: %v", err2)
	}
	if applied2 {
		t.Fatal("a doua procesare trebuie să returneze applied=false (duplicat detectat)")
	}

	// Soldurile NU trebuie să se schimbe după al doilea apel
	if s.getBalance("buyer", "BTC") != balBuyerBTC {
		t.Errorf("soldul BTC al cumpărătorului s-a schimbat după procesare duplicat")
	}
	if s.getBalance("buyer", "USD") != balBuyerUSD {
		t.Errorf("soldul USD al cumpărătorului s-a schimbat după procesare duplicat")
	}
	if s.getBalance("seller", "BTC") != balSellerBTC {
		t.Errorf("soldul BTC al vânzătorului s-a schimbat după procesare duplicat")
	}
	if s.getBalance("seller", "USD") != balSellerUSD {
		t.Errorf("soldul USD al vânzătorului s-a schimbat după procesare duplicat")
	}

	// Jurnalul contabil trebuie să conțină exact 4 înregistrări (nu 8)
	if len(s.ledger) != 4 {
		t.Errorf("jurnalul contabil: expected 4 intrări (o singură tranzacție), got %d", len(s.ledger))
	}
}

// TestIdempotency_CorrectBalances verifică că soldurile finale după o tranzacție
// sunt corecte: cumpărătorul primește instrumentul, vânzătorul primește USD.
func TestIdempotency_CorrectBalances(t *testing.T) {
	s := NewInMemorySettler()

	s.addBalance("alice", "USD", 1_000_000) // Alice: 10.000 USD
	s.addBalance("bob", "BTC", 200)         // Bob: 2 BTC (200 unități)

	ev := TradeEvent{
		TradeID:      "trade-xyz",
		BuyerUserID:  "alice",
		SellerUserID: "bob",
		Instrument:   "BTC",
		Quantity:     100,     // 1 BTC
		Price:        810_000, // 8100 USD (în cenți)
	}

	s.ApplyTrade(ev)

	usdValue := ev.Price * ev.Quantity / 100 // 810.000 cenți = 8100 USD

	if got := s.getBalance("alice", "BTC"); got != 100 {
		t.Errorf("alice BTC: expected 100, got %d", got)
	}
	if got := s.getBalance("alice", "USD"); got != 1_000_000-usdValue {
		t.Errorf("alice USD: expected %d, got %d", 1_000_000-usdValue, got)
	}
	if got := s.getBalance("bob", "BTC"); got != 100 {
		t.Errorf("bob BTC: expected 100 (200-100), got %d", got)
	}
	if got := s.getBalance("bob", "USD"); got != usdValue {
		t.Errorf("bob USD: expected %d, got %d", usdValue, got)
	}
}

// TestIdempotency_MultipleDifferentTrades verifică că evenimente diferite nu se
// interferează: două trade_id distincte produc fiecare propriile efecte contabile.
func TestIdempotency_MultipleDifferentTrades(t *testing.T) {
	s := NewInMemorySettler()

	s.addBalance("user1", "USD", 5_000_000)
	s.addBalance("user2", "BTC", 1000)

	ev1 := TradeEvent{
		TradeID: "trade-1", BuyerUserID: "user1", SellerUserID: "user2",
		Instrument: "BTC", Quantity: 100, Price: 810_000,
	}
	ev2 := TradeEvent{
		TradeID: "trade-2", BuyerUserID: "user1", SellerUserID: "user2",
		Instrument: "BTC", Quantity: 200, Price: 820_000,
	}

	s.ApplyTrade(ev1)
	s.ApplyTrade(ev2)

	// Fiecare eveniment produce 4 intrări în jurnal → total 8
	if len(s.ledger) != 8 {
		t.Errorf("expected 8 intrări în jurnal (2 tranzacții × 4), got %d", len(s.ledger))
	}

	// Trimitere duplicat pentru ev1 — nu trebuie să adauge intrări
	s.ApplyTrade(ev1)
	if len(s.ledger) != 8 {
		t.Errorf("duplicatul nu trebuie să adauge intrări, got %d", len(s.ledger))
	}
}
