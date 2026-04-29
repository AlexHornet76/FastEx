import { createContext, useState, useEffect } from 'react'
import { verifyToken } from '../../services/auth'

/**
 * AuthContext - Global authentication state
 * Shared across entire app
 * 
 * Usage:
 * const { isAuthenticated, user, login, logout } = useContext(AuthContext)
 */
export const AuthContext = createContext()

/**
 * AuthProvider - Wraps entire app to provide auth state
 * 
 * Features:
 * - Persists token to localStorage
 * - Verifies token on app startup
 * - Provides login/logout functions
 * - Handles 401 errors automatically
 * 
 * @param {ReactNode} children - App components to wrap
 */
export function AuthProvider({ children }) {
  const [auth, setAuth] = useState({
    token: localStorage.getItem('authToken'),
    user: null,
    isAuthenticated: !!localStorage.getItem('authToken'),
    loading: true // Initially loading to verify token
  })

  /**
   * On app startup, verify stored token is still valid
   */
  useEffect(() => {
    const verifyAuth = async () => {
      const token = localStorage.getItem('authToken')
      
      if (token) {
        try {
          // Check if token is still valid by calling backend
          const user = await verifyToken(token)
          setAuth({
            token,
            user,
            isAuthenticated: true,
            loading: false
          })
        } catch (err) {
          // Token expired or invalid
          console.log('Token verification failed:', err.message)
          localStorage.removeItem('authToken')
          setAuth({
            token: null,
            user: null,
            isAuthenticated: false,
            loading: false
          })
        }
      } else {
        // No token stored
        setAuth(prev => ({ ...prev, loading: false }))
      }
    }

    verifyAuth()
  }, [])

  /**
   * Login - Save token and user data
   * Called after successful authentication
   */
  const login = (token, user) => {
    localStorage.setItem('authToken', token)
    setAuth({
      token,
      user,
      isAuthenticated: true,
      loading: false
    })
  }

  /**
   * Logout - Clear token and user data
   */
  const logout = () => {
    localStorage.removeItem('authToken')
    setAuth({
      token: null,
      user: null,
      isAuthenticated: false,
      loading: false
    })
  }

  return (
    <AuthContext.Provider value={{ ...auth, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}