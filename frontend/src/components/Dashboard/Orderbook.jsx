import { useEffect, useState } from 'react'
import { getOrderbook } from '../../services/trading'
import './Dashboard.css'

export default function OrderBook({ instrument }) {
  const [orderbook, setOrderbook] = useState({ bids: [], asks: [] })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchOrderbook = async () => {
      try {
        const data = await getOrderbook(instrument.symbol)
        setOrderbook(data)
      } catch (err) {
        console.error('Failed to fetch orderbook:', err)
      }
      setLoading(false)
    }

    fetchOrderbook()
    const interval = setInterval(fetchOrderbook, 2000)
    return () => clearInterval(interval)
  }, [instrument])

  if (loading) {
    return <div className="orderbook">Loading orderbook...</div>
  }

  // Backend prices: int64 cents → divide by 100 for USD display
  // Backend quantities: int64 in 100ths units → divide by 100 for display
  const fmtPrice = (p) => (p / 100).toFixed(2)
  const fmtQty   = (q) => (q / 100).toFixed(2)

  return (
    <div className="orderbook">
      <h4>Order Book</h4>

      <div className="orderbook-grid">
        <div className="orderbook-side">
          <div className="side-header sell">Asks (Sell)</div>
          <div className="orderbook-table">
            <div className="table-header">
              <span>Price ($)</span>
              <span>Size</span>
            </div>
            {(orderbook.asks || []).slice(0, 8).map((ask, idx) => (
              <div key={idx} className="table-row sell">
                <span>${fmtPrice(ask.price)}</span>
                <span>{fmtQty(ask.quantity)}</span>
              </div>
            ))}
            {(orderbook.asks || []).length === 0 && (
              <div className="table-row" style={{ color: 'var(--text-tertiary)', justifyContent: 'center' }}>
                No asks
              </div>
            )}
          </div>
        </div>

        <div className="orderbook-side">
          <div className="side-header buy">Bids (Buy)</div>
          <div className="orderbook-table">
            <div className="table-header">
              <span>Price ($)</span>
              <span>Size</span>
            </div>
            {(orderbook.bids || []).slice(0, 8).map((bid, idx) => (
              <div key={idx} className="table-row buy">
                <span>${fmtPrice(bid.price)}</span>
                <span>{fmtQty(bid.quantity)}</span>
              </div>
            ))}
            {(orderbook.bids || []).length === 0 && (
              <div className="table-row" style={{ color: 'var(--text-tertiary)', justifyContent: 'center' }}>
                No bids
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
