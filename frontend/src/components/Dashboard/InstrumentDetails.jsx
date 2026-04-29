import { useState, useEffect } from 'react'
import { getBalance, placeOrder } from '../../services/trading'
import OrderBook from './OrderBook'
import Chart from './Chart'
import Alert from '../Common/Alert'
import Spinner from '../Common/Spinner'
import './Dashboard.css'

/**
 * InstrumentDetail Component
 * 
 * Shows:
 * - Candlestick chart (OHLCV)
 * - Order book (bids/asks)
 * - Place order form (BUY/SELL)
 * - Order confirmation
 */
export default function InstrumentDetail({ instrument, onBack, balance, onBalanceUpdate }) {
  const [side, setSide] = useState('BUY') // BUY or SELL
  const [quantity, setQuantity] = useState('')
  const [price, setPrice] = useState('')
  const [orderType, setOrderType] = useState('limit') // limit or market
  
  const [loading, setLoading] = useState(false)
  const [alert, setAlert] = useState(null)
  const [userBalance, setUserBalance] = useState(balance)

  /**
   * Fetch user's current balance
   */
  useEffect(() => {
    const fetchBalance = async () => {
      try {
        const data = await getBalance()
        setUserBalance(data.usd_balance || 0)
        onBalanceUpdate(data.usd_balance || 0)
      } catch (err) {
        console.error('Failed to fetch balance:', err)
      }
    }

    fetchBalance()
  }, [onBalanceUpdate])

  /**
   * Handle order placement
   */
  const handlePlaceOrder = async (e) => {
    e.preventDefault()

    // Validate inputs
    if (!quantity || parseFloat(quantity) <= 0) {
      setAlert({ type: 'error', message: 'Invalid quantity' })
      return
    }

    if (orderType === 'limit' && (!price || parseFloat(price) <= 0)) {
      setAlert({ type: 'error', message: 'Invalid price' })
      return
    }

    setLoading(true)
    setAlert(null)

    try {
      // Calculate total cost
      const qty = parseFloat(quantity)
      const limitPrice = parseFloat(price) || instrument.current_price
      const totalCost = qty * limitPrice

      // Check balance for BUY orders
      if (side === 'BUY' && totalCost > userBalance) {
        setAlert({
          type: 'error',
          message: `Insufficient balance. Need $${totalCost.toFixed(2)}, have $${userBalance.toFixed(2)}`
        })
        setLoading(false)
        return
      }

      // Place order
      const result = await placeOrder({
        instrument: instrument.symbol,
        side,
        quantity: qty,
        price: limitPrice,
        order_type: orderType
      })

      console.log('Order placed:', result)

      setAlert({
        type: 'success',
        message: `✓ Order placed: ${side} ${qty} ${instrument.symbol} @ $${limitPrice.toFixed(2)}`
      })

      // Reset form
      setQuantity('')
      setPrice('')

      // Refresh balance
      const data = await getBalance()
      setUserBalance(data.usd_balance || 0)
      onBalanceUpdate(data.usd_balance || 0)

    } catch (err) {
      console.error('Order placement error:', err)
      setAlert({
        type: 'error',
        message: 'Failed to place order: ' + err.message
      })
    }

    setLoading(false)
  }

  /**
   * Auto-fill price with current market price
   */
  const handleMarketPrice = () => {
    setPrice(instrument.current_price.toFixed(2))
  }

  return (
    <div className="instrument-detail">
      {/* Header with back button */}
      <div className="detail-header">
        <button onClick={onBack} className="btn-back">
          ← Back to Instruments
        </button>
        <h2>
          {instrument.symbol} - {instrument.name}
        </h2>
        <span className="current-price">
          ${instrument.current_price.toFixed(2)}
        </span>
      </div>

      {/* Alerts */}
      {alert && (
        <Alert {...alert} onClose={() => setAlert(null)} />
      )}

      <div className="detail-container">
        {/* Left side: Chart and order book */}
        <div className="detail-left">
          {/* Chart */}
          <div className="chart-container">
            <Chart instrument={instrument} />
          </div>

          {/* Order book */}
          <div className="orderbook-container">
            <OrderBook instrument={instrument} />
          </div>
        </div>

        {/* Right side: Order form */}
        <div className="detail-right">
          <div className="order-panel">
            <h3>Place Order</h3>

            {/* Side selector (BUY/SELL) */}
            <div className="side-selector">
              <button
                className={`side-btn buy ${side === 'BUY' ? 'active' : ''}`}
                onClick={() => setSide('BUY')}
              >
                🟢 BUY
              </button>
              <button
                className={`side-btn sell ${side === 'SELL' ? 'active' : ''}`}
                onClick={() => setSide('SELL')}
              >
                🔴 SELL
              </button>
            </div>

            {/* Order type selector */}
            <div className="order-type">
              <label>
                <input
                  type="radio"
                  value="limit"
                  checked={orderType === 'limit'}
                  onChange={(e) => setOrderType(e.target.value)}
                />
                Limit Order
              </label>
              <label>
                <input
                  type="radio"
                  value="market"
                  checked={orderType === 'market'}
                  onChange={(e) => setOrderType(e.target.value)}
                />
                Market Order
              </label>
            </div>

            {/* Order form */}
            <form onSubmit={handlePlaceOrder}>
              {/* Quantity */}
              <div className="form-group">
                <label>Quantity ({instrument.symbol})</label>
                <input
                  type="number"
                  step="0.01"
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                  placeholder="0.00"
                  disabled={loading}
                />
              </div>

              {/* Price (only for limit orders) */}
              {orderType === 'limit' && (
                <div className="form-group">
                  <label>Price (USD)</label>
                  <div className="price-input-group">
                    <input
                      type="number"
                      step="0.01"
                      value={price}
                      onChange={(e) => setPrice(e.target.value)}
                      placeholder="0.00"
                      disabled={loading}
                    />
                    <button
                      type="button"
                      onClick={handleMarketPrice}
                      className="btn-market"
                      disabled={loading}
                    >
                      Market
                    </button>
                  </div>
                </div>
              )}

              {/* Total cost estimation */}
              {quantity && price && orderType === 'limit' && (
                <div className="total-cost">
                  <span>Total: ${(parseFloat(quantity) * parseFloat(price)).toFixed(2)}</span>
                </div>
              )}

              {/* Balance info */}
              <div className="balance-info">
                <span className="label">Available Balance:</span>
                <span className="amount">${userBalance.toFixed(2)}</span>
              </div>

              {/* Submit button */}
              <button
                type="submit"
                className={`btn-order ${side.toLowerCase()}`}
                disabled={loading || !quantity}
              >
                {loading ? (
                  <>
                    <Spinner /> Processing...
                  </>
                ) : (
                  `${side} ${instrument.symbol}`
                )}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}