package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	// Generate Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key pair: %v\n", err)
		os.Exit(1)
	}

	// Encode as hex
	privateKeyHex := hex.EncodeToString(privateKey)
	publicKeyHex := hex.EncodeToString(publicKey)

	fmt.Println("Ed25519 Key Pair Generated")
	fmt.Println("===========================")
	fmt.Printf("Private Key (keep secret!): %s\n", privateKeyHex)
	fmt.Printf("Public Key:                 %s\n", publicKeyHex)
	fmt.Println("\nUse public key in POST /auth/register")
	fmt.Println("Use private key to sign challenges")
}
