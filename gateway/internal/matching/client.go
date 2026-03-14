package matching

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Client is an HTTP client for the matching engine
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new matching engine client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SubmitOrderRequest represents order submission to matching engine
type SubmitOrderRequest struct {
	UserID     string `json:"user_id"`
	Instrument string `json:"instrument"`
	Side       string `json:"side"`     // BUY or SELL
	Type       string `json:"type"`     // LIMIT or MARKET
	Price      int64  `json:"price"`    // Price in smallest unit (cents, satoshis)
	Quantity   int64  `json:"quantity"` // Quantity in smallest unit
}

// SubmitOrderResponse represents matching engine response
type SubmitOrderResponse struct {
	OrderID      string          `json:"order_id"`
	Status       string          `json:"status"`
	FilledQty    int64           `json:"filled_qty"`
	RemainingQty int64           `json:"remaining_qty"`
	Trades       []TradeResponse `json:"trades"`
}

// TradeResponse represents a trade
type TradeResponse struct {
	TradeID      string `json:"trade_id"`
	Instrument   string `json:"instrument"`
	Price        int64  `json:"price"`
	Quantity     int64  `json:"quantity"`
	BuyOrderID   string `json:"buy_order_id"`
	SellOrderID  string `json:"sell_order_id"`
	BuyerUserID  string `json:"buyer_user_id"`
	SellerUserID string `json:"seller_user_id"`
	Timestamp    string `json:"timestamp"`
}

// OrderBookResponse represents order book snapshot
type OrderBookResponse struct {
	Instrument string           `json:"instrument"`
	BestBid    int64            `json:"best_bid"`
	BestAsk    int64            `json:"best_ask"`
	Spread     int64            `json:"spread"`
	BidDepth   int              `json:"bid_depth"`
	AskDepth   int              `json:"ask_depth"`
	Bids       []PriceLevelInfo `json:"bids"`
	Asks       []PriceLevelInfo `json:"asks"`
}

// PriceLevelInfo represents aggregated price level
type PriceLevelInfo struct {
	Price      int64 `json:"price"`
	Quantity   int64 `json:"quantity"`
	OrderCount int   `json:"order_count"`
}

// ErrorResponse represents matching engine error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// SubmitOrder submits an order to the matching engine
func (c *Client) SubmitOrder(req SubmitOrderRequest) (*SubmitOrderResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/orders",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("matching engine error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("matching engine error: %s (code: %s)", errResp.Error, errResp.Code)
	}

	// Parse success response
	var result SubmitOrderResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

// CancelOrder cancels an order
func (c *Client) CancelOrder(orderID uuid.UUID, instrument string) error {
	url := fmt.Sprintf("%s/orders/%s?instrument=%s", c.baseURL, orderID.String(), instrument)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("cancel failed (status %d): %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("cancel failed: %s", errResp.Error)
	}

	return nil
}

// GetOrderBook retrieves order book snapshot
func (c *Client) GetOrderBook(instrument string) (*OrderBookResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/orderbook/" + instrument)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get order book failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result OrderBookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

// HealthCheck checks matching engine health
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("matching engine unhealthy (status %d)", resp.StatusCode)
	}

	return nil
}
