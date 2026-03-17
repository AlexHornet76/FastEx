# FastEx - Hardware-Authenticated Trading Exchange

**Bachelor Thesis Project**: Production-grade event-driven cryptocurrency exchange system with hardware-backed authentication and deterministic matching.

---

## 🎯 Project Overview

FastEx is a modern, security-first cryptocurrency/stocks exchange implementing industry-standard matching algorithms with **zero-password authentication**. Built using hardware-backed cryptographic signatures (WebAuthn-compatible), the system ensures secure trading without traditional password vulnerabilities.

**Key Innovation**: Hardware-based Ed25519 authentication with challenge-response protocol eliminates password storage and replay attacks.

---

## System Architecture (as of Sprint 3)

```
┌─────────────┐
│  Next.js    │  (Future Frontend)
│  Browser    │
└──────┬──────┘
       │ HTTPS + WebSocket
       │
┌──────▼──────────────────────────────────────────┐
│  Gateway Service                                 │
│  - REST API (auth, account, order routing)       │
│  - WebSocket (real-time updates - later)         │
│  - Hardware-backed auth (Ed25519)                │
│  - JWT issuance + middleware                      │
└──────┬──────────────────────────────────────────┘
       │ HTTP (internal)
       │
┌──────▼──────────────────────────────────────────┐
│  Matching Engine Service (engine)                │
│  - Per-instrument in-memory order book           │
│  - Deterministic price-time matching             │
│  - WAL (append-only, crash recovery)             │
│  - REST API (orders, cancel, orderbook snapshot) │
└──────┬──────────────────────────────────────────┘
       │ WAL tail / replay (outbox-style)
       │
┌──────▼──────────────────────────────────────────┐
│  Kafka (Zookeeper-based, docker-compose)         │
│  - Domain event topics (v1)                      │
│  - Ordering per instrument via message key       │
└──────────────────────────────────────────────────┘

Persistence:
- PostgreSQL (Gateway domain): users + auth challenges
- WAL files (Engine domain): order lifecycle + trades (per instrument)
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

## WAL-backed Kafka Writer (Outbox-style)

Kafka is used as an **integration bus** for downstream services. The matching engine’s WAL is the **single source of truth**.

### Why WAL-backed publishing?
Publishing directly to Kafka from the matching path can lose events when Kafka is down or when the process crashes between “state change” and “publish”.

Using WAL-backed publishing ensures:
- if an event exists in Kafka, it *must have been committed to WAL first*
- if Kafka is temporarily unavailable, events will be published later from WAL
- the engine remains correct even when Kafka is down (WAL + recovery still work)

### Kafka Topics (v1)
- `order.placed.v1`
- `trade.executed.v1`
- `order.canceled.v1`

### Writer responsibilities
The WAL-backed Kafka writer (publisher) is responsible for:
- reading committed WAL entries for an instrument
- converting WAL entries into domain events
- publishing to Kafka in WAL order
- checkpointing progress so it can resume after restarts

### Writer Flow (per instrument)
For each configured instrument (ex: **BTC**, **AAPL**), one publisher loop runs:

1. **Load cursor**
   - The publisher reads a cursor file:
     - `<WAL_DIR>/cursors/BTC.cursor`
     - `<WAL_DIR>/cursors/AAPL.cursor`
   - The cursor stores: `last_published_sequence_num`
   - If cursor does not exist → start from `0`

2. **Read WAL entries after cursor**
   - The publisher reads the instrument WAL file:
     - `<WAL_DIR>/BTC.wal`
     - `<WAL_DIR>/AAPL.wal`
   - It filters entries where `sequence_num > last_published_sequence_num`

3. **Transform WAL entry → Kafka event**
   - `ORDER_PLACED` → publish to `order.placed`
   - `TRADE_EXECUTED` → publish to `trade.executed`
   - `ORDER_CANCELED` → publish to `order.canceled`

4. **Publish to Kafka**
   - The Kafka message **key** is the instrument (`BTC`, `AAPL`).
   - This ensures all events for a single instrument go to the same partition, preserving ordering for consumers.

5. **Advance cursor (checkpoint)**
   - After a successful publish, the publisher writes the new cursor value:
     - `cursor = entry.sequence_num`
   - Cursor is written atomically (write temp file → rename) to avoid corruption.

6. **Repeat / poll**
   - The publisher runs continuously (polling/tailing behavior).
   - If Kafka is down, publishing fails and the cursor does **not** advance.
   - On the next retry, the publisher will attempt the same WAL entry again.

### Delivery semantics
- The writer is designed for **at-least-once** delivery.
- Duplicates are possible in edge cases (e.g., published to Kafka but crashed before cursor write).
- Downstream consumers must be idempotent:
  - dedupe `trade.executed.v1` using `trade_id`
  - dedupe order events using `order_id`

### End-to-end event lifecycle (one order)
1. Engine appends `ORDER_PLACED` to WAL (durable).
2. Engine matches and appends `TRADE_EXECUTED` entries (durable).
3. WAL publisher observes new WAL entries (sequence > cursor).
4. WAL publisher publishes corresponding Kafka events (instrument-keyed).
5. WAL publisher checkpoints cursor so it won’t republish on restart.

---

## ✅ Sprint 1 Features (Completed)

### Gateway Service
- ✅ REST API with CORS configuration for browser clients — browser-friendly REST entrypoints
- ✅ WebSocket support with JWT authentication — realtime channel foundation (auth-protected)
- ✅ Health check endpoints — service health + docker orchestration readiness
- ✅ Graceful shutdown handling — clean server shutdown and resource cleanup

### Hardware-Backed Authentication
- ✅ Ed25519 signature verification (WebAuthn-compatible) — public-key verification server-side
- ✅ Challenge-response protocol with cryptographic security — prove key ownership without secrets on server
- ✅ No password storage (asymmetric cryptography only) — eliminates password database risk
- ✅ Replay attack prevention (one-time challenge use + expiration) — challenge TTL + single-use enforcement

### JWT Authorization
- ✅ Token-based authentication with configurable expiration — short-lived access tokens
- ✅ JWT middleware for protected endpoints — centralized auth enforcement
- ✅ Authorization header transport strategy — `Authorization: Bearer <token>`

### PostgreSQL Integration
- ✅ User registration and public key storage — persist identity + public keys
- ✅ Challenge management with automatic cleanup — store/expire login challenges
- ✅ Connection pooling and health checks — stable DB connectivity
- ✅ Schema migrations on startup — deterministic DB schema bootstrapping

### Infrastructure
- ✅ Docker Compose deployment — reproducible local environment
- ✅ Structured logging with configurable levels — consistent logs for debugging
- ✅ Environment-based configuration — configurable services without code changes
- ✅ Production-ready error handling — explicit error responses + logging

---

## ✅ Sprint 2 Features (Completed)

### Completed Components

#### Order Data Models
- ✅ Order types (Limit, Market) — standard order representations
- ✅ Order sides (Buy, Sell) — bid/ask direction modeling
- ✅ Order lifecycle states (New, Open, Partial, Filled, Cancelled, Rejected) — clear state transitions
- ✅ Trade data structures with buyer/seller tracking — trade events include both parties
- ✅ Price representation in smallest units (avoiding floating-point errors) — integer prices/qty for correctness

#### Order Book Data Structure
- ✅ Two-sided order book (Buy side + Sell side) — separate bid/ask books
- ✅ Price level queues with FIFO ordering — time priority within price
- ✅ Efficient operations: O(log n) insert/delete, O(1) best price lookup — performance-oriented structure
- ✅ Lazy sorting for amortized performance — avoid unnecessary resorting
- ✅ Automatic cleanup of empty price levels — keeps book compact
- ✅ Thread-safe with read/write mutex — safe concurrent reads/writes

#### Matching Algorithm
- ✅ Price-time priority matching (industry standard) — fair execution model
- ✅ FIFO execution at same price level — deterministic ordering
- ✅ Maker/taker model (execution at resting order's price) — canonical execution rule
- ✅ Partial fill support with order state tracking — supports multi-fill orders
- ✅ Multi-level matching (single order matches multiple resting orders) — realistic liquidity consumption
- ✅ Full match, partial match, and no-match scenarios — complete execution outcomes
- ✅ Atomic matching operations (lock-protected for determinism) — consistent results under concurrency

#### Write-Ahead Log (WAL)
- ✅ Append-only log for crash recovery — durable source of truth
- ✅ JSON-based entry format (newline-delimited) — easy debugging + simple parsing
- ✅ Entry types: `OrderPlaced`, `TradeExecuted`, `OrderCancelled` — complete lifecycle coverage
- ✅ Monotonic sequence numbering — strict ordering of state transitions
- ✅ fsync after each write (durability guarantee) — survives crashes
- ✅ Concurrent write safety with mutex protection — safe multi-request usage
- ✅ Log replay for state reconstruction — deterministic restart recovery

#### Engine Integration
- ✅ Orchestration layer combining OrderBook + WAL — cohesive engine component
- ✅ Write-ahead pattern: log BEFORE mutate — correctness-first mutation ordering
- ✅ Deterministic recovery from crash — replay produces same resulting book
- ✅ Proper order lifecycle management during replay — consistent filled/partial/open handling

---

## ✅ Sprint 3 Features (Completed) — Kafka Integration (WAL-backed)

- ✅ Kafka + Zookeeper in Docker Compose — local event backbone
- ✅ Topics for domain events — `order.placed`, `trade.executed`, `order.canceled`
- ✅ WAL-backed Kafka writer (outbox-style) — events published only after WAL commit
- ✅ Per-instrument ordering via Kafka key — key = instrument (BTC/AAPL), ordered consumption per instrument
- ✅ Cursor checkpointing — publisher resumes from last published WAL sequence after restart
- ✅ At-least-once delivery model — duplicates possible; downstream consumers must be idempotent

---

## 🚀 Future Sprints

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