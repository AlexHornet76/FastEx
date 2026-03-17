package wal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// ReadEntriesFromFile reads WAL entries from filePath and calls fn for each entry
// with SequenceNum > afterSeq (strictly greater).
func ReadEntriesFromFile(filePath string, afterSeq uint64, fn func(*Entry) error) error {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open wal file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip malformed line
			continue
		}

		if e.SequenceNum <= afterSeq {
			continue
		}

		if err := fn(&e); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan wal: %w", err)
	}
	return nil
}
