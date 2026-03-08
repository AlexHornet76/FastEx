package wal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

func TestWAL_AppendAndReplay(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Open WAL
	wal, err := Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	// Create test order
	order := &models.Order{
		OrderID:    uuid.New(),
		UserID:     uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5000000,
		Quantity:   100,
		Status:     models.New,
		Timestamp:  time.Now(),
	}

	// Write order entry
	entry, err := NewOrderPlacedEntry(0, order)
	if err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if err := wal.Append(entry); err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}

	// Close and reopen to test persistence
	wal.Close()

	// Reopen for replay
	wal, err = Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	// Replay
	var replayedOrders []*models.Order
	err = wal.Replay(func(e *Entry) error {
		if e.Type == TypeOrderPlaced {
			data, err := e.ParseOrderPlacedData()
			if err != nil {
				return err
			}
			replayedOrders = append(replayedOrders, data.Order)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify
	if len(replayedOrders) != 1 {
		t.Fatalf("Expected 1 order, got %d", len(replayedOrders))
	}

	if replayedOrders[0].OrderID != order.OrderID {
		t.Errorf("Order ID mismatch")
	}

	if replayedOrders[0].Price != order.Price {
		t.Errorf("Price mismatch: expected %d, got %d", order.Price, replayedOrders[0].Price)
	}
}

func TestWAL_SequenceNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	// Append 3 entries
	for i := 0; i < 3; i++ {
		order := &models.Order{
			OrderID:    uuid.New(),
			Instrument: "BTC-USD",
			Side:       models.Buy,
			Price:      5000000,
			Quantity:   100,
		}

		entry, _ := NewOrderPlacedEntry(0, order)
		if err := wal.Append(entry); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	wal.Close()

	// Reopen for replay
	wal, err = Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	// Replay and check sequence numbers
	var seqNums []uint64
	err = wal.Replay(func(e *Entry) error {
		seqNums = append(seqNums, e.SequenceNum)
		return nil
	})

	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify sequential
	if len(seqNums) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(seqNums))
	}

	for i, seq := range seqNums {
		expected := uint64(i + 1)
		if seq != expected {
			t.Errorf("Entry %d: expected seq %d, got %d", i, expected, seq)
		}
	}
}

func TestWAL_MultipleEntryTypes(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	// Order placed
	order := &models.Order{
		OrderID:    uuid.New(),
		Instrument: "BTC-USD",
		Side:       models.Buy,
		Price:      5000000,
		Quantity:   100,
	}
	entry1, _ := NewOrderPlacedEntry(0, order)
	wal.Append(entry1)

	// Trade executed
	trade := &models.Trade{
		TradeID:     uuid.New(),
		Instrument:  "BTC-USD",
		BuyOrderID:  order.OrderID,
		SellOrderID: uuid.New(),
		Price:       5000000,
		Quantity:    50,
		Timestamp:   time.Now(),
	}
	entry2, _ := NewTradeExecutedEntry(0, trade)
	wal.Append(entry2)

	// Order cancelled
	entry3, _ := NewOrderCanceledEntry(0, order.OrderID, "BTC-USD", 5000000)
	wal.Append(entry3)

	wal.Close()

	// Reopen for replay
	wal, err = Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	// Replay and count types
	typeCounts := make(map[EntryType]int)
	err = wal.Replay(func(e *Entry) error {
		typeCounts[e.Type]++
		return nil
	})

	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if typeCounts[TypeOrderPlaced] != 1 {
		t.Errorf("Expected 1 ORDER_PLACED, got %d", typeCounts[TypeOrderPlaced])
	}

	if typeCounts[TypeTradeExecuted] != 1 {
		t.Errorf("Expected 1 TRADE_EXECUTED, got %d", typeCounts[TypeTradeExecuted])
	}

	if typeCounts[TypeOrderCanceled] != 1 {
		t.Errorf("Expected 1 ORDER_CANCELED, got %d", typeCounts[TypeOrderCanceled])
	}
}

func TestWAL_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	// Write entries
	{
		wal, _ := Open(tmpDir, "test.wal")
		order := &models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Price: 5000000}
		entry, _ := NewOrderPlacedEntry(0, order)
		wal.Append(entry)
		wal.Close()
	}

	// Verify file exists
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Fatal("WAL file should exist after close")
	}

	// Reopen and append more
	{
		wal, _ := Open(tmpDir, "test.wal")
		defer wal.Close()

		// Sequence number should continue from previous
		order := &models.Order{OrderID: uuid.New(), Instrument: "BTC-USD", Price: 5100000}
		entry, _ := NewOrderPlacedEntry(0, order)
		wal.Append(entry)

		if entry.SequenceNum != 2 {
			t.Errorf("Expected sequence 2, got %d", entry.SequenceNum)
		}
	}

	// Verify both entries
	wal2, err := Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal2.Close()

	var count int
	wal2.Replay(func(e *Entry) error {
		count++
		return nil
	})

	if count != 2 {
		t.Errorf("Expected 2 entries after reopen, got %d", count)
	}
}

func TestWAL_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	// Write concurrently (WAL should handle with mutex)
	const numGoroutines = 10
	const entriesPerGoroutine = 10

	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < entriesPerGoroutine; j++ {
				order := &models.Order{
					OrderID:    uuid.New(),
					Instrument: "BTC-USD",
					Price:      5000000,
				}
				entry, _ := NewOrderPlacedEntry(0, order)
				if err := wal.Append(entry); err != nil {
					errChan <- err
					return
				}
			}
			errChan <- nil
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent write failed: %v", err)
		}
	}

	wal.Close()

	// Reopen for replay
	wal, err = Open(tmpDir, "test.wal")
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	// Verify all entries written
	var count int
	wal.Replay(func(e *Entry) error {
		count++
		return nil
	})

	expected := numGoroutines * entriesPerGoroutine
	if count != expected {
		t.Errorf("Expected %d entries, got %d", expected, count)
	}
}
