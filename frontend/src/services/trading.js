import api from './api'

export async function getInstruments() {
  try {
    const response = await api.get('/instruments')
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch instruments: ' + error.message)
  }
}

export async function getOrderbook(symbol) {
  try {
    const response = await api.get(`/orderbook/${symbol}`)
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch orderbook: ' + error.message)
  }
}

export async function getBalance() {
  try {
    const response = await api.get('/account/balance')
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch balance: ' + error.message)
  }
}

/**
 * Place a new order.
 * @param {Object} order
 * @param {string} order.instrument - e.g. "BTC"
 * @param {string} order.side       - "BUY" or "SELL"
 * @param {string} order.type       - "LIMIT" or "MARKET"
 * @param {number} order.price      - int64 cents (already converted by caller)
 * @param {number} order.quantity   - int64 100ths-precision (already converted by caller)
 */
export async function placeOrder(order) {
  try {
    const response = await api.post('/api/orders', order)
    return response.data
  } catch (error) {
    throw new Error('Failed to place order: ' + error.message)
  }
}

export async function getHoldings() {
  try {
    const response = await api.get('/account/holdings')
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch holdings: ' + error.message)
  }
}

export async function getCostBasis() {
  try {
    const response = await api.get('/account/cost-basis')
    const items = response.data?.cost_basis ?? []
    // avg_raw_price is in cents → divide by 100 for display dollars
    return Object.fromEntries(
      items.map(item => [item.asset, parseFloat(item.avg_raw_price) / 100])
    )
  } catch (error) {
    return {}
  }
}

export async function cancelOrder(orderId) {
  try {
    const response = await api.delete(`/orders/${orderId}`)
    return response.data
  } catch (error) {
    throw new Error('Failed to cancel order: ' + error.message)
  }
}

// Funds the demo account with $100,000 USD + instrument balances.
export async function deposit() {
  try {
    const response = await api.post('/account/deposit')
    return response.data
  } catch (error) {
    throw new Error('Failed to deposit: ' + error.message)
  }
}
