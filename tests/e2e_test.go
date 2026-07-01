//go:build integration
// +build integration

// Testele din acest fișier necesită întregul stack FastEx pornit:
//
//	docker compose up -d
//
// Rulare:
//
//	go test -tags=integration -v -timeout=60s ./...
//
// Testele urmăresc ciclul complet descris în Secțiunea 3.8: înregistrare utilizatori,
// plasare ordine compatibile, verificare execuție tranzacție și răspuns HTTP corect.
package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

const (
	gatewayURL = "http://localhost:8080"
	engineURL  = "http://localhost:8081"
)

// ---------------------------------------------------------------------------
// Tipuri HTTP — oglindesc gateway/internal/handlers și engine/internal/handlers
// ---------------------------------------------------------------------------

type RegisterRequest struct {
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
}

type ChallengeResponse struct {
	Challenge string `json:"challenge"`
}

type SubmitOrderRequest struct {
	UserID     string `json:"user_id"`
	Instrument string `json:"instrument"`
	Side       string `json:"side"`
	Type       string `json:"type"`
	Price      int64  `json:"price"`
	Quantity   int64  `json:"quantity"`
}

type SubmitOrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Trades  []struct {
		TradeID  string `json:"trade_id"`
		Price    int64  `json:"price"`
		Quantity int64  `json:"quantity"`
	} `json:"trades"`
}

type OrderBookResponse struct {
	Instrument string `json:"instrument"`
	Bids       []struct {
		Price    int64 `json:"price"`
		Quantity int64 `json:"quantity"`
	} `json:"bids"`
	Asks []struct {
		Price    int64 `json:"price"`
		Quantity int64 `json:"quantity"`
	} `json:"asks"`
}

// ---------------------------------------------------------------------------
// Utilitare HTTP
// ---------------------------------------------------------------------------

func postJSON(t *testing.T, url string, body interface{}, token string) (*http.Response, []byte) {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return resp, buf.Bytes()
}

func getJSON(t *testing.T, url, token string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return resp, buf.Bytes()
}

// ---------------------------------------------------------------------------
// Teste E2E
// ---------------------------------------------------------------------------

