import { useState, useEffect } from 'react'
import { getHoldings, getBalance, getCostBasis } from '../../services/trading'
import { getMarketData } from '../../services/marketdata'
import './Dashboard.css'

const RAW_DIVISOR = 10_000

export default function Holdings({ onBack }) {
  const [holdings, setHoldings]       = useState([])
  const [usdBalance, setUsdBalance]   = useState(0)
  const [loading, setLoading]         = useState(true)
  const [error, setError]             = useState(null)

  useEffect(() => {
    let cancelled = false

    const fetchAll = async (isFirst) => {
      try {
        const [holdingsData, balanceData, costBasis] = await Promise.all([
          getHoldings(),
          getBalance(),
          getCostBasis(),
        ])
        if (cancelled) return

        setUsdBalance((parseFloat(balanceData.balance) || 0) / RAW_DIVISOR)

        const items = holdingsData.holdings ?? []
        const enriched = await Promise.all(
          items.map(async (h) => {
            const mkt = await getMarketData(h.asset).catch(() => ({ current_price: 0 }))
            const qty      = parseFloat(h.quantity) / 100
            const price    = mkt.current_price || 0
            const avgCost  = costBasis[h.asset] ?? 0
            const pnlPct   = avgCost > 0 ? ((price - avgCost) / avgCost) * 100 : null
            return {
              asset: h.asset,
              quantity: qty,
              current_price: price,
              avg_cost: avgCost,
              pnl_pct: pnlPct,
              value: qty * price,
            }
          })
        )
        if (cancelled) return

        setHoldings(enriched)
        setError(null)
      } catch (err) {
        if (!cancelled) setError(err.message)
      }
      if (!cancelled && isFirst) setLoading(false)
    }

    fetchAll(true)
    const iv = setInterval(() => fetchAll(false), 3000)
    return () => { cancelled = true; clearInterval(iv) }
  }, [])

  const assetsValue      = holdings.reduce((sum, h) => sum + h.value, 0)
  const totalPortfolio   = usdBalance + assetsValue

  return (
    <div className="holdings">
      <div className="holdings-header">
        <button onClick={onBack} className="btn-back">← Back</button>
        <h2>My Portfolio</h2>
      </div>

      {error && <div className="error-message">Failed to load portfolio: {error}</div>}

      {loading ? (
        <div className="loading">Loading portfolio...</div>
      ) : (
        <div className="holdings-container">
          <div className="holdings-summary">
            <div className="summary-item">
              <span className="label">USD Balance</span>
              <span className="value">${usdBalance.toFixed(2)}</span>
            </div>
            <div className="summary-item">
              <span className="label">Assets Value</span>
              <span className="value">${assetsValue.toFixed(2)}</span>
            </div>
            <div className="summary-item positive">
              <span className="label">Total Portfolio</span>
              <span className="value">${totalPortfolio.toFixed(2)}</span>
            </div>
          </div>

          {holdings.length === 0 ? (
            <div className="empty-state">
              <p>No instrument holdings yet.</p>
              <button onClick={onBack} className="btn-primary">Start Trading</button>
            </div>
          ) : (
            <div className="holdings-table">
              <div className="table-header" style={{ gridTemplateColumns: '1fr 1fr 1fr 1fr 1fr 1fr' }}>
                <span>Asset</span>
                <span>Quantity</span>
                <span>Avg Cost</span>
                <span>Price</span>
                <span>P&amp;L %</span>
                <span>Value (USD)</span>
              </div>
              {holdings.map(h => {
                const hasCost = h.avg_cost > 0
                const pnlPos  = h.pnl_pct !== null && h.pnl_pct >= 0
                return (
                  <div key={h.asset} className="table-row" style={{ gridTemplateColumns: '1fr 1fr 1fr 1fr 1fr 1fr' }}>
                    <span className="symbol">{h.asset}</span>
                    <span>{h.quantity.toFixed(4)}</span>
                    <span>{hasCost ? `$${h.avg_cost.toFixed(2)}` : '—'}</span>
                    <span>${h.current_price.toFixed(2)}</span>
                    <span className={h.pnl_pct !== null ? (pnlPos ? 'positive' : 'negative') : ''}>
                      {h.pnl_pct !== null
                        ? `${pnlPos ? '+' : ''}${h.pnl_pct.toFixed(2)}%`
                        : '—'}
                    </span>
                    <span>${h.value.toFixed(2)}</span>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
