import { useState } from 'react'
import { useAuth } from '../../hooks/useAuth'
import { requestChallenge, verifyChallenge } from '../../services/auth'
import { signMessage } from '../../utils/crypto'
import Alert from '../Common/Alert'
import Spinner from '../Common/Spinner'
import './Auth.css'

/**
 * Login Component
 * 
 * Hardware-backed authentication flow (Challenge-Response):
 * 1. User enters username
 * 2. User provides private key (file or paste)
 * 3. Frontend requests challenge from backend
 * 4. Frontend signs challenge with private key
 * 5. Frontend sends signature to backend for verification
 * 6. Backend returns JWT token
 * 7. User is authenticated and redirected to dashboard
 * 
 * Color palette:
 * - Background: #0e1117 (dark black)
 * - Cards: #1a1f2b (dark gray)
 * - Text: #e0e0e0 (light white)
 * - Accent: #2962ff (blue - buttons)
 * - Success: #26a69a (green - status)
 * - Error: #ef5350 (red - errors)
 * 
 * @param {function} onSwitchToRegister - Callback to switch to registration screen
 */
export default function Login({ onSwitchToRegister }) {
  const { login } = useAuth()

  // Form state
  const [username, setUsername] = useState('')
  const [privateKey, setPrivateKey] = useState('')

  // UI state
  const [loading, setLoading] = useState(false)
  const [alert, setAlert] = useState(null)
  const [step, setStep] = useState('input') // 'input' | 'signing'

  /**
   * Handle file input for private key
   * Allows user to select .txt file with private key
   */
  const handleImportKey = (e) => {
    const file = e.target.files[0]
    if (file) {
      const reader = new FileReader()
      reader.onload = (event) => {
        // Extract private key from file (should be hex string)
        const content = event.target.result.trim()
        setPrivateKey(content)
        setAlert({
          type: 'success',
          message: '✓ Private key imported successfully'
        })
      }
      reader.onerror = () => {
        setAlert({
          type: 'error',
          message: '✗ Failed to read file'
        })
      }
      reader.readAsText(file)
    }
  }

  /**
   * Clear private key from input
   */
  const handleClearKey = () => {
    setPrivateKey('')
    setAlert(null)
  }

  /**
   * Handle login form submission
   * 
   * Process:
   * 1. Validate inputs
   * 2. Request challenge from backend
   * 3. Sign challenge with private key locally
   * 4. Send signature to backend for verification
   * 5. Receive JWT token
   * 6. Save token to localStorage
   * 7. Redirect to dashboard
   */
const handleLogin = async (e) => {
  e.preventDefault()

  // Validation
  if (!username.trim()) {
    setAlert({ type: 'error', message: 'Please enter your username' })
    return
  }

  if (!privateKey.trim()) {
    setAlert({ type: 'error', message: 'Please provide your private key' })
    return
  }

  if (!/^[a-f0-9]{128}$/i.test(privateKey.trim())) {
    setAlert({
      type: 'error',
      message: 'Invalid private key format (should be 128 hex characters)'
    })
    return
  }

  setLoading(true)
  setAlert(null)
  setStep('signing')

  try {
    console.log('=== LOGIN PROCESS ===')
    console.log('Step 1: Requesting challenge for username:', username)

    // Step 1: Request challenge from backend
    const challengeResponse = await requestChallenge(username)
    const challenge = challengeResponse.challenge

    console.log('Step 2: Received challenge, signing locally...')

    // Step 2: Sign challenge with private key (done locally)
    const signature = await signMessage(privateKey.trim(), challenge)
    const timestamp = Math.floor(Date.now() / 1000)

    console.log('Step 3: Sending signed challenge to backend...')

    // Step 3: Verify challenge (send signature to backend)
    const verifyResponse = await verifyChallenge({
      username,
      challenge,
      signature,
      timestamp
    })

    console.log('Step 4: Backend verified signature, received JWT')

    // Step 4: Save token and user data to AuthContext
    const { token, user } = verifyResponse
    login(token, user)

    console.log('Step 5: Token saved, auth state updated')
    console.log('=== LOGIN SUCCESS ===')

    setAlert({
      type: 'success',
      message: '🎉 Login successful! Redirecting to dashboard...'
    })

    // IMPORTANT: Don't need to redirect manually - App.jsx will detect isAuthenticated change
    // The component tree will re-render and show Dashboard instead of Login

  } catch (err) {
    console.error('Login error:', err)
    console.log('=== LOGIN FAILED ===')

    setStep('input')
    setAlert({
      type: 'error',
      message: '✗ Login failed: ' + err.message
    })
  }

  setLoading(false)
}

  return (
    <div className="auth-container">
      <div className="auth-card">
        {/* Header */}
        <div className="auth-header">
          <h1>FastEx Login</h1>
          <p className="auth-subtitle">Hardware-Authenticated Exchange</p>
        </div>

        {/* Alerts */}
        {alert && (
          <Alert
            {...alert}
            onClose={() => setAlert(null)}
          />
        )}

        {/* Login form */}
        <form onSubmit={handleLogin}>

          {/* Username input */}
          <div className="form-group">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Enter your username"
              disabled={loading}
              required
            />
          </div>

          {/* Private key section */}
          <div className="form-group">
            <label htmlFor="privateKey">Private Key (Ed25519 hex)</label>
            <p className="form-help-text">
              ⚠️ Your private key will be used only to sign the challenge locally
            </p>

            {privateKey ? (
              // Show when private key is entered
              <div className="key-display">
                <div className="key-status">
                  <span className="status-icon">✓</span>
                  <span>Private key loaded (never sent to server)</span>
                </div>

                <div className="key-preview">
                  <label>Loaded private key preview:</label>
                  <textarea
                    value={privateKey.substring(0, 64) + '...'}
                    readOnly
                    rows="2"
                  />
                </div>

                <button
                  type="button"
                  onClick={handleClearKey}
                  className="btn btn-secondary"
                  disabled={loading}
                >
                  Use Different Key
                </button>
              </div>
            ) : (
              // Show when no private key
              <div className="key-input-section">
                {/* Text input for private key */}
                <textarea
                  id="privateKey"
                  value={privateKey}
                  onChange={(e) => setPrivateKey(e.target.value)}
                  placeholder="Paste your Ed25519 private key (128 hex characters)"
                  rows="4"
                  disabled={loading}
                  className="private-key-textarea"
                />

                {/* Or file upload */}
                <div className="file-upload-section">
                  <label htmlFor="keyFile" className="file-upload-label">
                    📁 Or upload private key file
                  </label>
                  <input
                    id="keyFile"
                    type="file"
                    onChange={handleImportKey}
                    accept=".txt,.key"
                    disabled={loading}
                    className="file-input"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Submit button */}
          <button
            type="submit"
            className="btn btn-primary full-width"
            disabled={loading || !username.trim() || !privateKey.trim()}
          >
            {loading ? (
              <>
                <Spinner /> {step === 'signing' ? 'Signing...' : 'Logging in...'}
              </>
            ) : (
              '🔓 Login'
            )}
          </button>

          {/* Security warning */}
          <div className="warning-box">
            <strong>🔒 Security Note:</strong>
            <ul>
              <li>Your private key is NEVER sent to our servers</li>
              <li>We only verify the signature you create</li>
              <li>This is hardware-backed Ed25519 authentication</li>
              <li>Clear your private key after login</li>
            </ul>
          </div>

        </form>

        {/* Footer - switch to register */}
        <div className="auth-footer">
          <p>Don't have an account?</p>
          <button
            onClick={onSwitchToRegister}
            className="link-button"
          >
            Register here
          </button>
        </div>
      </div>
    </div>
  )
}