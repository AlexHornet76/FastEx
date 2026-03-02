package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	// Subcommands
	registerCmd := flag.NewFlagSet("register", flag.ExitOnError)
	registerUsername := registerCmd.String("username", "", "Username")
	registerPublicKey := registerCmd.String("public-key", "", "Public key (hex)")

	loginCmd := flag.NewFlagSet("login", flag.ExitOnError)
	loginUsername := loginCmd.String("username", "", "Username")
	loginPrivateKey := loginCmd.String("private-key", "", "Private key (hex)")

	signCmd := flag.NewFlagSet("sign", flag.ExitOnError)
	signPrivateKey := signCmd.String("private-key", "", "Private key (hex)")
	signChallenge := signCmd.String("challenge", "", "Challenge (base64)")

	if len(os.Args) < 2 {
		fmt.Println("Usage: test-client <command> [options]")
		fmt.Println("\nCommands:")
		fmt.Println("  register   Register new user")
		fmt.Println("  login      Full login flow (challenge + verify)")
		fmt.Println("  sign       Sign challenge (helper)")
		os.Exit(1)
	}

	baseURL := "http://localhost:8080"

	switch os.Args[1] {
	case "register":
		registerCmd.Parse(os.Args[2:])
		if *registerUsername == "" || *registerPublicKey == "" {
			registerCmd.PrintDefaults()
			os.Exit(1)
		}
		register(baseURL, *registerUsername, *registerPublicKey)

	case "login":
		loginCmd.Parse(os.Args[2:])
		if *loginUsername == "" || *loginPrivateKey == "" {
			loginCmd.PrintDefaults()
			os.Exit(1)
		}
		login(baseURL, *loginUsername, *loginPrivateKey)

	case "sign":
		signCmd.Parse(os.Args[2:])
		if *signPrivateKey == "" || *signChallenge == "" {
			signCmd.PrintDefaults()
			os.Exit(1)
		}
		signChallenge := signChallengeFunc(*signPrivateKey, *signChallenge)
		fmt.Printf("Signature: %s\n", signChallenge)

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func register(baseURL, username, publicKey string) {
	payload := map[string]string{
		"username":   username,
		"public_key": publicKey,
	}

	resp, err := postJSON(baseURL+"/auth/register", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Register failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Registration successful: %s\n", resp)
}

func login(baseURL, username, privateKeyHex string) {
	// Step 1: Request challenge
	payload := map[string]string{"username": username}
	resp, err := postJSON(baseURL+"/auth/challenge", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Challenge request failed: %v\n", err)
		os.Exit(1)
	}

	var chalResp struct {
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal([]byte(resp), &chalResp); err != nil {
		fmt.Fprintf(os.Stderr, "Parse challenge failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Challenge received: %s\n", chalResp.Challenge)

	// Step 2: Sign challenge
	signature := signChallengeFunc(privateKeyHex, chalResp.Challenge)
	timestamp := time.Now().Unix()

	// Step 3: Verify
	verifyPayload := map[string]interface{}{
		"username":  username,
		"challenge": chalResp.Challenge,
		"signature": signature,
		"timestamp": timestamp,
	}

	resp, err = postJSON(baseURL+"/auth/verify", verifyPayload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Verify failed: %v\n", err)
		os.Exit(1)
	}

	var tokenResp struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal([]byte(resp), &tokenResp); err != nil {
		fmt.Fprintf(os.Stderr, "Parse token failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nLogin successful!\n")
	fmt.Printf("JWT Token: %s\n", tokenResp.Token)
	fmt.Printf("Expires in: %d seconds\n", tokenResp.ExpiresIn)
}

func signChallengeFunc(privateKeyHex, challengeB64 string) string {
	// Decode private key from hex
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid private key: %v\n", err)
		os.Exit(1)
	}

	// Ensure correct private key length (64 bytes for Ed25519)
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "Invalid private key size: expected %d, got %d\n",
			ed25519.PrivateKeySize, len(privateKeyBytes))
		os.Exit(1)
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)

	// CRITICAL: Decode challenge from base64 to raw bytes
	challengeBytes, err := base64.StdEncoding.DecodeString(challengeB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid challenge base64: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Debug: Challenge bytes (hex): %x\n", challengeBytes)
	fmt.Printf("Debug: Challenge length: %d bytes\n", len(challengeBytes))

	// Sign the decoded bytes (NOT the base64 string)
	signature := ed25519.Sign(privateKey, challengeBytes)

	// Return signature as hex
	return hex.EncodeToString(signature)
}

func postJSON(url string, payload interface{}) (string, error) {
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}
