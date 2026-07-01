import api from './api'

/**
 * Get ticker for an instrument.
 * Returns { current_price (USD float), volume_24h, change_24h }.
 * Backend last_price is int64 cents → divide by 100 for display.
 */
export async function getMarketData(symbol) {
  try {
    const response = await api.get(`/market/ticker/${symbol}`)
    const data = response.data || {}
    return {
      current_price: data.last_price ? data.last_price / 100 : 0,
      change_24h: data.change_pct ?? 0,
      volume_24h: data.volume ? data.volume / 100 : 0,
    }
  } catch (error) {
    console.error(`Failed to fetch market data for ${symbol}:`, error)
    return { current_price: 0, change_24h: 0, volume_24h: 0 }
  }
}

/**
 * Get OHLCV candles for chart.
 * Backend open/high/low/close are int64 cents → divide by 100 for display.
 */
export async function getCandles(symbol, limit = 50) {
  try {
    const response = await api.get(`/market/candles/${symbol}`, { params: { limit } })
    const candles = (response.data && response.data.candles) || []
    return candles.map(c => ({
      ...c,
      open:  c.open  / 100,
      high:  c.high  / 100,
      low:   c.low   / 100,
      close: c.close / 100,
    }))
  } catch (error) {
    console.error(`Failed to fetch candles for ${symbol}:`, error)
    return []
  }
}

export async function getOrderbookSnapshot(symbol) {
  try {
    const response = await api.get(`/orderbook/${symbol}`)
    return response.data || { bids: [], asks: [] }
  } catch (error) {
    console.error(`Failed to fetch orderbook for ${symbol}:`, error)
    return { bids: [], asks: [] }
  }
}
