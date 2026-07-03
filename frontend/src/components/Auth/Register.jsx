import { useState } from 'react'
import { register } from '../../services/auth'
import { generateKeyPair } from '../../utils/crypto'
import Alert from '../Common/Alert'
import Spinner from '../Common/Spinner'
import './Auth.css'

/**
 * Register Component
 * 
 * User flow:
 * 1. Enter username
 * 2. Generate Ed25519 keypair (locally, never sent to server)
 * 3. Show public key for inspection
 * 4. Send {username, public_key_hex} to backend
 * 5. User downloads/saves private key
 * 6. Redirect to login
 * 
 * @param {function} onSwitchToLogin - Callback to switch to login screen
 */
export default function Register({ onSwitchToLogin }) {
  // Form state
  const [username, setUsername] = useState('')
  
  // Key generation state
  const [keyPair, setKeyPair] = useState(null)
  
  // UI state
  const [loading, setLoading] = useState(false)
  const [alert, setAlert] = useState(null)

  /**
   * Generate Ed25519 keypair
   * Runs locally - private key never leaves device
   */
  const handleGenerateKeys = async () => {
    setLoading(true)
    setAlert(null)
    
    try {
      const pair = await generateKeyPair()
      setKeyPair(pair)
      setAlert({
        type: 'success',
        message: '✓ Key pair generated successfully! Download your private key after registration.'
      })
    } catch (err) {
      setAlert({
        type: 'error',
        message: 'Failed to generate key pair: ' + err.message
      })
    }
    
    setLoading(false)
  }

  /**
   * Submit registration form
   * Sends {username, public_key_hex} to backend
   */
  const handleRegister = async (e) => {
    e.preventDefault()
    
    // Validation
    if (!username.trim()) {
      setAlert({ type: 'error', message: 'Please enter a username' })
      return
    }
    
    if (!keyPair) {
      setAlert({ type: 'error', message: 'Please generate a key pair first' })
      return
    }

    setLoading(true)
    setAlert(null)

    try {
      // Call backend
      await register(username, keyPair.publicKey)
      
      // Success - show download alert
      setAlert({
        type: 'success',
        message: 'Registration successful! Download your private key and keep it safe!',
        download: keyPair.privateKey // User can download private key
      })
      
      // Redirect to login after 2 seconds
      setTimeout(() => {
        onSwitchToLogin()
      }, 2000)
      
    } catch (err) {
      setAlert({
        type: 'error',
        message: 'Registration failed: ' + err.message
      })
    }
    
    setLoading(false)
  }

  /**
   * Generate new keys (discard current pair)
   */
  const handleGenerateNewKeys = () => {
    setKeyPair(null)
    setAlert(null)
  }

  return (
    <div className="auth-container">
      <div className="auth-card">
        {/* Header */}
        <div className="auth-header">
          <h1>FastEx Registration</h1>
          <p className="auth-subtitle">Hardware-Authenticated Exchange</p>
        </div>

        {/* Alerts */}
        {alert && (
          <Alert 
            {...alert} 
            onClose={() => setAlert(null)} 
          />
        )}

        {/* Registration Form */}
        <form onSubmit={handleRegister}>
          
          {/* Username Input */}
          <div className="form-group">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Choose your username"
              disabled={loading}
              required
              minLength="3"
              maxLength="20"
              pattern="[a-zA-Z0-9_]+"
              title="Username can only contain letters, numbers, and underscores"
            />
          </div>

          {/* Key Pair Generation */}
          <div className="form-group">
            <label>Ed25519 Key Pair (Generated Locally)</label>
            <p className="form-help-text">
              ✓ Your private key is generated on your device and NEVER sent to our servers
            </p>
            
            {keyPair ? (
              // Show when keys are generated
              <div className="key-display">
                <div className="key-status">
                  <span className="status-icon">✓</span>
                  <span>Key pair generated successfully</span>
                </div>
                
                {/* Show public key for inspection */}
                <div className="key-preview">
                  <label>Your Public Key (can be shared):</label>
                  <textarea 
                    value={keyPair.publicKey}
                    readOnly
                    rows="3"
                  />
                </div>

                <button
                  type="button"
                  onClick={handleGenerateNewKeys}
                  className="btn btn-secondary"
                  disabled={loading}
                >
                  Generate New Keys
                </button>
              </div>
            ) : (
              // Show when keys not generated
              <button
                type="button"
                onClick={handleGenerateKeys}
                className="btn btn-secondary full-width"
                disabled={loading}
              >
                {loading ? (
                  <>
                    <Spinner /> Generating...
                  </>
                ) : (
                  'Generate Key Pair'
                )}
              </button>
            )}
          </div>

          {/* Submit Button */}
          <button
            type="submit"
            className="btn btn-primary full-width"
            disabled={loading || !keyPair}
          >
            {loading ? (
              <>
                <Spinner /> Registering...
              </>
            ) : (
              'Register'
            )}
          </button>

          {/* Private Key Warning */}
          <div className="warning-box">
            <strong>Important:</strong>
            <ul>
              <li>Download your private key after registration</li>
              <li>Store it somewhere safe (password manager, hardware wallet)</li>
              <li>Never share your private key with anyone</li>
              <li>We cannot recover a lost private key</li>
            </ul>
          </div>

        </form>

        {/* Switch to Login */}
        <div className="auth-footer">
          <p>Already have an account?</p>
          <button 
            onClick={onSwitchToLogin} 
            className="link-button"
          >
            Login here
          </button>
        </div>
      </div>
    </div>
  )
}