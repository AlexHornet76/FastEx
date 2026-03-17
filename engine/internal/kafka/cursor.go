package kafka

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func CursorPath(walDir string, instrument string) string {
	return filepath.Join(walDir, "cursors", instrument+".cursor")
}

func LoadCursor(walDir string, instrument string) (uint64, error) {
	path := CursorPath(walDir, instrument)

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read cursor: %w", err)
	}

	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, nil
	}

	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cursor %q: %w", s, err)
	}
	return v, nil
}

func SaveCursor(walDir string, instrument string, seq uint64) error {
	path := CursorPath(walDir, instrument)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir cursor dir: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(strconv.FormatUint(seq, 10)+"\n"), 0644); err != nil {
		return fmt.Errorf("write cursor tmp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename cursor tmp: %w", err)
	}
	return nil
}
