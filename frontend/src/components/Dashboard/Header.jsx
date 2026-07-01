import { useEffect, useState } from 'react'
import { getBalance } from '../../services/trading'
import './Dashboard.css'

// Raw balance unit → display dollars
const RAW_DIVISOR = 10_000

export default function Header({ user, balance, onViewHoldings, onLogout }) {
  const [displayBalance, setDisplayBalance] = useState(0)

  useEffect(() => {
    const fetchBalance = async () => {
      try {
        const data = await getBalance()
        setDisplayBalance((parseFloat(data.balance) || 0) / RAW_DIVISOR)
      } catch (err) {
        console.error('Failed to fetch balance:', err)
      }
    }

    fetchBalance()
    const interval = setInterval(fetchBalance, 3000)
    return () => clearInterval(interval)
  }, [balance])

  return (
    <header className="dashboard-header">
      <div className="header-left">
        <h1 className="logo">⚡ FastEx</h1>
      </div>

      <div className="header-center">
        <div className="balance-display">
          <span className="balance-label">USD Balance</span>
          <span className="balance-amount">${displayBalance.toFixed(2)}</span>
        </div>
      </div>

      <div className="header-right">
        <button onClick={onViewHoldings} className="btn-secondary" title="View your holdings">
          💼 My Portfolio
        </button>

        <span className="user-info">{user?.username}</span>

        <button onClick={onLogout} className="btn-logout">
          🔓 Logout
        </button>
      </div>
    </header>
  )
}
