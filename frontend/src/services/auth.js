import api from './api'

/**
 * Register a new user with hardware-backed authentication
 * 
 * Process:
 * 1. User generates Ed25519 keypair locally (private key stays on device)
 * 2. Frontend sends username + public key to backend
 * 3. Backend stores public key in database
 * 4. User saves private key locally (never transmitted)
 * 
 * @param {string} username - Desired username
 * @param {string} publicKey - Public key in hex format (safe to share)
 * @returns {Promise<Object>} Server response with success status
 * @throws {Error} If registration fails (username taken, invalid format, etc)
 */
export async function register(username, publicKey) {
  try {
    console.log('Sending registration request:', {
      username,
      publicKeyLength: publicKey.length,
      publicKeyPreview: publicKey.substring(0, 20) + '...'
    })

    const response = await api.post('/auth/register', {
      username,
      public_key: publicKey
    })

    console.log('Registration response:', response.data)
    return response.data
  } catch (error) {
    console.error('Registration error:', {
      status: error.response?.status,
      data: error.response?.data,
      message: error.message
    })

    if (error.response?.data?.error) {
      throw new Error(error.response.data.error)
    }
    if (error.response?.data?.message) {
      throw new Error(error.response.data.message)
    }
    throw new Error('Registration failed: ' + error.message)
  }
}

/**
 * Request authentication challenge
 * 
 * Process:
 * 1. User enters username to login
 * 2. Frontend requests random challenge from backend
 * 3. Backend generates and stores challenge (expires in ~5 minutes)
 * 4. Frontend signs challenge with user's private key
 * 5. Frontend sends signature back to verify ownership
 * 
 * @param {string} username - Username to get challenge for
 * @returns {Promise<Object>} {challenge: "base64_string"}
 * @throws {Error} If user not found or server error
 */
export async function requestChallenge(username) {
  try {
    console.log('Requesting challenge for username:', username)

    const response = await api.post('/auth/challenge', { username })

    console.log('Received challenge from backend')
    return response.data
  } catch (error) {
    console.error('Challenge request error:', error)

    if (error.response?.data?.error) {
      throw new Error(error.response.data.error)
    }
    if (error.response?.data?.message) {
      throw new Error(error.response.data.message)
    }

    throw new Error('Failed to request challenge: ' + error.message)
  }
}

/**
 * Verify challenge and receive JWT token
 * 
 * Process:
 * 1. Client signs challenge with their private key
 * 2. Frontend sends signature + challenge to backend
 * 3. Backend verifies signature using stored public key
 * 4. Backend validates challenge hasn't expired
 * 5. Backend invalidates challenge (one-time use)
 * 6. Backend returns JWT token for authenticated requests
 * 
 * @param {Object} payload - Authentication data
 * @param {string} payload.username - Username
 * @param {string} payload.challenge - Original challenge (base64)
 * @param {string} payload.signature - Signed challenge (hex)
 * @param {string} payload.timestamp - Timestamp when signed (ISO 8601)
 * @returns {Promise<Object>} {token: "JWT_STRING", user: {id, username}}
 * @throws {Error} If signature verification fails or challenge expired
 */
export async function verifyChallenge(payload) {
  try {
    console.log('Verifying challenge with signature...')

    const response = await api.post('/auth/verify', payload)

    console.log('Challenge verified, JWT received')
    return response.data
  } catch (error) {
    console.error('Challenge verification error:', error)

    if (error.response?.data?.error) {
      throw new Error(error.response.data.error)
    }
    if (error.response?.data?.message) {
      throw new Error(error.response.data.message)
    }

    throw new Error('Verification failed: ' + error.message)
  }
}

/**
 * Verify that a JWT token is still valid
 * 
 * Used on app startup to restore user session without re-entering password
 * This is called once when app loads to check if stored token is valid
 * 
 * @param {string} token - JWT token to verify
 * @returns {Promise<Object>} User object if token valid
 * @throws {Error} If token expired or invalid
 */
export async function verifyToken(token) {
  try {
    const response = await api.get('/auth/verify', {
      headers: { Authorization: `Bearer ${token}` }
    })
    return response.data
  } catch (error) {
    throw new Error('Token verification failed')
  }
}