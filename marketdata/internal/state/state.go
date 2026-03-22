package state

import (
	"sync"
	"time"
)

type Ticker struct {
	Instrument string    `json:"instrument"`
	LastPrice  int64     `json:"last_price"`
	LastQty    int64     `json:"last_qty"`
	LastTrade  time.Time `json:"last_trade"`

	TradeCount int64 `json:"trade_count"`
	Volume     int64 `json:"volume"`
}

type Store struct {
	mu      sync.RWMutex
	tickers map[string]*Ticker
	candles *CandleStore
}

func NewStore() *Store {
	return &Store{
		tickers: make(map[string]*Ticker),
		candles: NewCandleStore(10), // keep last 10 candles per instrument for demo
	}
}

func (s *Store) ApplyTrade(instrument string, price, qty int64, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x := s.tickers[instrument]
	if x == nil {
		x = &Ticker{Instrument: instrument}
		s.tickers[instrument] = x
	}

	x.LastPrice = price
	x.LastQty = qty
	x.LastTrade = t
	x.TradeCount++
	x.Volume += qty
	s.candles.ApplyTrade(instrument, qty, price, t)
}

func (s *Store) GetTicker(instrument string) (*Ticker, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	x := s.tickers[instrument]
	if x == nil {
		return nil, false
	}

	// return copy so caller can’t mutate internal state
	cp := *x
	return &cp, true
}

func (s *Store) GetCandles(instrument string, limit int) ([]Candle, bool) {
	return s.candles.GetCandles(instrument, limit)
}
