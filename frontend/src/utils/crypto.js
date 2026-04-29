import nacl from 'tweetnacl'

/**
 * Convert Uint8Array to hex string
 * Uses native browser Uint8Array instead of Node.js Buffer
 * 
 * @param {Uint8Array} bytes - Bytes to convert
 * @returns {string} Hex string
 */
function bytesToHex(bytes) {
  return Array.from(bytes)
    .map(byte => byte.toString(16).padStart(2, '0'))
    .join('')
}

/**
 * Convert hex string to Uint8Array
 * Uses native browser Uint8Array instead of Node.js Buffer
 * 
 * @param {string} hex - Hex string to convert
 * @returns {Uint8Array} Bytes
 */
function hexToBytes(hex) {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substr(i, 2), 16)
  }
  return bytes
}

/**
 * Convert string to Uint8Array
 * Uses TextEncoder (native browser API)
 * 
 * @param {string} str - String to convert
 * @returns {Uint8Array} Bytes
 */
function stringToBytes(str) {
  return new TextEncoder().encode(str)
}

/**
 * Convert base64 string to Uint8Array
 * Uses native browser atob() function
 * 
 * @param {string} base64 - Base64 string
 * @returns {Uint8Array} Bytes
 */
function base64ToBytes(base64) {
  const binaryString = atob(base64)
  const bytes = new Uint8Array(binaryString.length)
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return bytes
}

/**
 * Validate private key format
 * Ed25519 private keys should be 128 hex characters (64 bytes)
 * 
 * @param {string} privateKeyHex - Private key in hex format
 * @returns {boolean} True if valid format
 */
export function validatePrivateKeyFormat(privateKeyHex) {
  // Ed25519 secret key is 64 bytes = 128 hex characters
  return /^[a-f0-9]{128}$/i.test(privateKeyHex.trim())
}

/**
 * Generate an Ed25519 key pair for hardware-backed authentication
 * 
 * Security notes:
 * - Key pair generated locally on user's device (never sent to server)
 * - Uses TweetNaCl.js which implements libsodium
 * - Ed25519 is industry standard for digital signatures
 * - Private key should be stored securely by user
 * 
 * @returns {Promise<Object>} Object with publicKey and privateKey in hex format
 * @throws {Error} If key generation fails
 */
export async function generateKeyPair() {
  try {
    // Generate keypair using TweetNaCl.js (libsodium wrapper)
    const keyPair = nacl.sign.keyPair()

    // Convert from Uint8Array to hex strings for easy storage/display
    // Using native browser APIs instead of Node.js Buffer
    return {
      // Public key: can be shared and stored on server
      publicKey: bytesToHex(keyPair.publicKey),
      // Private key: must be kept secret by user
      privateKey: bytesToHex(keyPair.secretKey)
    }
  } catch (err) {
    throw new Error('Failed to generate key pair: ' + err.message)
  }
}

/**
 * Sign a message with private key
 * 
 * Used during login to prove ownership of private key without sending the key itself
 * 
 * Process:
 * 1. Backend sends random challenge
 * 2. User's device signs challenge with private key
 * 3. Frontend sends signature to backend
 * 4. Backend verifies signature using stored public key
 * 5. If valid, user is authenticated
 * 
 * @param {string} privateKeyHex - Private key in hex format
 * @param {string} message - Message to sign (base64 encoded)
 * @returns {Promise<string>} Signature in hex format
 * @throws {Error} If signing fails
 */
export async function signMessage(privateKeyHex, message) {
  try {
    // Validate private key format first
    if (!validatePrivateKeyFormat(privateKeyHex)) {
      throw new Error('Invalid private key format (should be 128 hex characters)')
    }

    // Convert hex strings back to bytes using native browser APIs
    const privateKey = hexToBytes(privateKeyHex.trim())
    const messageBytes = base64ToBytes(message)

    // Create detached signature (just the signature, not message + signature)
    // This is what backend expects for verification
    const signed = nacl.sign.detached(messageBytes, privateKey)

    // Return signature as hex string
    return bytesToHex(signed)
  } catch (err) {
    throw new Error('Failed to sign message: ' + err.message)
  }
}