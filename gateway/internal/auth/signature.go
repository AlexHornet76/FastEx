package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// VerifyEd25519Signature verifies challenge signature using stored public key
// publicKeyHex: hex-encoded Ed25519 public key (64 chars)
// challengeB64: base64-encoded challenge (as stored in DB and sent to client)
// signatureHex: hex-encoded signature from client
func VerifyEd25519Signature(publicKeyHex, challengeB64, signatureHex string) error {
	// Decode public key from hex
	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	// Decode signature from hex
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size: expected %d, got %d", ed25519.SignatureSize, len(signature))
	}

	// CRITICAL FIX: Decode challenge from base64 to raw bytes
	// The client signs the raw bytes, not the base64 string
	challengeBytes, err := base64.StdEncoding.DecodeString(challengeB64)
	if err != nil {
		return fmt.Errorf("invalid challenge base64: %w", err)
	}

	// Verify signature
	if !ed25519.Verify(publicKey, challengeBytes, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
