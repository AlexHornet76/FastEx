import { useState, useEffect } from 'react'
import { getHoldings } from '../../services/trading'
import './Dashboard.css'

/**
 * Holdings Component
 * Shows user's owned instruments with:
 * - Quantity
 * - Average buy price
 * - Current price
 * - Profit/Loss
 */
export default function Holdings({ onBack }) {
  const [holdings, setHoldings] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    const fetchHoldings = async () => {
      try {
        setLoading(true)
        const data = await getHoldings()
        setHoldings(data.holdings ?? [])
        setError(null)
      } catch (err) {
        console.error('Failed to fetch holdings:', err)
        setError(err.message)
      }
      setLoading(false)
    }

    fetchHoldings()
  }, [])

  const totalValue = holdings.reduce((sum, h) => sum + (h.current_price * h.quantity), 0)
  const totalCost = holdings.reduce((sum, h) => sum + (h.average_price * h.quantity), 0)
  const totalPnL = totalValue - totalCost

  return (
    <div className="holdings">
      <div className="holdings-header">
        <button onClick={onBack} className="btn-back">
          ← Back
        </button>
        <h2>My Holdings</h2>
      </div>

      {error && (
        <div className="error-message">
          Failed to load holdings: {error}
        </div>
      )}

      {loading ? (
        <div className="loading">Loading holdings...</div>
      ) : holdings.length === 0 ? (
        <div className="empty-state">
          <p>You don't own any instruments yet</p>
          <button onClick={onBack} className="btn-primary">
            Start Trading
          </button>
        </div>
      ) : (
        <div className="holdings-container">
          {/* Summary */}
          <div className="holdings-summary">
            <div className="summary-item">
              <span className="label">Total Value</span>
              <span className="value">${totalValue.toFixed(2)}</span>
            </div>
            <div className="summary-item">
              <span className="label">Total Cost</span>
              <span className="value">${totalCost.toFixed(2)}</span>
            </div>
            <div className={`summary-item ${totalPnL >= 0 ? 'positive' : 'negative'}`}>
              <span className="label">Total P&L</span>
              <span className="value">
                {totalPnL >= 0 ? '📈' : '📉'} ${Math.abs(totalPnL).toFixed(2)}
                ({totalCost > 0 ? ((totalPnL / totalCost) * 100).toFixed(2) : '0.00'}%)
              </span>
            </div>
          </div>

          {/* Holdings table */}
          <div className="holdings-table">
            <div className="table-header">
              <span>Instrument</span>
              <span>Quantity</span>
              <span>Avg Buy Price</span>
              <span>Current Price</span>
              <span>Value</span>
              <span>P&L</span>
            </div>

            {holdings.map((holding) => {
              const value = holding.current_price * holding.quantity
              const cost = holding.average_price * holding.quantity
              const pnl = value - cost
              const pnlPercent = cost > 0 ? (pnl / cost) * 100 : 0

              return (
                <div key={holding.instrument} className="table-row">
                  <span className="symbol">{holding.instrument}</span>
                  <span>{Number(holding.quantity).toFixed(4)}</span>
                  <span>${Number(holding.average_price).toFixed(2)}</span>
                  <span>${Number(holding.current_price).toFixed(2)}</span>
                  <span>${value.toFixed(2)}</span>
                  <span className={pnl >= 0 ? 'positive' : 'negative'}>
                    {pnl >= 0 ? '📈' : '📉'} ${Math.abs(pnl).toFixed(2)} ({pnlPercent.toFixed(2)}%)
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}