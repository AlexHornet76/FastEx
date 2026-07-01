package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
)

// Deposit proxies POST /account/deposit → settlement POST /deposit?user_id=...
func (sph *SettlementProxyHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	upstream := *sph.baseURL
	upstream.Path = "/deposit"
	q := url.Values{}
	q.Set("user_id", claims.UserID)
	if r.URL.Query().Get("seed") == "1" {
		q.Set("seed", "1")
	}
	upstream.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstream.String(), nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "UPSTREAM_ERROR", "Failed to create upstream request")
		return
	}

	resp, err := sph.client.Do(req)
	if err != nil {
		slog.Error("deposit proxy request failed", "upstream", upstream.String(), "error", err)
		respondError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Settlement service unavailable")
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	_, _ = w.Write(body)
}

type SettlementProxyHandler struct {
	baseURL *url.URL
	client  *http.Client
}

func NewSettlementProxyHandler(settlementURL string) (*SettlementProxyHandler, error) {
	u, err := url.Parse(settlementURL)
	if err != nil {
		return nil, fmt.Errorf("parse settlement URL: %w", err)
	}

	return &SettlementProxyHandler{
		baseURL: u,
		client:  &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// GetTrades proxies GET /trades?user_id=...&limit=...&cursor=...&instrument=...&status=...
// Public endpoint in gateway: GET /api/trades?limit=...&cursor=...&instrument=...&status=...
func (sph *SettlementProxyHandler) GetTrades(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	upstream := *sph.baseURL
	upstream.Path = "/api/trades"

	q := r.URL.Query()
	q.Set("user_id", claims.UserID)
	upstream.RawQuery = q.Encode()

	sph.proxyGET(w, r, upstream.String())
}

func (sph *SettlementProxyHandler) proxyGET(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "UPSTREAM_ERROR", "Failed to create upstream request")
		return
	}
	resp, err := sph.client.Do(req)
	if err != nil {
		slog.Error("settlement proxy request failed", "upstream", upstreamURL, "error", err)
		respondError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Settlement service unavailable")
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	_, _ = w.Write(body)
}

// GetBalance proxies GET /balance?user_id=...
// Public endpoint in gateway: GET /account/balance
func (sph *SettlementProxyHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	upstream := *sph.baseURL
	upstream.Path = "/balance"

	q := r.URL.Query()
	q.Set("user_id", claims.UserID)
	upstream.RawQuery = q.Encode()

	slog.Debug("proxying balance request", "user_id", claims.UserID, "upstream", upstream.String())
	sph.proxyGET(w, r, upstream.String())
}

// GetHoldings proxies GET /holdings?user_id=...&instrument=...&limit=...&cursor=...
// Public endpoint in gateway: GET /account/holdings
func (sph *SettlementProxyHandler) GetHoldings(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	upstream := *sph.baseURL
	upstream.Path = "/holdings"

	q := r.URL.Query()
	q.Set("user_id", claims.UserID)
	upstream.RawQuery = q.Encode()

	slog.Debug("proxying holdings request", "user_id", claims.UserID, "upstream", upstream.String())
	sph.proxyGET(w, r, upstream.String())
}

// GetCostBasis proxies GET /cost-basis?user_id=...
// Public endpoint in gateway: GET /account/cost-basis
func (sph *SettlementProxyHandler) GetCostBasis(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	upstream := *sph.baseURL
	upstream.Path = "/cost-basis"

	q := url.Values{}
	q.Set("user_id", claims.UserID)
	upstream.RawQuery = q.Encode()

	sph.proxyGET(w, r, upstream.String())
}

// GetLedger proxies GET /ledger?user_id=...&limit=...&cursor=...&type=...
// Public endpoint in gateway: GET /account/ledger
func (sph *SettlementProxyHandler) GetLedger(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	upstream := *sph.baseURL
	upstream.Path = "/ledger"

	q := r.URL.Query()
	q.Set("user_id", claims.UserID)
	upstream.RawQuery = q.Encode()

	slog.Debug("proxying ledger request", "user_id", claims.UserID, "upstream", upstream.String())
	sph.proxyGET(w, r, upstream.String())
}
