package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
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

type registerReq struct {
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
}

type challengeReq struct {
	Username string `json:"username"`
}

type challengeResp struct {
	Challenge string `json:"challenge"`
}

type verifyReq struct {
	Username  string `json:"username"`
	Challenge string `json:"challenge"` // base64 string from server
	Signature string `json:"signature"` // hex
	Timestamp int64  `json:"timestamp"`
}

type verifyResp struct {
	Token string `json:"token"`
}

func main() {
	var (
		baseURL  = flag.String("base-url", "http://localhost:8080", "Gateway base URL")
		username = flag.String("username", "test1", "Username")
		timeout  = flag.Duration("timeout", 5*time.Second, "HTTP timeout")
	)
	flag.Parse()

	client := &http.Client{Timeout: *timeout}

	// 1) keypair (hex)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	must(err)

	pubHex := hex.EncodeToString(pub)
	privHex := hex.EncodeToString(priv)

	fmt.Println("== Keys ==")
	fmt.Println("username:", *username)
	fmt.Println("public_key(hex):", pubHex)
	fmt.Println("private_key(hex):", privHex)
	fmt.Println()

	// 2) register (ignore if already exists)
	fmt.Println("== Register ==")
	regBody := mustJSON(registerReq{Username: *username, PublicKey: pubHex})
	regStatus, regResp := doPOST(client, *baseURL+"/auth/register", regBody)
	fmt.Println("status:", regStatus)
	fmt.Println("response:", regResp)
	fmt.Println()

	// 3) challenge
	fmt.Println("== Challenge ==")
	chBody := mustJSON(challengeReq{Username: *username})
	_, chRaw := doPOST(client, *baseURL+"/auth/challenge", chBody)

	var ch challengeResp
	if err := json.Unmarshal([]byte(chRaw), &ch); err != nil || ch.Challenge == "" {
		fmt.Fprintln(os.Stderr, "challenge response was:", chRaw)
		must(fmt.Errorf("failed to parse challenge (expected {\"challenge\":\"...\"})"))
	}
	fmt.Println("challenge(b64):", ch.Challenge)
	fmt.Println()

	// 4) sign challenge (server expects signature over decoded base64 bytes)
	chBytes, err := base64.StdEncoding.DecodeString(ch.Challenge)
	must(err)
	sig := ed25519.Sign(priv, chBytes)
	sigHex := hex.EncodeToString(sig)

	fmt.Println("== Signature ==")
	fmt.Println("signature(hex):", sigHex)
	fmt.Println()

	// 5) verify -> token
	fmt.Println("== Verify ==")
	verBody := mustJSON(verifyReq{
		Username:  *username,
		Challenge: ch.Challenge,
		Signature: sigHex,
		Timestamp: time.Now().Unix(),
	})
	_, verRaw := doPOST(client, *baseURL+"/auth/verify", verBody)

	// verify might return token or error; try to parse token
	var vr verifyResp
	_ = json.Unmarshal([]byte(verRaw), &vr)

	if vr.Token == "" {
		fmt.Fprintln(os.Stderr, "verify response was:", verRaw)
		must(fmt.Errorf("no token in verify response"))
	}

	fmt.Println("== JWT ==")
	fmt.Println(vr.Token)
}

func doPOST(c *http.Client, url string, body []byte) (int, string) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	must(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	must(err)
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	must(err)
	return b
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
