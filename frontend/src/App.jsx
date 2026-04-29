import { useState } from 'react'
import { AuthProvider } from './components/Auth/AuthContext'
import { useAuth } from './hooks/useAuth'
import Register from './components/Auth/Register'
import Login from './components/Auth/Login'
import Dashboard from './components/Dashboard/Dashboard'
import './App.css'

/**
 * AppContent - Main app logic (inside AuthProvider)
 * Shows Register, Login, or Dashboard based on auth state
 */
function AppContent() {
  const { isAuthenticated, loading } = useAuth()
  const [authMode, setAuthMode] = useState('login') // 'login' | 'register'

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
        <p>Loading...</p>
      </div>
    )
  }

  // User is authenticated - show Dashboard
  if (isAuthenticated) {
    return <Dashboard />
  }

  // Not authenticated - show Login or Register
  if (authMode === 'register') {
    return (
      <Register
        onSwitchToLogin={() => setAuthMode('login')}
      />
    )
  }

  return (
    <Login
      onSwitchToRegister={() => setAuthMode('register')}
    />
  )
}

/**
 * App - Root component
 * Wraps everything in AuthProvider
 */
export default function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  )
}