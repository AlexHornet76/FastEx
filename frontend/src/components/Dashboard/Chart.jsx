import { useEffect, useRef, useState } from 'react'
import { getCandles } from '../../services/marketdata'
import './Dashboard.css'

const SVG_H       = 280   // chart height in px
const PAD_L       = 62    // Y-axis width (stays fixed)
const PAD_R       = 12
const PAD_T       = 12
const PAD_B       = 20
const CANDLE_SLOT = 16    // px per candle (fixed width, not stretched)
const CANDLE_W    = 9     // body width

export default function Chart({ instrument }) {
  const [candles, setCandles] = useState([])
  const [loading, setLoading]  = useState(true)
  const [containerW, setContainerW] = useState(600)
  const scrollRef = useRef(null)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      const data = await getCandles(instrument.symbol, 200)
      if (!cancelled) { setCandles(data); setLoading(false) }
    }
    load()
    const iv = setInterval(load, 1500)
    return () => { cancelled = true; clearInterval(iv) }
  }, [instrument.symbol])

  // Track container width so candles stay right-aligned when few exist
  useEffect(() => {
    if (!scrollRef.current) return
    const ro = new ResizeObserver(([e]) => setContainerW(e.contentRect.width))
    ro.observe(scrollRef.current)
    return () => ro.disconnect()
  }, [])

  // Scroll to the newest (rightmost) candle when count changes
  useEffect(() => {
    if (scrollRef.current)
      scrollRef.current.scrollLeft = scrollRef.current.scrollWidth
  }, [candles.length])

  if (loading) return (
    <div className="chart">
      <div className="chart-header"><h4>{instrument.symbol} Chart</h4></div>
      <div className="chart-empty">Loading...</div>
    </div>
  )

  if (candles.length === 0) return (
    <div className="chart">
      <div className="chart-header"><h4>{instrument.symbol} Chart</h4></div>
      <div className="chart-empty">No trades yet — place matching buy/sell orders to see price action.</div>
    </div>
  )

  const prices     = candles.flatMap(c => [c.high, c.low])
  const minP       = Math.min(...prices)
  const maxP       = Math.max(...prices)
  const padPct     = (maxP - minP) * 0.08 || 1
  const rangeMin   = minP - padPct
  const rangeMax   = maxP + padPct
  const priceRange = rangeMax - rangeMin

  const drawH    = SVG_H - PAD_T - PAD_B
  const candlesW = candles.length * CANDLE_SLOT + PAD_R
  // SVG is at least as wide as the container so candles stay right-aligned
  const svgW     = Math.max(candlesW, containerW)
  const leftPad  = svgW - candlesW  // empty space on the left

  // Y is shared between both SVGs (same scale)
  const toY = p => PAD_T + drawH * (1 - (p - rangeMin) / priceRange)
  // X is offset by leftPad so newest candle always appears at the right edge
  const toX = i => leftPad + i * CANDLE_SLOT + CANDLE_SLOT / 2

  const gridPrices = Array.from({ length: 5 }, (_, i) => rangeMin + priceRange * (i / 4))

  return (
    <div className="chart">
      <div className="chart-header">
        <h4>{instrument.symbol} Chart</h4>
        <span style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>
          {candles.length} candles · 2 sec
        </span>
      </div>

      <div className="chart-body" style={{ height: SVG_H }}>

        {/* Fixed Y-axis — doesn't scroll */}
        <svg className="chart-yaxis" width={PAD_L} height={SVG_H}>
          <rect x={0} y={0} width={PAD_L} height={SVG_H} fill="var(--bg-secondary)" />
          {gridPrices.map((p, i) => (
            <g key={i}>
              <line x1={PAD_L - 4} y1={toY(p)} x2={PAD_L} y2={toY(p)}
                stroke="#444" strokeWidth={0.8} />
              <text x={PAD_L - 7} y={toY(p) + 4} textAnchor="end" fill="#666" fontSize={9}>
                ${p.toFixed(0)}
              </text>
            </g>
          ))}
        </svg>

        {/* Scrollable candle area */}
        <div className="chart-scroll" ref={scrollRef}>
          <svg width={svgW} height={SVG_H}>
            {/* Grid lines */}
            {gridPrices.map((p, i) => (
              <line key={i}
                x1={0} y1={toY(p)} x2={svgW} y2={toY(p)}
                stroke="#2a2a2a" strokeWidth={0.8} strokeDasharray="3,3" />
            ))}

            {/* Candles */}
            {candles.map((c, i) => {
              const x     = toX(i)
              const isUp  = c.close >= c.open
              const color = isUp ? '#26a69a' : '#ef5350'
              const top   = toY(Math.max(c.open, c.close))
              const bot   = toY(Math.min(c.open, c.close))
              const bodyH = Math.max(1, bot - top)
              return (
                <g key={i}>
                  <line x1={x} y1={toY(c.high)} x2={x} y2={toY(c.low)}
                    stroke={color} strokeWidth={1} />
                  <rect x={x - CANDLE_W / 2} y={top} width={CANDLE_W} height={bodyH}
                    fill={color} />
                </g>
              )
            })}
          </svg>
        </div>

      </div>
    </div>
  )
}
