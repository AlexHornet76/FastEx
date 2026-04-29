import api from './api'

/**
 * Get market data for an instrument (price, change, volume)
 */
export async function getMarketData(symbol) {
  try {
    const response = await api.get(`/instruments/${symbol}`)
    return response.data || {
      current_price: 0,
      change_24h: 0,
      volume_24h: 0
    }
  } catch (error) {
    console.error(`Failed to fetch market data for ${symbol}:`, error)
    return {
      current_price: 0,
      change_24h: 0,
      volume_24h: 0
    }
  }
}

/**
 * Get candles for chart
 */
export async function getCandles(symbol, timeframe = '1h') {
  try {
    const response = await api.get(`/marketdata/candles/${symbol}`, {
      params: { timeframe }
    })
    return response.data || []
  } catch (error) {
    console.error(`Failed to fetch candles for ${symbol}:`, error)
    return []
  }
}

/**
 * Get order book snapshot
 */
export async function getOrderbookSnapshot(symbol) {
  try {
    const response = await api.get(`/orderbook/${symbol}`)
    return response.data || { bids: [], asks: [] }
  } catch (error) {
    console.error(`Failed to fetch orderbook for ${symbol}:`, error)
    return { bids: [], asks: [] }
  }
}