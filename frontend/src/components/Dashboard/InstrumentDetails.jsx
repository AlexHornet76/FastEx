import { useState, useEffect } from 'react'
import { getBalance, placeOrder, deposit } from '../../services/trading'
import { getMarketData } from '../../services/marketdata'
import OrderBook from './Orderbook'
import Chart from './Chart'
import Alert from '../Common/Alert'
import Spinner from '../Common/Spinner'
import './Dashboard.css'

// Raw balance unit → display dollars: usdValue = price_cents * qty_100ths = dollars * 10,000
const RAW_DIVISOR = 10_000

export default function InstrumentDetail({ instrument, onBack, balance, onBalanceUpdate }) {
  const [side, setSide] = useState('BUY')
  const [quantity, setQuantity] = useState('')
  const [price, setPrice] = useState('')
  const [orderType, setOrderType] = useState('limit')

  const [loading, setLoading] = useState(false)
  const [depositLoading, setDepositLoading] = useState(false)
  const [alert, setAlert] = useState(null)
  const [userBalance, setUserBalance] = useState(balance)
  const [livePrice, setLivePrice] = useState(instrument.current_price ?? 0)

  // Poll live price every 2 s so header + market button always reflect current price
  useEffect(() => {
    let cancelled = false
    const poll = async () => {
      const mkt = await getMarketData(instrument.symbol).catch(() => null)
      if (!cancelled && mkt && mkt.current_price > 0) setLivePrice(mkt.current_price)
    }
    poll()
    const iv = setInterval(poll, 2000)
    return () => { cancelled = true; clearInterval(iv) }
  }, [instrument.symbol])

  const refreshBalance = async () => {
    try {
      const data = await getBalance()
      const display = (parseFloat(data.balance) || 0) / RAW_DIVISOR
      setUserBalance(display)
      onBalanceUpdate(display)
    } catch (err) {
      console.error('Failed to fetch balance:', err)
    }
  }

  useEffect(() => {
    refreshBalance()
  }, []) // eslint-disable-line

  const handleDeposit = async () => {
    setDepositLoading(true)
    setAlert(null)
    try {
      await deposit()
      setAlert({ type: 'success', message: 'Demo account funded: +$100,000 USD added to your balance.' })
      await refreshBalance()
    } catch (err) {
      setAlert({ type: 'error', message: 'Deposit failed: ' + err.message })
    }
    setDepositLoading(false)
  }

  const handlePlaceOrder = async (e) => {
    e.preventDefault()

    const qty = parseFloat(quantity)
    const limitPrice = parseFloat(price) || livePrice

    if (!qty || qty <= 0) {
      setAlert({ type: 'error', message: 'Invalid quantity' })
      return
    }
    if (orderType === 'limit' && (!price || limitPrice <= 0)) {
      setAlert({ type: 'error', message: 'Invalid price' })
      return
    }

    // Balance check in display dollars
    const totalCost = qty * limitPrice
    if (side === 'BUY' && totalCost > userBalance) {
      setAlert({
        type: 'error',
        message: `Insufficient balance. Need $${totalCost.toFixed(2)}, have $${userBalance.toFixed(2)}`,
      })
      return
    }

    setLoading(true)
    setAlert(null)

    try {
      // Convert to engine int64 units:
      //   price  → cents         (× 100)
      //   qty    → 100ths units  (× 100)
      const result = await placeOrder({
        instrument: instrument.symbol,
        side,
        type: orderType.toUpperCase(),
        price: Math.round(limitPrice * 100),
        quantity: Math.round(qty * 100),
      })

      if (result.status === 'REJECTED') {
        setAlert({
          type: 'error',
          message: `Order rejected — no ${side === 'BUY' ? 'sell' : 'buy'} orders in the book. Try a limit order or run the seeder.`,
        })
      } else if (result.status === 'PARTIAL') {
        const filled = (result.filled_qty / 100).toFixed(4)
        setAlert({
          type: 'info',
          message: `Partially filled: ${filled} of ${qty.toFixed(2)} ${instrument.symbol} — insufficient book depth for the rest.`,
        })
      } else {
        setAlert({
          type: 'success',
          message: `Order filled: ${side} ${qty.toFixed(2)} ${instrument.symbol} @ $${limitPrice.toFixed(2)}`,
        })
      }

      setQuantity('')
      setPrice('')
      await refreshBalance()
    } catch (err) {
      console.error('Order error:', err)
      setAlert({ type: 'error', message: 'Failed to place order: ' + err.message })
    }

    setLoading(false)
  }

  const handleMarketPrice = () => {
    setPrice(livePrice.toFixed(2))
  }

  return (
    <div className="instrument-detail">
      <div className="detail-header">
        <button onClick={onBack} className="btn-back">← Back</button>
        <h2>{instrument.symbol} — {instrument.name}</h2>
        <span className="current-price">
          ${livePrice.toFixed(2)}
        </span>
      </div>

      {alert && <Alert {...alert} onClose={() => setAlert(null)} />}

      <div className="detail-container">
        <div className="detail-left">
          <div className="chart-container">
            <Chart instrument={instrument} />
          </div>

          <div className="orderbook-wrapper">
            <OrderBook instrument={instrument} />
          </div>
        </div>

        <div className="detail-right">
          <div className="order-panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h3>Place Order</h3>
              <button
                className="btn-secondary"
                style={{ fontSize: 11, padding: '4px 10px' }}
                onClick={handleDeposit}
                disabled={depositLoading}
                title="Add $100,000 demo USD + instrument balances"
              >
                {depositLoading ? 'Funding...' : '💰 Fund Demo'}
              </button>
            </div>

            <div className="side-selector">
              <button
                className={`side-btn buy ${side === 'BUY' ? 'active' : ''}`}
                onClick={() => setSide('BUY')}
              >
                BUY
              </button>
              <button
                className={`side-btn sell ${side === 'SELL' ? 'active' : ''}`}
                onClick={() => setSide('SELL')}
              >
                SELL
              </button>
            </div>

            <div className="order-type">
              <label>
                <input type="radio" value="limit" checked={orderType === 'limit'}
                  onChange={(e) => setOrderType(e.target.value)} />
                Limit
              </label>
              <label>
                <input type="radio" value="market" checked={orderType === 'market'}
                  onChange={(e) => setOrderType(e.target.value)} />
                Market
              </label>
            </div>

            <form onSubmit={handlePlaceOrder}>
              <div className="form-group">
                <label>Quantity ({instrument.symbol})</label>
                <input
                  type="number"
                  step="0.01"
                  min="0.01"
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                  placeholder="0.00"
                  disabled={loading}
                />
              </div>

              {orderType === 'limit' ? (
                <div className="form-group">
                  <label>Price (USD)</label>
                  <input
                    type="number"
                    step="0.01"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    placeholder="0.00"
                    disabled={loading}
                  />
                </div>
              ) : (
                <div className="form-group">
                  <label>Execution Price (live)</label>
                  <div className="market-price-live">${livePrice.toFixed(2)}</div>
                </div>
              )}

              {quantity && (orderType === 'market' || price) && (
                <div className="total-cost">
                  <span>
                    Est. Total: ${(
                      parseFloat(quantity || 0) *
                      (orderType === 'market' ? livePrice : parseFloat(price || 0))
                    ).toFixed(2)}
                  </span>
                </div>
              )}

              <div className="balance-info">
                <span className="label">Available USD:</span>
                <span className="amount">${userBalance.toFixed(2)}</span>
              </div>

              <button
                type="submit"
                className={`btn-order ${side.toLowerCase()}`}
                disabled={loading || !quantity}
              >
                {loading ? <><Spinner /> Processing...</> : `${side} ${instrument.symbol}`}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}
