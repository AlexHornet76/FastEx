// Fișierul testează durabilitatea WAL și recuperarea după crash.
// Simulează scenariul descris în Secțiunea 3.8: plasarea unor ordine,
// oprirea bruscă a procesului (prin ieșire fără Sync suplimentar) și
// verificarea că jurnalul poate fi redat corect la repornire.
package tests

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// WAL minimal — JSON lines + fsync, oglindind engine/internal/wal
// ---------------------------------------------------------------------------

type EntryType string

const (
	TypeOrderPlaced  EntryType = "ORDER_PLACED"
	TypeTradeExecuted EntryType = "TRADE_EXECUTED"
	TypeOrderCanceled EntryType = "ORDER_CANCELED"
)

type WALEntry struct {
	SequenceNum uint64      `json:"seq"`
	Type        EntryType   `json:"type"`
	Timestamp   time.Time   `json:"ts"`
	Payload     interface{} `json:"payload"`
}

type MinimalWAL struct {
	file    *os.File
	writer  *bufio.Writer
	nextSeq uint64
}

// openWAL deschide sau creează fișierul WAL și determină numărul de secvență următor
// prin redarea intrărilor existente (recovery path la startup).
func openWAL(path string) (*MinimalWAL, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := &MinimalWAL{file: f, writer: bufio.NewWriter(f)}

	// Determină nextSeq citind toate intrările existente
	scanner := bufio.NewScanner(f)
	var maxSeq uint64
	for scanner.Scan() {
		var e WALEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.SequenceNum > maxSeq {
			maxSeq = e.SequenceNum
		}
	}
	w.nextSeq = maxSeq + 1

	// Repoziționează la final pentru scriere
	f.Seek(0, 2)
	return w, nil
}

// Append serializează intrarea, o scrie în buffer, flushează la kernel și
// sincronizează pe disc (echivalentul fsync din WAL.Append din engine).
func (w *MinimalWAL) Append(entryType EntryType, payload interface{}) (uint64, error) {
	seq := w.nextSeq
	w.nextSeq++

	entry := WALEntry{
		SequenceNum: seq,
		Type:        entryType,
		Timestamp:   time.Now(),
		Payload:     payload,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}

	w.writer.Write(data)
	w.writer.WriteByte('\n')

	// Flush buffer -> kernel
	if err := w.writer.Flush(); err != nil {
		return 0, err
	}
	// Sync kernel -> disc fizic (fsync) — garanția de durabilitate
	if err := w.file.Sync(); err != nil {
		return 0, err
	}

	return seq, nil
}

func (w *MinimalWAL) Close() error { return w.file.Close() }

// replay redă toate intrările dintr-un fișier WAL și le pasează callback-ului.
func replay(path string, fn func(WALEntry)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e WALEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			fn(e)
		}
	}
	return scanner.Err()
}

// ---------------------------------------------------------------------------
// Teste
// ---------------------------------------------------------------------------

