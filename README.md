**Bachelor Thesis Project:** Production-grade event-driven exchange system

## Architecture

```
┌─────────────┐
│  Next.js    │  (Future Frontend)
│  Browser    │
└──────┬──────┘
       │ HTTPS + WebSocket
       │
┌──────▼──────────────────────────────────────────┐
│  Gateway Service                                 │
│  - REST API (order submission, account)         │
│  - WebSocket (real-time updates)                │
│  - Hardware-backed auth (WebAuthn-compatible)   │
│  - JWT middleware                                │
└──────┬──────────────────────────────────────────┘
       │
┌──────▼──────────┐
│  PostgreSQL     │
│  - Users        │
│  - Challenges   │
└─────────────────┘

(Future Sprints: Matching Engine, Kafka, Settlement, Market Data)
```

## Security Model

### NO PASSWORDS. Hardware-backed authentication only.

**Registration Flow:**
1. Client generates Ed25519 key pair (private key never leaves device)
2. POST `/auth/register` with `{username, public_key_hex}`
3. Server stores public key in PostgreSQL

**Login Flow (Challenge-Response):**
1. POST `/auth/challenge` with `{username}`
2. Server returns `{challenge: "base64_random_bytes"}`
3. Client signs challenge with private key: `signature = sign(private_key, challenge)`
4. POST `/auth/verify` with `{username, challenge, signature, timestamp}`
5. Server:
   - Verifies signature using stored public key
   - Checks challenge not expired/reused
   - Validates timestamp skew < 5 minutes
   - Deletes challenge (one-time use)
   - Returns JWT: `{token: "eyJhbGciOi..."}`
6. Client includes JWT in `Authorization: Bearer <token>` header

**Replay Attack Prevention:**
- Challenge used once then deleted
- Challenge expires in 5 minutes
- Signature includes timestamp
- JWT expires in 15 minutes

## Sprint 1 Features

✅ Gateway service (REST + WebSocket)  
✅ Hardware-backed authentication (Ed25519)  
✅ Challenge-response with replay protection  
✅ JWT issuance and middleware  
✅ PostgreSQL schema (users, challenges)  
✅ CORS for Next.js  
✅ Docker Compose  
✅ Health endpoints  
✅ Structured logging  


---

## Next Sprints

- **Sprint 2:** Matching Engine + WAL
- **Sprint 3:** Kafka Integration
- **Sprint 4:** Settlement Service
- **Sprint 5:** Market Data Service
- **Sprint 6:** End-to-End Integration
- **Sprint 7:** Observability (Prometheus, Grafana)
- **Sprint 8:** Advanced Features + Thesis Defense Prep

---

## License

MIT License (for thesis purposes)

## Author

Bachelor Thesis Project, Hornet Alex-Andrei - 2026