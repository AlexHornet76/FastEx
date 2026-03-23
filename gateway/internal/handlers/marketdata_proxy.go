package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

type MarketDataProxyHandler struct {
	baseURL *url.URL
	client  *http.Client
}

func NewMarketDataProxyHandler(marketDataURL string) (*MarketDataProxyHandler, error) {
	u, err := url.Parse(marketDataURL)
	if err != nil {
		return nil, err
	}

	return &MarketDataProxyHandler{
		baseURL: u,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

// GetTicker proxies GET /market/ticker/{instrument}
func (h *MarketDataProxyHandler) GetTicker(w http.ResponseWriter, r *http.Request) {
	instrument := r.PathValue("instrument")
	if instrument == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Instrument required")
		return
	}

	upstream := *h.baseURL
	upstream.Path = "/market/ticker/" + instrument

	h.proxyGET(w, r, upstream.String())
}

// GetCandles proxies GET /market/candles/{instrument}?limit=...
func (h *MarketDataProxyHandler) GetCandles(w http.ResponseWriter, r *http.Request) {
	instrument := r.PathValue("instrument")
	if instrument == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Instrument required")
		return
	}

	upstream := *h.baseURL
	upstream.Path = "/market/candles/" + instrument
	upstream.RawQuery = r.URL.RawQuery

	h.proxyGET(w, r, upstream.String())
}

func (h *MarketDataProxyHandler) proxyGET(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "UPSTREAM_ERROR", "Failed to create upstream request")
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Error("marketdata proxy request failed", "upstream", upstreamURL, "error", err)
		respondError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Marketdata service unavailable")
		return
	}
	defer resp.Body.Close()

	// pass-through status + content-type
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	_, _ = w.Write(body)
}