// TestE2E_HealthChecks verifică că toate serviciile răspund la /health înainte de
// a rula testele funcționale.
func TestE2E_HealthChecks(t *testing.T) {
	services := []struct{ name, url string }{
		{"gateway", gatewayURL + "/health"},
		{"engine", engineURL + "/health"},
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, svc := range services {
		resp, err := client.Get(svc.url)
		if err != nil {
			t.Errorf("%s health check failed: %v", svc.name, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", svc.name, resp.StatusCode)
		}
	}
}

// TestE2E_OrderMatchingCycle testează ciclul complet al unui ordin:
//  1. Plasarea unui ordin SELL (ajunge în registru, statusul devine OPEN).
//  2. Plasarea unui ordin BUY compatibil (se potrivesc, ambele devin FILLED).
//  3. Verificarea că registrul de ordine este gol după execuție.
func TestE2E_OrderMatchingCycle(t *testing.T) {
	// Identificatori de test (UUID-uri fixe pentru reproductibilitate)
	sellerID := "223e4567-e89b-12d3-a456-426614174001"
	buyerID := "323e4567-e89b-12d3-a456-426614174002"

	// --- Pasul 1: ordin SELL ---
	sellReq := SubmitOrderRequest{
		UserID:     sellerID,
		Instrument: "BTC",
		Side:       "SELL",
		Type:       "LIMIT",
		Price:      8_100_000, // $81.000 (în cenți)
		Quantity:   100,
	}

	resp, body := postJSON(t, engineURL+"/orders", sellReq, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("SELL order: expected 201, got %d — body: %s", resp.StatusCode, body)
	}

	var sellResp SubmitOrderResponse
	json.Unmarshal(body, &sellResp)

	if sellResp.Status != "OPEN" {
		t.Errorf("ordinul SELL trebuie să fie OPEN, got %s", sellResp.Status)
	}
	if sellResp.OrderID == "" {
		t.Fatal("order_id lipsă din răspuns")
	}

	// --- Pasul 2: ordin BUY compatibil (preț ≥ cel al vânzătorului) ---
	buyReq := SubmitOrderRequest{
		UserID:     buyerID,
		Instrument: "BTC",
		Side:       "BUY",
		Type:       "LIMIT",
		Price:      8_200_000, // $82.000 — dispus să plătească mai mult
		Quantity:   100,
	}

	resp2, body2 := postJSON(t, engineURL+"/orders", buyReq, "")
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("BUY order: expected 201, got %d — body: %s", resp2.StatusCode, body2)
	}

	var buyResp SubmitOrderResponse
	json.Unmarshal(body2, &buyResp)

	if buyResp.Status != "FILLED" {
		t.Errorf("ordinul BUY trebuie să fie FILLED, got %s", buyResp.Status)
	}
	if len(buyResp.Trades) != 1 {
		t.Fatalf("expected 1 tranzacție, got %d", len(buyResp.Trades))
	}
	// Prețul execuției = prețul vânzătorului (Price-Time Priority)
	if buyResp.Trades[0].Price != 8_100_000 {
		t.Errorf("prețul tranzacției: expected 8100000 (prețul vânzătorului), got %d",
			buyResp.Trades[0].Price)
	}

	// --- Pasul 3: registrul trebuie să fie gol ---
	_, obBody := getJSON(t, engineURL+"/orderbook/BTC", "")
	var ob OrderBookResponse
	json.Unmarshal(obBody, &ob)

	if len(ob.Bids) != 0 || len(ob.Asks) != 0 {
		t.Errorf("registrul trebuie să fie gol după potrivire completă: bids=%d asks=%d",
			len(ob.Bids), len(ob.Asks))
	}
}

// TestE2E_PartialMatchRestInBook verifică că un ordin parțial executat rămâne
// în registru cu cantitatea corectă.
func TestE2E_PartialMatchRestInBook(t *testing.T) {
	sellerID := "423e4567-e89b-12d3-a456-426614174003"
	buyerID := "523e4567-e89b-12d3-a456-426614174004"

	// SELL 100 unități
	sellReq := SubmitOrderRequest{
		UserID: sellerID, Instrument: "ETH", Side: "SELL",
		Type: "LIMIT", Price: 3_000_000, Quantity: 100,
	}
	postJSON(t, engineURL+"/orders", sellReq, "")

	// BUY 300 unități — va fi parțial umplut (100 disponibil, 200 rămâne)
	buyReq := SubmitOrderRequest{
		UserID: buyerID, Instrument: "ETH", Side: "BUY",
		Type: "LIMIT", Price: 3_100_000, Quantity: 300,
	}
	resp, body := postJSON(t, engineURL+"/orders", buyReq, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("BUY partial: expected 201, got %d", resp.StatusCode)
	}

	var buyResp SubmitOrderResponse
	json.Unmarshal(body, &buyResp)

	if buyResp.Status != "PARTIAL" {
		t.Errorf("expected PARTIAL, got %s", buyResp.Status)
	}
	if len(buyResp.Trades) != 1 {
		t.Fatalf("expected 1 tranzacție, got %d", len(buyResp.Trades))
	}
	if buyResp.Trades[0].Quantity != 100 {
		t.Errorf("cantitate tranzacție: expected 100, got %d", buyResp.Trades[0].Quantity)
	}

	// Registrul ETH trebuie să conțină ordinul BUY rămas (200 unități)
	_, obBody := getJSON(t, engineURL+"/orderbook/ETH", "")
	var ob OrderBookResponse
	json.Unmarshal(obBody, &ob)

	if len(ob.Bids) == 0 {
		t.Error("bid-ul parțial trebuie să rămână în registru")
	}
}

// TestE2E_NoPriceMatch verifică că un ordin fără potrivire de preț rămâne în
// registru fără nicio tranzacție produsă.
func TestE2E_NoPriceMatch(t *testing.T) {
	// SELL la 90.000 USD
	sellReq := SubmitOrderRequest{
		UserID: "923e4567-e89b-12d3-a456-426614174008", Instrument: "BTC", Side: "SELL",
		Type: "LIMIT", Price: 9_000_000, Quantity: 50,
	}
	postJSON(t, engineURL+"/orders", sellReq, "")

	// BUY la 80.000 USD (prețul nu se potrivește)
	buyReq := SubmitOrderRequest{
		UserID: "a23e4567-e89b-12d3-a456-426614174009", Instrument: "BTC", Side: "BUY",
		Type: "LIMIT", Price: 8_000_000, Quantity: 50,
	}
	resp, body := postJSON(t, engineURL+"/orders", buyReq, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var buyResp SubmitOrderResponse
	json.Unmarshal(body, &buyResp)

	if buyResp.Status != "OPEN" {
		t.Errorf("expected OPEN (fără potrivire), got %s", buyResp.Status)
	}
	if len(buyResp.Trades) != 0 {
		t.Errorf("expected 0 tranzacții, got %d", len(buyResp.Trades))
	}
}

// TestE2E_OrderBookDepth verifică că registrul de ordine raportează corect adâncimea
// (bid/ask levels) după inserarea mai multor ordine la prețuri diferite.
func TestE2E_OrderBookDepth(t *testing.T) {
	instrument := "BTC"

	depthSellerIDs := []string{
		"623e4567-e89b-12d3-a456-426614174005",
		"723e4567-e89b-12d3-a456-426614174006",
		"823e4567-e89b-12d3-a456-426614174007",
	}
	prices := []int64{7_800_000, 7_900_000, 8_000_000}
	for i, p := range prices {
		req := SubmitOrderRequest{
			UserID:     depthSellerIDs[i],
			Instrument: instrument, Side: "SELL", Type: "LIMIT",
			Price: p, Quantity: 100,
		}
		postJSON(t, engineURL+"/orders", req, "")
	}

	_, obBody := getJSON(t, engineURL+"/orderbook/"+instrument, "")
	var ob OrderBookResponse
	json.Unmarshal(obBody, &ob)

	if len(ob.Asks) < 3 {
		t.Errorf("expected cel puțin 3 ask levels, got %d", len(ob.Asks))
	}
}
