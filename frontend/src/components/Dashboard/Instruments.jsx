import { useState, useEffect, useMemo } from 'react'
import { getInstruments } from '../../services/trading'
import { getMarketData } from '../../services/marketdata'
import './Dashboard.css'

export default function InstrumentsList({ onSelectInstrument }) {
  const [instrumentDefs, setInstrumentDefs] = useState([])   // lista stabila (fetch o data)
  const [prices, setPrices]                 = useState({})   // { symbol: { current_price, change_24h } }
  const [searchTerm, setSearchTerm]         = useState('')
  const [loading, setLoading]               = useState(true)
  const [error, setError]                   = useState(null)

  // Fetch instrument list once (definitions don't change)
  useEffect(() => {
    getInstruments()
      .then(data => { setInstrumentDefs(data); setError(null) })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  // Poll only prices, silently (no loading flash)
  useEffect(() => {
    if (instrumentDefs.length === 0) return
    let cancelled = false

    const fetchPrices = async () => {
      const results = await Promise.all(
        instrumentDefs.map(async (inst) => {
          try {
            const mkt = await getMarketData(inst.symbol)
            return [inst.symbol, { current_price: mkt.current_price || 0, change_24h: mkt.change_24h || 0 }]
          } catch {
            return [inst.symbol, { current_price: 0, change_24h: 0 }]
          }
        })
      )
      if (!cancelled) setPrices(Object.fromEntries(results))
    }

    fetchPrices()
    const iv = setInterval(fetchPrices, 3000)
    return () => { cancelled = true; clearInterval(iv) }
  }, [instrumentDefs])

  // Computed: combine defs + prices, then filter by search
  const instruments = useMemo(() =>
    instrumentDefs.map(inst => ({
      ...inst,
      ...prices[inst.symbol],
    }))
  , [instrumentDefs, prices])

  const filtered = useMemo(() => {
    const term = searchTerm.trim().toLowerCase()
    if (!term) return instruments
    return instruments.filter(inst =>
      inst.symbol.toLowerCase().includes(term) ||
      inst.name.toLowerCase().includes(term)
    )
  }, [instruments, searchTerm])

  if (loading) return (
    <div className="instruments-container">
      <div className="loading">Loading instruments...</div>
    </div>
  )

  return (
    <div className="instruments-container">
      <div className="instruments-header">
        <h2>Trading Instruments</h2>
        <div className="search-box">
          <input
            type="text"
            placeholder="Search instruments (BTC, AAPL, etc.)"
            value={searchTerm}
            onChange={e => setSearchTerm(e.target.value)}
            className="search-input"
          />
        </div>
      </div>

      {error && (
        <div className="error-message">✗ Failed to load instruments: {error}</div>
      )}

      <div className="instruments-grid">
        {filtered.length === 0 ? (
          <div className="no-results">No instruments found matching "{searchTerm}"</div>
        ) : (
          filtered.map(instrument => (
            <div
              key={instrument.id}
              className="instrument-card"
              onClick={() => onSelectInstrument(instrument)}
            >
              <div className="card-header">
                <span className="symbol">{instrument.symbol}</span>
                <span className="name">{instrument.name}</span>
              </div>
              <div className="card-price">
                ${(instrument.current_price || 0).toFixed(2)}
              </div>
              <div className={`card-change ${(instrument.change_24h || 0) >= 0 ? 'positive' : 'negative'}`}>
                {(instrument.change_24h || 0) >= 0 ? '+' : ''}{(instrument.change_24h || 0).toFixed(2)}%
              </div>
              <div className="card-hint">Click to trade →</div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
