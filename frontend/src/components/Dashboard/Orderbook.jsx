import { useEffect, useState } from 'react'
import { getOrderbook } from '../../services/trading'
import './Dashboard.css'

/**
 * OrderBook Component
 * Shows bids (buy orders) and asks (sell orders)
 */
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
    // Refresh every 2 seconds
    const interval = setInterval(fetchOrderbook, 2000)
    return () => clearInterval(interval)
  }, [instrument])

  if (loading) {
    return <div className="orderbook">Loading orderbook...</div>
  }

  return (
    <div className="orderbook">
      <h4>Order Book</h4>

      <div className="orderbook-container">
        {/* Asks (sell orders) - red */}
        <div className="orderbook-side">
          <div className="side-header sell">Asks (Sell)</div>
          <div className="orderbook-table">
            <div className="table-header">
              <span>Price</span>
              <span>Size</span>
            </div>
            {(orderbook.asks || []).slice(0, 5).map((ask, idx) => (
              <div key={idx} className="table-row sell">
                <span>${ask.price.toFixed(2)}</span>
                <span>{ask.quantity.toFixed(4)}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Bids (buy orders) - green */}
        <div className="orderbook-side">
          <div className="side-header buy">Bids (Buy)</div>
          <div className="orderbook-table">
            <div className="table-header">
              <span>Price</span>
              <span>Size</span>
            </div>
            {(orderbook.bids || []).slice(0, 5).map((bid, idx) => (
              <div key={idx} className="table-row buy">
                <span>${bid.price.toFixed(2)}</span>
                <span>{bid.quantity.toFixed(4)}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}