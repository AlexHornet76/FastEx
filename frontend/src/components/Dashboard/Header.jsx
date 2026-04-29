import { useEffect, useState } from 'react'
import { getBalance } from '../../services/trading'
import './Dashboard.css'

/**
 * Header Component
 * Shows user info, balance, and navigation buttons
 */
export default function Header({ user, balance, onViewHoldings, onLogout }) {
  const [loading, setLoading] = useState(true)
  const [balanceData, setBalanceData] = useState(0)

  useEffect(() => {
    const fetchBalance = async () => {
      try {
        const data = await getBalance()
        setBalanceData(data.usd_balance || 0)
      } catch (err) {
        console.error('Failed to fetch balance:', err)
      }
      setLoading(false)
    }

    fetchBalance()
    // Refresh balance every 30 seconds
    const interval = setInterval(fetchBalance, 30000)
    return () => clearInterval(interval)
  }, [])

  return (
    <header className="dashboard-header">
      <div className="header-left">
        <h1 className="logo">⚡ FastEx</h1>
      </div>

      <div className="header-center">
        <div className="balance-display">
          <span className="balance-label">USD Balance</span>
          <span className="balance-amount">
            ${balanceData.toFixed(2)}
          </span>
        </div>
      </div>

      <div className="header-right">
        <button 
          onClick={onViewHoldings}
          className="btn-secondary"
          title="View your holdings"
        >
          💼 My Holdings
        </button>
        
        <span className="user-info">
          {user?.username}
        </span>

        <button 
          onClick={onLogout}
          className="btn-logout"
        >
          🔓 Logout
        </button>
      </div>
    </header>
  )
}