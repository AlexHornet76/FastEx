import axios from 'axios'

/**
 * Create Axios instance with base configuration
 * - baseURL points to your Go backend
 * - Automatically adds JWT to all requests
 */
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  headers: {
    'Content-Type': 'application/json'
  }
})

/**
 * Request interceptor: Add JWT token to every request
 * This runs BEFORE each request is sent
 */
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('authToken')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

/**
 * Response interceptor: Handle errors globally
 * If 401 (unauthorized), user is logged out
 */
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Token expired or invalid
      localStorage.removeItem('authToken')
      window.location.href = '/'
    }
    return Promise.reject(error)
  }
)

export default api