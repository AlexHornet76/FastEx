import { useState, useEffect } from 'react'
import { getInstruments } from '../../services/trading'
import { getMarketData } from '../../services/marketdata'
import './Dashboard.css'

export default function InstrumentsList({ onSelectInstrument, onBalanceUpdate }) {
  const [instruments, setInstruments] = useState([])
  const [filteredInstruments, setFilteredInstruments] = useState([])
  const [searchTerm, setSearchTerm] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  /**
   * Load instruments and enrich with market data
   */
  useEffect(() => {
    const fetchInstruments = async () => {
      try {
        setLoading(true)
        const data = await getInstruments()
        
        // Enrich with market data (prices)
        const enriched = await Promise.all(
          data.map(async (inst) => {
            try {
              const marketData = await getMarketData(inst.symbol)
              return {
                ...inst,
                current_price: marketData.current_price || 0,
                change_24h: marketData.change_24h || 0,
                volume_24h: marketData.volume_24h || 0
              }
            } catch (err) {
              console.warn(`Failed to get market data for ${inst.symbol}:`, err)
              // Return instrument with placeholder prices
              return {
                ...inst,
                current_price: 0,
                change_24h: 0,
                volume_24h: 0
              }
            }
          })
        )

        setInstruments(enriched)
        setFilteredInstruments(enriched)
        setError(null)
      } catch (err) {
        console.error('Failed to fetch instruments:', err)
        setError(err.message)
      }
      setLoading(false)
    }

    fetchInstruments()
    // Refresh every 10 seconds
    const interval = setInterval(fetchInstruments, 10000)
    return () => clearInterval(interval)
  }, [])

  const handleSearch = (term) => {
    setSearchTerm(term)
    if (!term.trim()) {
      setFilteredInstruments(instruments)
      return
    }

    const filtered = instruments.filter(inst =>
      inst.symbol.toLowerCase().includes(term.toLowerCase()) ||
      inst.name.toLowerCase().includes(term.toLowerCase())
    )
    setFilteredInstruments(filtered)
  }

  if (loading) {
    return (
      <div className="instruments-container">
        <div className="loading">Loading instruments...</div>
      </div>
    )
  }

  return (
    <div className="instruments-container">
      <div className="instruments-header">
        <h2>Trading Instruments</h2>
        <div className="search-box">
          <input
            type="text"
            placeholder="🔍 Search instruments (BTC, AAPL, etc.)"
            value={searchTerm}
            onChange={(e) => handleSearch(e.target.value)}
            className="search-input"
          />
        </div>
      </div>

      {error && (
        <div className="error-message">
          ✗ Failed to load instruments: {error}
        </div>
      )}

      <div className="instruments-grid">
        {filteredInstruments.length === 0 ? (
          <div className="no-results">
            No instruments found matching "{searchTerm}"
          </div>
        ) : (
          filteredInstruments.map((instrument) => (
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
                <span className="price">
                  ${(instrument.current_price || 0).toFixed(2)}
                </span>
              </div>

              <div className={`card-change ${(instrument.change_24h || 0) >= 0 ? 'positive' : 'negative'}`}>
                {(instrument.change_24h || 0) >= 0 ? '📈' : '📉'} {(instrument.change_24h || 0).toFixed(2)}%
              </div>

              <div className="card-hint">
                Click to trade →
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}