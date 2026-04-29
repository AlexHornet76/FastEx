import { useEffect, useState } from 'react'
import './Dashboard.css'

/**
 * Chart Component
 * Simple candlestick chart visualization
 * 
 * For now: ASCII art candles
 * Future: Use TradingView Lightweight Charts or similar
 */
export default function Chart({ instrument }) {
  const [candles, setCandles] = useState([])

  useEffect(() => {
    // Generate sample candles (in real app, fetch from backend)
    const generateCandles = () => {
      const data = []
      let price = instrument.current_price
      
      for (let i = 0; i < 20; i++) {
        const change = (Math.random() - 0.5) * 10
        const open = price
        const close = price + change
        const high = Math.max(open, close) + Math.random() * 5
        const low = Math.min(open, close) - Math.random() * 5

        data.push({
          time: `${20 - i}h`,
          open,
          close,
          high,
          low,
          positive: close >= open
        })

        price = close
      }

      return data.reverse()
    }

    setCandles(generateCandles())
  }, [instrument])

  return (
    <div className="chart">
      <div className="chart-header">
        <h4>24h Chart</h4>
      </div>

      <div className="chart-body">
        {/* Simple ASCII candle representation */}
        <div className="candles-container">
          {candles.map((candle, idx) => {
            const heightPercent = ((candle.high - candle.low) / candle.high) * 100
            const openPercent = ((candle.open - candle.low) / (candle.high - candle.low)) * 100
            const closePercent = ((candle.close - candle.low) / (candle.high - candle.low)) * 100

            return (
              <div key={idx} className="candle-wrapper">
                <div 
                  className={`candle ${candle.positive ? 'up' : 'down'}`}
                  style={{
                    height: `${heightPercent}%`,
                    top: `${100 - heightPercent}%`
                  }}
                >
                  <div 
                    className="candle-body"
                    style={{
                      top: `${Math.max(openPercent, closePercent)}%`,
                      height: `${Math.abs(closePercent - openPercent)}%`
                    }}
                  />
                </div>
                <span className="candle-time">{candle.time}</span>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}