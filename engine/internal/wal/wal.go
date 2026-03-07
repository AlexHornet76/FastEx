package wal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// Write-ahead log
type WAL struct {
	filePath    string
	file        *os.File
	writer      *bufio.Writer
	sequenceNum uint64     // atomic counter
	mu          sync.Mutex // protect file writes
}

// Open opens or creates WAL file
func Open(dirPath, fileName string) (*WAL, error) {
	// Ensure directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("create WAL directory: %w", err)
	}
	filePath := filepath.Join(dirPath, fileName)

	// Open file in append mode
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open WAL file: %w", err)
	}

	// Determine current sequence number by reading existing entries
	seqNum, err := getLastSequenceNumber(filePath)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("get last sequence number: %w", err)
	}
	wal := &WAL{
		filePath:    filePath,
		file:        file,
		writer:      bufio.NewWriter(file),
		sequenceNum: seqNum,
	}

	return wal, nil
}

// getLastSequenceNumber reads the WAL file and returns the last sequence number
func getLastSequenceNumber(filePath string) (uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // New file, start at 0
		}
		return 0, err
	}
	defer file.Close()

	var lastSeq uint64
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // Skip malformed lines
		}
		if entry.SequenceNum > lastSeq {
			lastSeq = entry.SequenceNum
		}
	}
	return lastSeq, scanner.Err()
}

// Close closes the WAL file
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}

	return w.file.Close()
}

// Append writes an entry to the WAL
func (w *WAL) Append(entry *Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry.SequenceNum = atomic.AddUint64(&w.sequenceNum, 1)
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}
	if _, err := w.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}

	// Flush buffer to OS
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("flush buffer: %w", err)
	}

	// Force to disk (fsync)
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync to disk: %w", err)
	}

	return nil

}

// Replay reads all entries from the WAL and calls the callback for each entry
func (w *WAL) Replay(callback func(*Entry) error) error {
	file, err := os.Open(w.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open WAL file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse entry
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return fmt.Errorf("parse entry at line %d: %w", lineNum, err)
		}

		// Call callback
		if err := callback(&entry); err != nil {
			return fmt.Errorf("replay entry %d: %w", entry.SequenceNum, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read WAL file: %w", err)
	}

	return nil
}

// Truncate removes all entries from the WAL (for testing)
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.writer.Reset(w.file)
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	_, err := w.file.Seek(0, io.SeekStart)
	atomic.StoreUint64(&w.sequenceNum, 0)
	return err
}
