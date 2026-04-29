import api from './api'

/**
 * Get all available trading instruments
 * 
 * @returns {Promise<Array>} List of instruments with current prices
 */
export async function getInstruments() {
  try {
    const response = await api.get('/instruments')
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch instruments: ' + error.message)
  }
}

/**
 * Get order book for an instrument
 * 
 * @param {string} symbol - Instrument symbol (BTC, AAPL, etc)
 * @returns {Promise<Object>} Order book with bids and asks
 */
export async function getOrderbook(symbol) {
  try {
    const response = await api.get(`/orderbook/${symbol}`)
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch orderbook: ' + error.message)
  }
}

/**
 * Get user's balance
 * 
 * @returns {Promise<Object>} User's USD balance
 */
export async function getBalance() {
  try {
    const response = await api.get('/account/balance')
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch balance: ' + error.message)
  }
}

/**
 * Place a new order
 * 
 * @param {Object} order - Order details
 * @param {string} order.instrument - Instrument symbol
 * @param {string} order.side - BUY or SELL
 * @param {number} order.quantity - Quantity to buy/sell
 * @param {number} order.price - Limit price
 * @param {string} order.order_type - limit or market
 * @returns {Promise<Object>} Placed order details
 */
export async function placeOrder(order) {
  try {
    const response = await api.post('/orders', order)
    return response.data
  } catch (error) {
    throw new Error('Failed to place order: ' + error.message)
  }
}

/**
 * Get user's holdings
 * 
 * @returns {Promise<Array>} User's owned instruments with details
 */
export async function getHoldings() {
  try {
    const response = await api.get('/account/holdings')
    return response.data
  } catch (error) {
    throw new Error('Failed to fetch holdings: ' + error.message)
  }
}

/**
 * Cancel an order
 * 
 * @param {string} orderId - Order ID to cancel
 * @returns {Promise<Object>} Cancellation result
 */
export async function cancelOrder(orderId) {
  try {
    const response = await api.delete(`/orders/${orderId}`)
    return response.data
  } catch (error) {
    throw new Error('Failed to cancel order: ' + error.message)
  }
}