// TestWAL_PersistenceAfterSync verifică că intrările scrise cu fsync sunt vizibile
// la redeschiderea fișierului — garanția de bază a oricărui WAL.
func TestWAL_PersistenceAfterSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTC-USD.wal")

	w, err := openWAL(path)
	if err != nil {
		t.Fatalf("openWAL: %v", err)
	}

	_, err = w.Append(TypeOrderPlaced, map[string]interface{}{
		"order_id": "ord-001", "side": "BUY", "price": 8100000, "qty": 100,
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	_, err = w.Append(TypeOrderPlaced, map[string]interface{}{
		"order_id": "ord-002", "side": "SELL", "price": 8050000, "qty": 50,
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Simulare "crash": închidem fără operații suplimentare (fsync deja apelat).
	w.Close()

	// Redare după repornire
	var entries []WALEntry
	if err := replay(path, func(e WALEntry) { entries = append(entries, e) }); err != nil {
		t.Fatalf("replay: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 intrări după repornire, got %d", len(entries))
	}
	if entries[0].SequenceNum != 1 {
		t.Errorf("prima intrare trebuie să aibă seq=1, got %d", entries[0].SequenceNum)
	}
	if entries[1].SequenceNum != 2 {
		t.Errorf("a doua intrare trebuie să aibă seq=2, got %d", entries[1].SequenceNum)
	}
	if entries[0].Type != TypeOrderPlaced {
		t.Errorf("tipul intrării: expected ORDER_PLACED, got %s", entries[0].Type)
	}
}

// TestWAL_SequenceNumbers verifică că numerele de secvență sunt consecutive și
// că după redeschidere continuă de unde au rămas (nu reîncep de la 1).
func TestWAL_SequenceNumbers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ETH-USD.wal")

	// Sesiunea 1: scrie 3 intrări
	{
		w, _ := openWAL(path)
		for i := 0; i < 3; i++ {
			w.Append(TypeOrderPlaced, map[string]interface{}{"i": i})
		}
		w.Close()
	}

	// Sesiunea 2: redeschide și mai scrie 2 intrări
	{
		w, _ := openWAL(path)
		if w.nextSeq != 4 {
			t.Errorf("la redeschidere nextSeq trebuie să fie 4 (continuare), got %d", w.nextSeq)
		}
		w.Append(TypeTradeExecuted, map[string]interface{}{"trade_id": "t-001"})
		w.Append(TypeOrderCanceled, map[string]interface{}{"order_id": "ord-001"})
		w.Close()
	}

	// Verificare secvențe globale
	var seqs []uint64
	replay(path, func(e WALEntry) { seqs = append(seqs, e.SequenceNum) })

	if len(seqs) != 5 {
		t.Fatalf("expected 5 intrări totale, got %d", len(seqs))
	}
	for i, s := range seqs {
		expected := uint64(i + 1)
		if s != expected {
			t.Errorf("intrarea %d: expected seq=%d, got %d", i, expected, s)
		}
	}
}

// TestWAL_CrashRecovery este testul central al durabilității: simulează scenariul
// descris în Secțiunea 3.8 — ordine active + crash + repornire + reconstituire stare.
//
// Procedura:
//  1. Plasează 3 ordine (un BUY parțial executat, un SELL intact, un trade executat).
//  2. Simulează crash prin oprirea bruscă (fișierul este închis fără operații extra).
//  3. Repornire: redă WAL-ul și reconstituie starea registrului.
//  4. Verifică că starea post-repornire este identică cu starea pre-crash.
func TestWAL_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTC-USD.wal")

	type OrderPayload struct {
		OrderID string `json:"order_id"`
		Side    string `json:"side"`
		Price   int64  `json:"price"`
		Qty     int64  `json:"qty"`
		Filled  int64  `json:"filled"`
		Status  string `json:"status"`
	}
	type TradePayload struct {
		TradeID     string `json:"trade_id"`
		BuyOrderID  string `json:"buy_order_id"`
		SellOrderID string `json:"sell_order_id"`
		Price       int64  `json:"price"`
		Qty         int64  `json:"qty"`
	}

	// --- Faza 1: operare normală, scriere WAL ---
	w, err := openWAL(path)
	if err != nil {
		t.Fatalf("openWAL: %v", err)
	}

	// Ordin de cumpărare plasat
	w.Append(TypeOrderPlaced, OrderPayload{
		OrderID: "buy-1", Side: "BUY", Price: 8100000, Qty: 300, Status: "OPEN",
	})
	// Ordin de vânzare plasat
	w.Append(TypeOrderPlaced, OrderPayload{
		OrderID: "sell-1", Side: "SELL", Price: 8100000, Qty: 100, Status: "OPEN",
	})
	// Trade executat (buy-1 parțial umplut cu sell-1)
	w.Append(TypeTradeExecuted, TradePayload{
		TradeID: "trade-1", BuyOrderID: "buy-1", SellOrderID: "sell-1",
		Price: 8100000, Qty: 100,
	})

	// --- Faza 2: crash simulat ---
	// Procesul se oprește brusc; deoarece fiecare Append a apelat fsync,
	// toate cele 3 intrări sunt garantate pe disc.
	w.Close() // în scenariu real: SIGKILL — Close nu ar mai fi apelat

	// --- Faza 3: repornire și reconstituire stare ---
	type BookState struct {
		orders map[string]OrderPayload
		trades []TradePayload
	}
	state := BookState{orders: make(map[string]OrderPayload)}

	err = replay(path, func(e WALEntry) {
		raw, _ := json.Marshal(e.Payload)
		switch e.Type {
		case TypeOrderPlaced:
			var p OrderPayload
			json.Unmarshal(raw, &p)
			state.orders[p.OrderID] = p
		case TypeTradeExecuted:
			var p TradePayload
			json.Unmarshal(raw, &p)
			state.trades = append(state.trades, p)
			// Actualizare stare ordine din trade
			if o, ok := state.orders[p.BuyOrderID]; ok {
				o.Filled += p.Qty
				if o.Filled >= o.Qty {
					o.Status = "FILLED"
				} else {
					o.Status = "PARTIAL"
				}
				state.orders[p.BuyOrderID] = o
			}
			if o, ok := state.orders[p.SellOrderID]; ok {
				o.Filled += p.Qty
				if o.Filled >= o.Qty {
					o.Status = "FILLED"
				} else {
					o.Status = "PARTIAL"
				}
				state.orders[p.SellOrderID] = o
			}
		}
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// --- Faza 4: verificare stare post-repornire ---
	if len(state.orders) != 2 {
		t.Fatalf("expected 2 ordine în registru, got %d", len(state.orders))
	}
	if len(state.trades) != 1 {
		t.Fatalf("expected 1 tranzacție, got %d", len(state.trades))
	}

	buy1 := state.orders["buy-1"]
	if buy1.Status != "PARTIAL" {
		t.Errorf("buy-1: expected PARTIAL după trade de 100/300, got %s", buy1.Status)
	}
	if buy1.Filled != 100 {
		t.Errorf("buy-1.Filled: expected 100, got %d", buy1.Filled)
	}

	sell1 := state.orders["sell-1"]
	if sell1.Status != "FILLED" {
		t.Errorf("sell-1: expected FILLED, got %s", sell1.Status)
	}
}

// TestWAL_MultipleEntryTypes verifică că toate tipurile de intrări (ORDER_PLACED,
// TRADE_EXECUTED, ORDER_CANCELED) sunt scrise și redate corect.
func TestWAL_MultipleEntryTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wal")

	w, _ := openWAL(path)
	w.Append(TypeOrderPlaced, map[string]interface{}{"order_id": "o1"})
	w.Append(TypeTradeExecuted, map[string]interface{}{"trade_id": "t1"})
	w.Append(TypeOrderCanceled, map[string]interface{}{"order_id": "o1"})
	w.Close()

	counts := make(map[EntryType]int)
	replay(path, func(e WALEntry) { counts[e.Type]++ })

	if counts[TypeOrderPlaced] != 1 {
		t.Errorf("ORDER_PLACED: expected 1, got %d", counts[TypeOrderPlaced])
	}
	if counts[TypeTradeExecuted] != 1 {
		t.Errorf("TRADE_EXECUTED: expected 1, got %d", counts[TypeTradeExecuted])
	}
	if counts[TypeOrderCanceled] != 1 {
		t.Errorf("ORDER_CANCELED: expected 1, got %d", counts[TypeOrderCanceled])
	}
}
