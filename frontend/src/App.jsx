import { useState } from 'react'
import { AuthProvider } from './components/Auth/AuthContext'
import { useAuth } from './hooks/useAuth'
import Register from './components/Auth/Register'
import Login from './components/Auth/Login'
import './App.css'

/**
 * AppContent - Main app logic (inside AuthProvider)
 * Shows Register, Login, or Dashboard based on auth state
 */
function AppContent() {
  const { isAuthenticated, loading } = useAuth()
  // Track which auth screen to show
  const [authMode, setAuthMode] = useState('login') // 'login' | 'register'

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
        <p>Loading...</p>
      </div>
    )
  }

  // Not authenticated - show Login or Register
  if (!isAuthenticated) {
    return authMode === 'register' ? (
      <Register
        onSwitchToLogin={() => setAuthMode('login')}
      />
    ) : (
      <Login
        onSwitchToRegister={() => setAuthMode('register')}
      />
    )
  }

  // Authenticated - show Dashboard placeholder
  return (
    <div className="dashboard-placeholder">
      <h1>🎉 Welcome to FastEx!</h1>
      <p>Dashboard coming soon...</p>
    </div>
  )
}

/**
 * App - Root component
 * Wraps everything in AuthProvider to provide global auth state
 */
export default function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  )
}