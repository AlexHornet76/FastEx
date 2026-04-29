import { useState } from 'react'
import { useAuth } from '../../hooks/useAuth'
import InstrumentsList from './Instruments'
import InstrumentDetail from './InstrumentDetails'
import Holdings from './Holdings'
import Header from './Header'
import './Dashboard.css'

/**
 * Dashboard Component
 * 
 * Main trading interface showing:
 * 1. Instruments list (searchable)
 * 2. Instrument detail (when clicked - place orders, view chart)
 * 3. Holdings (user's owned instruments)
 * 4. Balance (account balance)
 * 
 * Views:
 * - 'list': Show all instruments
 * - 'detail': Show specific instrument + trading interface
 * - 'holdings': Show user's holdings
 */
export default function Dashboard() {
  const { user, logout } = useAuth()
  const [view, setView] = useState('list') // 'list' | 'detail' | 'holdings'
  const [selectedInstrument, setSelectedInstrument] = useState(null)
  const [balance, setBalance] = useState(0)

  /**
   * Handle instrument selection
   * Load instrument details and switch to detail view
   */
  const handleSelectInstrument = (instrument) => {
    setSelectedInstrument(instrument)
    setView('detail')
  }

  /**
   * Handle back from detail view
   */
  const handleBackToList = () => {
    setSelectedInstrument(null)
    setView('list')
  }

  /**
   * Handle holdings view
   */
  const handleViewHoldings = () => {
    setView('holdings')
  }

  /**
   * Handle back from holdings
   */
  const handleBackFromHoldings = () => {
    setView('list')
  }

  return (
    <div className="dashboard">
      {/* Header with user info and balance */}
      <Header 
        user={user} 
        balance={balance}
        onViewHoldings={handleViewHoldings}
        onLogout={logout}
      />

      {/* Main content */}
      <div className="dashboard-content">
        {view === 'list' && (
          <InstrumentsList
            onSelectInstrument={handleSelectInstrument}
            onBalanceUpdate={setBalance}
          />
        )}

        {view === 'detail' && (
          <InstrumentDetail
            instrument={selectedInstrument}
            onBack={handleBackToList}
            balance={balance}
            onBalanceUpdate={setBalance}
          />
        )}

        {view === 'holdings' && (
          <Holdings
            onBack={handleBackFromHoldings}
          />
        )}
      </div>
    </div>
  )
}