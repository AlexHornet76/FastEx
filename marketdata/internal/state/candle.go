package state

import (
	"math"
	"sort"
	"sync"
	"time"
)

type Candle struct {
	Instrument  string    `json:"instrument"`
	BucketStart time.Time `json:"bucket_start"` // UTC minute boundary

	Open  int64 `json:"open"`
	High  int64 `json:"high"`
	Low   int64 `json:"low"`
	Close int64 `json:"close"`

	Volume int64 `json:"volume"` // sum(quantity)
	Trades int64 `json:"trades"` // count of trades

	LastUpdate time.Time `json:"last_update"`
}

type CandleStore struct {
	mu sync.RWMutex

	// instrument -> bucketStartUnix -> candle
	candles map[string]map[int64]*Candle

	// keep last N per instrument
	limit int
}

func NewCandleStore(limit int) *CandleStore {
	return &CandleStore{
		candles: make(map[string]map[int64]*Candle),
		limit:   limit,
	}
}

// minuteBucket returns the UTC time truncated to the minute boundary.
// all trades within the same minute will map to the same bucket.
func minuteBucket(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

func (cs *CandleStore) ApplyTrade(instrument string, qty, price int64, t time.Time) {
	b := minuteBucket(t)
	bk := b.Unix()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	m := cs.candles[instrument]
	if m == nil {
		m = make(map[int64]*Candle)
		cs.candles[instrument] = m
	}

	c := m[bk]
	if c == nil {
		// first trade in this bucket initializes OHLC to price
		c = &Candle{
			Instrument:  instrument,
			BucketStart: b,
			Open:        price,
			High:        price,
			Low:         price,
			Close:       price,
			Volume:      0,
			Trades:      0,
		}
		m[bk] = c
	} else {
		if price > c.High {
			c.High = price
		}
		if price < c.Low {
			c.Low = price
		}
		c.Close = price
	}
	c.Volume += qty
	c.Trades++
	c.LastUpdate = time.Now().UTC()

	// evict old buckets if we exceed the limit
	if cs.limit > 0 && len(m) > cs.limit {
		var oldest int64 = math.MaxInt64
		for k := range m {
			if k < oldest {
				oldest = k
			}
		}
		delete(m, oldest)
	}
}

func (cs *CandleStore) GetCandles(instrument string, limit int) ([]Candle, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	m := cs.candles[instrument]
	if m == nil {
		return nil, false
	}

	// sort by bucket start time asc, then slice tail
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	if limit <= 0 || limit > len(keys) {
		limit = len(keys)
	}
	keys = keys[len(keys)-limit:]

	out := make([]Candle, 0, len(keys))
	for _, k := range keys {
		cp := *m[k]
		out = append(out, cp)
	}
	return out, true
}
