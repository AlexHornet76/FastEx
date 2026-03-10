//go:build integration
// +build integration

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/AlexHornet76/FastEx/engine/internal/handlers"
)

const baseURL = "http://localhost:8081"

func TestFullOrderFlow(t *testing.T) {
	// Test health
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Submit sell order
	sellReq := handlers.SubmitOrderRequest{
		UserID:     "123e4567-e89b-12d3-a456-426614174000",
		Instrument: "BTC-USD",
		Side:       "SELL",
		Type:       "LIMIT",
		Price:      5010000, // $50,100
		Quantity:   500,
	}

	sellResp := submitOrder(t, sellReq)
	if sellResp.Status != "OPEN" {
		t.Errorf("Sell order should be OPEN, got %s", sellResp.Status)
	}

	// Submit buy order (matches)
	buyReq := handlers.SubmitOrderRequest{
		UserID:     "223e4567-e89b-12d3-a456-426614174001",
		Instrument: "BTC-USD",
		Side:       "BUY",
		Type:       "LIMIT",
		Price:      5020000, // $50,200
		Quantity:   500,
	}

	buyResp := submitOrder(t, buyReq)
	if buyResp.Status != "FILLED" {
		t.Errorf("Buy order should be FILLED, got %s", buyResp.Status)
	}

	if len(buyResp.Trades) != 1 {
		t.Errorf("Expected 1 trade, got %d", len(buyResp.Trades))
	}

	if buyResp.Trades[0].Price != 5010000 {
		t.Errorf("Trade should execute at sell price (5010000), got %d", buyResp.Trades[0].Price)
	}

	// Get order book
	obResp := getOrderBook(t, "BTC-USD")
	if obResp.Instrument != "BTC-USD" {
		t.Errorf("Wrong instrument in order book")
	}
}

func submitOrder(t *testing.T, req handlers.SubmitOrderRequest) *handlers.SubmitOrderResponse {
	jsonData, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/orders", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Submit order failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var result handlers.SubmitOrderResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func getOrderBook(t *testing.T, instrument string) *handlers.OrderBookResponse {
	resp, err := http.Get(baseURL + "/orderbook/" + instrument)
	if err != nil {
		t.Fatalf("Get order book failed: %v", err)
	}
	defer resp.Body.Close()

	var result handlers.OrderBookResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}
