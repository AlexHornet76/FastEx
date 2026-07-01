//go:build integration
// +build integration

package tests

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestMain rulează o singură dată înainte de toate testele din pachet.
// Șterge fișierele WAL din containerul engine și îl repornește astfel încât
// order book-ul să fie gol și starea să fie deterministă.
func TestMain(m *testing.M) {
	if err := resetEngineState(); err != nil {
		fmt.Printf("Warning: engine reset failed (%v) — tests may use stale WAL state\n", err)
	}
	os.Exit(m.Run())
}

func resetEngineState() error {
	// Șterge fișierele WAL din container
	clear := exec.Command("docker", "exec", "engine", "sh", "-c", "rm -f /app/data/wal/*.wal")
	if out, err := clear.CombinedOutput(); err != nil {
		return fmt.Errorf("clear WAL: %w — %s", err, out)
	}

	// Repornește engine-ul ca să pornească cu WAL gol
	if err := exec.Command("docker", "restart", "engine").Run(); err != nil {
		return fmt.Errorf("restart engine: %w", err)
	}

	// Așteaptă până când engine-ul e healthy (max 15s)
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 15; i++ {
		time.Sleep(time.Second)
		resp, err := client.Get(engineURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("Engine reset OK — order book is clean.")
			return nil
		}
	}
	return fmt.Errorf("engine did not become healthy within 15s")
}
