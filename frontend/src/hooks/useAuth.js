import { useContext } from 'react'
import { AuthContext } from '../components/Auth/AuthContext'

/**
 * useAuth - Hook to access auth state from anywhere
 * 
 * Usage:
 * const { user, isAuthenticated, login, logout } = useAuth()
 * 
 * Throws error if used outside AuthProvider
 */
export function useAuth() {
  const context = useContext(AuthContext)
  
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  
  return context
}