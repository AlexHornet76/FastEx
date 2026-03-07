# FastEx - Hardware-Authenticated Trading Exchange

**Bachelor Thesis Project**: Production-grade event-driven cryptocurrency exchange system with hardware-backed authentication and deterministic matching.

---

## 🎯 Project Overview

FastEx is a modern, security-first cryptocurrency/stocks exchange implementing industry-standard matching algorithms with **zero-password authentication**. Built using hardware-backed cryptographic signatures (WebAuthn-compatible), the system ensures secure trading without traditional password vulnerabilities.

**Key Innovation**: Hardware-based Ed25519 authentication with challenge-response protocol eliminates password storage and replay attacks.

---

## 🏗️ System Architecture

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
┌──────▼──────────┐     ┌─────────────────────────┐
│  PostgreSQL     │     │  Matching Engine        │
│  - Users        │     │  - Order Book (BTC-USD) │
│  - Challenges   │     │  - Price-Time Priority  │
└─────────────────┘     │  - WAL (Crash Recovery) │
                        └─────────────────────────┘

(Future: Kafka, Settlement, Market Data, Observability)
```

---

## 🔐 Security Model

### NO PASSWORDS. Hardware-backed authentication only.

### Registration Flow

1. **Client** generates Ed25519 key pair (private key never leaves device)
2. **POST** `/auth/register` with `{username, public_key_hex}`
3. **Server** stores public key in PostgreSQL

### Login Flow (Challenge-Response)

1. **POST** `/auth/challenge` with `{username}`
2. **Server** returns `{challenge: "base64_random_bytes"}`
3. **Client** signs challenge: `signature = sign(private_key, challenge)`
4. **POST** `/auth/verify` with `{username, challenge, signature, timestamp}`
5. **Server** verifies:
   - Signature using stored public key
   - Challenge not expired/reused
   - Timestamp skew < 5 minutes
   - Deletes challenge (one-time use)
6. **Returns** JWT: `{token: "eyJhbGciOi..."}`
7. **Client** includes JWT in `Authorization: Bearer <token>` header

### Replay Attack Prevention

- ✅ Challenge used once then deleted
- ✅ Challenge expires in 5 minutes
- ✅ Signature includes timestamp
- ✅ JWT expires in 15 minutes

---

## 📊 Matching Engine

### Price-Time Priority with FIFO Execution

#### Order Book Structure

- **Two-Sided Book**: Separate buy side (bids) and sell side (asks)
- **Price Levels**: Orders grouped by price, sorted for optimal matching
  - Buy side: Highest price first (best bid at top)
  - Sell side: Lowest price first (best ask at top)
- **FIFO Queue**: Orders at same price level execute in arrival order (first-in, first-out)

#### Order Matching Flow

1. **Client** submits order: `BUY 10 BTC @ $50,000` (via authenticated endpoint)
2. **Write-Ahead Log**: Order written to disk FIRST (crash safety)
3. **Matching Engine**:
   - Checks opposite side (sell orders) for compatible prices
   - Matches best price first (lowest sell price for buy orders)
   - Executes at resting order's price (maker sets price, taker pays)
   - Generates trades: `Trade: 10 BTC @ $49,500` (if seller offered $49,500)
4. **Write-Ahead Log**: Trades written to disk
5. **Order Book Update**:
   - Fully filled orders removed from book
   - Partially filled orders remain with updated quantity
   - Unmatched quantity added to book as new resting order
6. **Response**: Client receives trade confirmations and remaining order status

#### Execution Price Rules

- Buy order matches sells at price ≤ buy limit
- Sell order matches buys at price ≥ sell limit
- **Trade always executes at the resting order's price** (existing book price)
- **Example**: Buy at $52,000 matches sell at $50,000 → Trade executes at $50,000

#### Crash Recovery

- **Write-Ahead Log** records every operation before execution
- **On restart**: Replay log sequentially to rebuild exact order book state
- **Deterministic replay** ensures identical state reconstruction
- **Operations logged**: `OrderPlaced`, `TradeExecuted`, `OrderCancelled`

---

## ✅ Sprint 1 Features (Completed)

### Gateway Service
- ✅ REST API with CORS configuration for browser clients
- ✅ WebSocket support with JWT authentication
- ✅ Health check endpoints
- ✅ Graceful shutdown handling

### Hardware-Backed Authentication
- ✅ Ed25519 signature verification (WebAuthn-compatible)
- ✅ Challenge-response protocol with cryptographic security
- ✅ No password storage (asymmetric cryptography only)
- ✅ Replay attack prevention (one-time challenge use + expiration)

### JWT Authorization
- ✅ Token-based authentication with configurable expiration
- ✅ JWT middleware for protected endpoints
- ✅ Authorization header transport strategy

### PostgreSQL Integration
- ✅ User registration and public key storage
- ✅ Challenge management with automatic cleanup
- ✅ Connection pooling and health checks
- ✅ Schema migrations on startup

### Infrastructure
- ✅ Docker Compose deployment
- ✅ Structured logging with configurable levels
- ✅ Environment-based configuration
- ✅ Production-ready error handling

---

## 🔄 Sprint 2 Features (In Progress)

### Completed Components

#### Order Data Models
- ✅ Order types (Limit, Market)
- ✅ Order sides (Buy, Sell)
- ✅ Order lifecycle states (New, Open, Partial, Filled, Cancelled, Rejected)
- ✅ Trade data structures with buyer/seller tracking
- ✅ Price representation in smallest units (avoiding floating-point errors)

#### Order Book Data Structure
- ✅ Two-sided order book (Buy side + Sell side)
- ✅ Price level queues with FIFO ordering
- ✅ Efficient operations: O(log n) insert/delete, O(1) best price lookup
- ✅ Lazy sorting for amortized performance
- ✅ Automatic cleanup of empty price levels
- ✅ Thread-safe with read/write mutex

#### Matching Algorithm
- ✅ Price-time priority matching (industry standard)
- ✅ FIFO execution at same price level
- ✅ Maker/taker model (execution at resting order's price)
- ✅ Partial fill support with order state tracking
- ✅ Multi-level matching (single order matches multiple resting orders)
- ✅ Full match, partial match, and no-match scenarios
- ✅ Atomic matching operations (lock-protected for determinism)

#### Write-Ahead Log (WAL)
- ✅ Append-only log for crash recovery
- ✅ JSON-based entry format (newline-delimited)
- ✅ Entry types: `OrderPlaced`, `TradeExecuted`, `OrderCancelled`
- ✅ Monotonic sequence numbering
- ✅ fsync after each write (durability guarantee)
- ✅ Concurrent write safety with mutex protection
- ✅ Log replay for state reconstruction

### In Progress

#### Engine Integration
- 🔄 Orchestration layer combining OrderBook + WAL
- 🔄 Write-ahead pattern: log BEFORE mutate
- 🔄 Deterministic recovery from crash
- 🔄 Proper order lifecycle management during replay

### Upcoming Sprint 2 Tasks

- ⏳ **REST API for Order Submission**
  - POST /orders endpoint (authenticated)
  - Order validation and rejection handling
  - Real-time order status responses

- ⏳ **Concurrency Model**
  - Per-instrument goroutine architecture
  - Channel-based order submission
  - Lock-free inter-service communication design

- ⏳ **Gateway ↔ Matching Engine Integration**
  - gRPC or HTTP communication protocol
  - Request/response flow for order submission
  - Error propagation and retry logic

- ⏳ **Configuration & Deployment**
  - Matching engine service containerization
  - Docker Compose integration with gateway
  - Environment configuration for instruments
  - Health checks and monitoring endpoints

---

## 🚀 Future Sprints

### Sprint 3: Kafka Integration
- Event publishing (`OrderPlaced`, `TradeExecuted`)
- Partitioning strategy by instrument
- Exactly-once vs at-least-once semantics

### Sprint 4: Settlement Service
- Kafka consumer for `TradeExecuted` events
- PostgreSQL balance management
- Idempotent transaction processing
- Double-entry bookkeeping validation

### Sprint 5: Market Data Service
- Order book snapshots and deltas
- OHLCV candle aggregation
- WebSocket real-time feeds
- Trade history API

### Sprint 6: End-to-End Integration
- Full request flow: Gateway → Matching → Kafka → Settlement
- WebSocket order status updates
- Rate limiting and circuit breakers
- Load testing (target: 1000+ orders/sec)

### Sprint 7: Observability (Prometheus, Grafana)
- Metrics: latency histograms, throughput, Kafka lag
- Distributed tracing with correlation IDs
- Centralized logging
- Alerting rules

### Sprint 8: Advanced Features + Thesis Defense
- Advanced order types (Stop-Loss, Post-Only)
- Order amendment support
- Performance benchmarking and optimization
- Architecture documentation and threat modeling
- Thesis defense preparation

### Sprint 9: Frontend Development (Next.js)
- User interface for registration and login
- WebAuthn/hardware key integration in browser
- Real-time order book visualization
- Order submission interface
- Live trade feed and order status updates
- Portfolio and balance display
- Responsive design for mobile and desktop
- WebSocket integration for real-time data

---

## 🎨 System Design Highlights

### Matching Engine
- **Deterministic execution** for reproducible behavior
- **Crash-safe** through WAL replay
- **Price-time priority** ensures fairness
- **Lock-based concurrency** (one match at a time per instrument)
- **Future**: Per-instrument goroutines for horizontal scalability

### Data Integrity
- **Write-Ahead Logging** prevents data loss
- **Atomic operations** ensure consistency
- **Idempotent operations** for retry safety
- **Sequence numbers** for ordering guarantees

### Performance Considerations
- **In-memory order book** for low latency
- **Lazy sorting** for amortized O(1) inserts
- **Connection pooling** for database efficiency
- **fsync tuning** tradeoff (durability vs throughput)

---

## 🛠️ Technology Stack

- **Language**: Go 1.21+
- **Database**: PostgreSQL 15+
- **Authentication**: Ed25519 cryptographic signatures
- **API**: REST + WebSocket
- **Deployment**: Docker Compose
- **Future**: Kafka, Prometheus, Grafana

---

## 📦 Getting Started

### Prerequisites
- Go 1.21 or higher
- Docker & Docker Compose
- PostgreSQL 15+ (via Docker)

### Installation

```bash
# Clone the repository
git clone https://github.com/AlexHornet76/FastEx.git
cd FastEx

# Start PostgreSQL with Docker Compose
docker-compose up -d postgres

# Run the gateway service
cd gateway
go run cmd/main.go

# Run the matching engine (future)
cd engine
go run cmd/main.go
```

### Running Tests

```bash
# Test WAL implementation
go test ./engine/internal/wal -v

# Test matching engine
go test ./engine/internal/matching -v

# Test order book
go test ./engine/internal/orderbook -v
```

---

## 📝 License

MIT License (for thesis purposes)

---

## 👤 Author

**Alex-Andrei Hornet**  
Bachelor Thesis Project - 2026

---

## 🙏 Acknowledgments

This project implements industry-standard exchange architecture patterns used by major cryptocurrency exchanges, adapted for educational and research purposes.