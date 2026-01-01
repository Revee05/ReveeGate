# ReveeGate

# Real-Time Live Streaming Donation Payment Gateway
**Private · Modular Monolith · Production-Grade Architecture**

---

## Executive Summary

ReveeGate adalah sistem payment gateway untuk donasi live streaming yang dirancang dengan prinsip **low latency**, **high reliability**, dan **production-ready** dari awal. Sistem ini memungkinkan viewer untuk berdonasi secara real-time dengan notifikasi langsung ke overlay streaming (OBS) dengan latensi minimal.

---

## Project Scope

### Primary Objective
Membangun **End-to-End Payment Gateway untuk Donasi Live Streaming** dengan karakteristik:
- **Real-time notification**: Donasi tampil di overlay < 1 detik
- **Production-ready**: Siap deploy ke production tanpa refactoring
- **Self-hosted**: Full control & privacy untuk single owner
- **Modular architecture**: Mudah maintain dan scale

### Target Use Case
- **User**: Single owner (private use)
- **Viewers**: Dapat berdonasi tanpa registrasi
- **Streamer**: Menerima notifikasi real-time di OBS overlay
- **Admin**: Monitoring & reconciliation via dashboard

---

## Technical Constraints

### Non-Negotiable Requirements
- **Architecture**: Modular Monolith (bukan microservices)
- **Language**: Go untuk backend (NO TypeScript di seluruh stack)
- **Deployment**: Single VPS deployment
- **Target Region**: Indonesia only
- **Performance**: End-to-end latency < 1 detik
- **Reliability**: 99.9% uptime target

---

## Technology Stack

### Backend Core
**Language**: Go 1.21+

**Framework & Libraries**:
- HTTP Router: `chi/v5` (lightweight, production-tested)
- Database Driver: `pgx/v5` (native PostgreSQL driver)
- Query Builder: `sqlc` (type-safe SQL generation)
- Cache & Pub/Sub: `go-redis/v9`
- WebSocket: `gorilla/websocket` atau native `nhooyr.io/websocket`
- Validation: `go-playground/validator/v10`
- Logging: `slog` (Go 1.21+ native structured logging)
- Environment: `godotenv` atau `viper`

**Rationale**: Native Go libraries untuk performance, minimal dependencies untuk stability.

---

### Database Layer
**Primary Database**: PostgreSQL 15+
- ACID compliance untuk transaction integrity
- JSONB support untuk flexible metadata
- Row-level security
- Full-text search capabilities

**Cache & Message Broker**: Redis 7+
- Session management
- Rate limiting counters
- Real-time pub/sub untuk WebSocket events
- Idempotency key storage (dengan TTL)

---

### Frontend Stack

#### 1. Public Donation Page
**Stack**: Vanilla JavaScript + HTML5 + CSS3
- **Framework**: None (untuk minimal loading time)
- **Styling**: Tailwind CSS via CDN (opsional, dapat diganti Vanilla CSS)
- **Validation**: Client-side validation dengan Constraint Validation API
- **Features**:
  - Mobile-first responsive design
  - Progressive enhancement
  - Zero authentication required
  - Accessibility compliance (WCAG 2.1 Level AA)

#### 2. Live Overlay (OBS Browser Source)
**Stack**: Pure HTML + CSS + Vanilla JavaScript
- **Size**: < 50KB total (critical performance)
- **Connection**: WebSocket/SSE ke backend
- **Animation**: CSS animations only (GPU-accelerated)
- **Features**:
  - Auto-reconnect mechanism
  - Graceful fallback
  - Chromium-optimized (OBS uses Chromium)

#### 3. Admin Dashboard
**Stack**: React 18 (JavaScript only, NO TypeScript)
- **State Management**: React Context API atau Zustand
- **Routing**: React Router v6
- **HTTP Client**: Fetch API atau Axios
- **UI Components**: Headless UI + Tailwind CSS
- **Charts**: Chart.js atau Recharts
- **Features**:
  - Real-time transaction monitoring
  - Manual reconciliation interface
  - Payment provider status dashboard
  - Webhook logs viewer

---

## Payment Integration Strategy

### Supported Payment Methods (Indonesia Market)
1. **QRIS (Dynamic QR Code)**
   - Universal payment standard
   - Support all e-wallets & banking apps
   - Instant confirmation

2. **Virtual Account (Bank Transfer)**
   - BCA, BNI, Mandiri, BRI, Permata
   - 24/7 availability
   - Auto-verification via webhook

3. **E-Wallet Direct**
   - GoPay, DANA, OVO, ShopeePay, LinkAja
   - Deep-link integration
   - Real-time callback

### Payment Provider Architecture

**Design Pattern**: Provider Adapter Pattern (Anti Vendor Lock-in)

```
┌─────────────────────────────────────┐
│   Payment Service (Domain Layer)   │
└──────────────┬──────────────────────┘
               │
        ┌──────▼──────┐
        │  Provider   │
        │  Interface  │
        └──────┬──────┘
               │
      ┌────────┴────────┐
      │                 │
┌─────▼─────┐     ┌────▼─────┐
│ Midtrans  │     │  Xendit  │
│  Adapter  │     │  Adapter │
└───────────┘     └──────────┘
```

**Benefits**:
- Switch provider tanpa mengubah business logic
- A/B testing multiple providers
- Fallback mechanism saat provider down
- Centralized error handling

**Interface Contract** (Go):
```go
type PaymentProvider interface {
    CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
    VerifyWebhook(payload []byte, signature string) error
    GetPaymentStatus(ctx context.Context, externalID string) (*PaymentStatus, error)
}
```

---

## System Architecture

### Project Structure (Modular Monolith)

```
/ReveeGate
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
│
├── internal/
│   ├── config/                     # Configuration management
│   │   ├── config.go
│   │   └── validator.go
│   │
│   ├── domain/                     # Business entities & interfaces
│   │   ├── donation/
│   │   │   ├── entity.go
│   │   │   ├── repository.go       # Interface only
│   │   │   └── service.go          # Interface only
│   │   ├── payment/
│   │   │   ├── entity.go
│   │   │   ├── repository.go
│   │   │   └── service.go
│   │   └── provider/
│   │       └── interface.go        # Payment provider contract
│   │
│   ├── service/                    # Business logic implementation
│   │   ├── donation_service.go
│   │   ├── payment_service.go
│   │   └── notification_service.go
│   │
│   ├── repository/                 # Data access layer
│   │   ├── postgres/
│   │   │   ├── donation_repo.go
│   │   │   └── payment_repo.go
│   │   └── redis/
│   │       ├── cache.go
│   │       └── pubsub.go
│   │
│   ├── provider/                   # Payment provider adapters
│   │   ├── midtrans/
│   │   │   ├── client.go
│   │   │   ├── webhook.go
│   │   │   └── signature.go
│   │   └── xendit/
│   │       ├── client.go
│   │       ├── webhook.go
│   │       └── signature.go
│   │
│   ├── http/
│   │   ├── handler/                # HTTP request handlers
│   │   │   ├── donation_handler.go
│   │   │   ├── webhook_handler.go
│   │   │   └── admin_handler.go
│   │   ├── middleware/             # HTTP middleware
│   │   │   ├── auth.go
│   │   │   ├── ratelimit.go
│   │   │   ├── cors.go
│   │   │   ├── security.go
│   │   │   └── logger.go
│   │   ├── dto/                    # Data Transfer Objects
│   │   │   └── donation.go
│   │   └── router.go               # Route definitions
│   │
│   ├── realtime/                   # Real-time communication
│   │   ├── websocket/
│   │   │   ├── hub.go              # Connection manager
│   │   │   ├── client.go           # Client connection
│   │   │   └── handler.go
│   │   └── sse/
│   │       └── handler.go
│   │
│   ├── worker/                     # Background jobs
│   │   ├── payment_expiry.go
│   │   └── reconciliation.go
│   │
│   └── crypto/                     # Cryptographic utilities
│       ├── hmac.go
│       └── hash.go
│
├── db/
│   ├── migrations/                 # Database migrations (golang-migrate)
│   │   ├── 000001_init.up.sql
│   │   └── 000001_init.down.sql
│   └── sqlc/                       # SQLC generated code
│       ├── sqlc.yaml
│       ├── queries.sql
│       └── generated/
│
├── web-donor/                      # Public donation page
│   ├── index.html
│   ├── styles.css
│   └── app.js
│
├── web-overlay/                    # OBS overlay page
│   ├── index.html
│   ├── styles.css
│   └── overlay.js
│
├── web-dashboard/                  # Admin dashboard (React)
│   ├── public/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── utils/
│   │   └── App.jsx
│   ├── package.json
│   └── vite.config.js
│
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── Dockerfile.dev
│   ├── nginx/
│   │   └── nginx.conf
│   └── docker-compose.yml
│
├── scripts/
│   ├── migrate.sh
│   └── seed.sh
│
├── .env.example
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Donation Flow (End-to-End)

### User Journey
```
┌─────────┐     ┌──────────┐     ┌──────────┐     ┌─────────┐     ┌─────────┐
│ Viewer  │────▶│ Donation │────▶│ Payment  │────▶│ Provider│────▶│  OBS    │
│  Opens  │     │   Form   │     │ Provider │     │ Webhook │     │ Overlay │
│  Page   │     │  Submit  │     │   (QR)   │     │Callback │     │ Shows   │
└─────────┘     └──────────┘     └──────────┘     └─────────┘     └─────────┘
```

### Detailed Steps

**Phase 1: Donation Creation** (Target: < 500ms)
1. Viewer mengakses Public Donation Page
2. Mengisi form:
   - Nama donor (opsional, default "Anonymous")
   - Pesan donasi (max 500 karakter)
   - Nominal (min Rp 5.000)
3. Memilih metode pembayaran (QRIS / VA / E-Wallet)
4. Client mengirim POST request ke `/api/v1/donations`
5. Backend validasi input & membuat record `donations` (status: `PENDING`)
6. Backend memanggil Payment Provider API
7. Provider mengembalikan QR code / VA number / deep-link
8. Backend menyimpan `payment` record dengan `external_id`
9. Response dikembalikan ke client (QR code / payment instructions)

**Phase 2: Payment Processing** (Target: < 30 detik untuk user action)
10. User melakukan pembayaran via mobile banking / e-wallet
11. Payment provider memproses transaksi
12. Status masih `PENDING` hingga konfirmasi

**Phase 3: Webhook Confirmation** (Target: < 1 detik)
13. Provider mengirim webhook POST ke `/api/v1/webhooks/[provider]`
14. Backend menerima webhook:
    - Validasi signature (HMAC SHA256)
    - Validasi timestamp (anti replay attack, max 5 menit)
    - Check IP whitelist
15. Ekstrak `external_id` dan status pembayaran
16. Validasi idempotency key (cek Redis, TTL 24 jam)
17. Update `payment` status → `PAID`
18. Update `donation` status → `COMPLETED`
19. Simpan webhook log untuk audit trail

**Phase 4: Real-time Notification** (Target: < 200ms)
20. Backend publish event ke Redis Pub/Sub
21. WebSocket Hub menerima event
22. Broadcast ke semua connected overlay clients
23. OBS overlay menerima event via WebSocket
24. Tampilkan animasi donasi di stream

**Total End-to-End Latency**: < 1 detik (dari webhook received hingga tampil di overlay)

---

## Core Features

### 1. Donation Management
- [ ] Create donation dengan payment method selection
- [ ] Generate QR code / VA number / e-wallet deep-link
- [ ] Payment expiry handling (default: 24 jam)
- [ ] Donation history & search
- [ ] Donor statistics & leaderboard

### 2. Payment Processing
- [ ] Multi-provider support (Midtrans + Xendit)
- [ ] Dynamic payment method routing
- [ ] Transaction retry mechanism
- [ ] Payment status polling (fallback jika webhook gagal)
- [ ] Refund handling (manual via admin)

### 3. Webhook Management
- [ ] Webhook signature verification (HMAC SHA256)
- [ ] Timestamp validation (anti replay attack)
- [ ] IP whitelist validation
- [ ] Idempotency handling (prevent double notification)
- [ ] Webhook retry mechanism (exponential backoff)
- [ ] Webhook logs & debugging interface

### 4. Real-time Notification
- [ ] WebSocket connection management
- [ ] Auto-reconnect mechanism
- [ ] Event broadcasting ke multiple overlay instances
- [ ] Fallback ke Server-Sent Events (SSE)
- [ ] Connection health monitoring

### 5. Admin Dashboard
- [ ] Real-time transaction monitoring
- [ ] Manual payment reconciliation
- [ ] Webhook logs viewer
- [ ] Payment provider status dashboard
- [ ] Analytics & reporting (daily/weekly/monthly)
- [ ] System health metrics

### 6. Background Jobs
- [ ] Payment expiry worker (check setiap 5 menit)
- [ ] Auto-reconciliation worker (setiap 1 jam)
- [ ] Cleanup expired donations (setiap hari)
- [ ] Metrics aggregation worker

---

## Security Implementation

### 1. API Security

#### Rate Limiting
```go
// Per IP address
- Public endpoints: 100 requests/minute
- Donation creation: 10 requests/minute
- Webhook endpoints: 1000 requests/minute (provider traffic)
- Admin endpoints: 300 requests/minute
```

**Implementation**: Redis-backed sliding window algorithm

#### Authentication & Authorization
- **Public Donation Page**: No authentication required
- **OBS Overlay**: JWT token atau secure random token
- **Admin Dashboard**: JWT dengan refresh token mechanism
  - Access token TTL: 15 menit
  - Refresh token TTL: 7 hari
  - Secure HTTP-only cookies
- **Webhook Endpoints**: Signature verification + IP whitelist

#### CORS Configuration
```go
// Allowed origins (environment-based)
Production:
  - https://yourdomain.com
  - https://overlay.yourdomain.com
  - https://admin.yourdomain.com

Development:
  - http://localhost:3000
  - http://localhost:5173
```

---

### 2. Payment Security

#### Webhook Signature Verification
**Algorithm**: HMAC SHA256

```go
// Midtrans signature verification
hash := hmac.New(sha256.New, []byte(serverKey))
hash.Write([]byte(orderId + statusCode + grossAmount + serverKey))
expectedSignature := hex.EncodeToString(hash.Sum(nil))

if expectedSignature != receivedSignature {
    return ErrInvalidSignature
}
```

#### Anti Replay Attack
- Timestamp validation: reject requests > 5 menit
- Nonce tracking dengan Redis (TTL 10 menit)
- Webhook event ID deduplication

#### Idempotency
- Idempotency key: `payment_id + external_id`
- Storage: Redis dengan TTL 24 jam
- Prevent double notification ke overlay

#### IP Whitelist (Provider Specific)
```go
// Midtrans IPs
midtransIPs := []string{
    "103.127.16.0/23",  // Production
    "103.208.23.0/24",  // Sandbox
}

// Xendit IPs
xenditIPs := []string{
    "18.139.71.0/24",
    "13.229.120.0/24",
}
```

---

### 3. Web Application Security

#### Content Security Policy (CSP)
```http
Content-Security-Policy:
  default-src 'self';
  script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com;
  style-src 'self' 'unsafe-inline';
  img-src 'self' data: https:;
  connect-src 'self' wss://yourdomain.com;
  frame-ancestors 'none';
  base-uri 'self';
  form-action 'self';
```

#### Security Headers
```http
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

#### Input Validation & Sanitization
- **Server-side validation**: Semua input divalidasi dengan `validator/v10`
- **SQL Injection prevention**: Prepared statements via `sqlc`
- **XSS prevention**: HTML escaping untuk user-generated content
- **CSRF protection**: Double submit cookie pattern untuk admin dashboard

**Validation Rules**:
```go
type CreateDonationRequest struct {
    DonorName      string  `validate:"max=100"`
    Message        string  `validate:"max=500"`
    Amount         int64   `validate:"required,min=5000,max=100000000"`
    PaymentMethod  string  `validate:"required,oneof=qris va_bca gopay dana"`
}
```

#### Session Management
- Secure session cookies dengan `httpOnly`, `secure`, `sameSite=strict`
- Session storage: Redis
- Session TTL: 30 menit dengan sliding expiration
- Logout: Clear session dari Redis + clear cookie

---

### 4. Database Security

#### Connection Security
```go
// PostgreSQL connection with SSL
connString := fmt.Sprintf(
    "postgres://%s:%s@%s/%s?sslmode=require&pool_max_conns=25",
    user, password, host, database,
)
```

#### Query Safety
- **NO raw SQL queries** kecuali di migration
- **SQLC** untuk type-safe queries
- Prepared statements untuk semua dynamic queries
- Row-level security (RLS) untuk multi-tenancy di masa depan

#### Data Encryption
- **At rest**: Database encryption (provider-level)
- **In transit**: TLS 1.3 untuk semua connections
- **Sensitive data**: Bcrypt untuk password (admin), HMAC untuk tokens

#### Backup Strategy
- Daily automated backups (PostgreSQL WAL)
- Backup retention: 30 hari
- Encrypted backups di offsite storage
- Monthly backup restore testing

---

### 5. Infrastructure Security

#### Docker Security
```dockerfile
# Non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser
USER appuser

# Read-only filesystem
docker run --read-only --tmpfs /tmp:rw,noexec,nosuid
```

#### NGINX Security
```nginx
# Hide version
server_tokens off;

# SSL/TLS configuration
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers HIGH:!aNULL:!MD5;
ssl_prefer_server_ciphers on;

# Request size limits
client_max_body_size 1M;
client_body_buffer_size 128k;

# Timeout configuration
client_body_timeout 10s;
client_header_timeout 10s;
send_timeout 10s;

# DDoS protection
limit_req_zone $binary_remote_addr zone=donation:10m rate=10r/m;
limit_req zone=donation burst=20 nodelay;
```

#### Environment Variables
- **NO hardcoded secrets** di codebase
- Secrets management: `.env` file (local) + Docker secrets (production)
- Rotate secrets setiap 90 hari
- Audit log untuk secret access

#### Logging & Monitoring
- **Structured logging** dengan `slog`
- **NO sensitive data** di logs (mask payment details, tokens)
- Centralized logging (file-based, dapat integrasikan dengan ELK di masa depan)
- Alert untuk suspicious activities:
  - Multiple failed signature verifications
  - Unusual payment patterns
  - High error rates

---

### 6. OBS Overlay Security

#### Token-based Access
```javascript
// Overlay connects dengan secure token
const ws = new WebSocket(`wss://yourdomain.com/ws/overlay?token=${OVERLAY_TOKEN}`);
```

- Token generation: Secure random 32 bytes
- Token storage: Redis dengan TTL unlimited (manual revoke)
- Token validation: Setiap WebSocket connection
- Token rotation: Manual via admin dashboard

#### Message Validation
- Validate semua incoming WebSocket messages
- Schema validation untuk donation events
- Sanitize HTML sebelum render di overlay

---

### 7. Compliance & Privacy

#### Data Protection
- **Minimal data collection**: Hanya data yang diperlukan
- **Data retention**: 
  - Completed donations: 1 tahun
  - Failed/expired donations: 30 hari
  - Webhook logs: 90 hari
- **Data anonymization**: Hash donor IP address
- **GDPR-ready**: Donor dapat request data deletion

#### Audit Trail
- Semua admin actions dicatat
- Webhook events disimpan untuk reconciliation
- Payment status changes dengan timestamp
- Immutable audit logs (append-only)

---

## Implementation Requirements

### Backend Implementation (Go)

#### 1. Create Donation Endpoint
```go
// internal/http/handler/donation_handler.go
func (h *DonationHandler) CreateDonation(w http.ResponseWriter, r *http.Request) {
    var req dto.CreateDonationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if err := h.validator.Struct(req); err != nil {
        respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    ctx := r.Context()
    
    // Create donation via service
    donation, payment, err := h.donationService.CreateDonation(ctx, service.CreateDonationParams{
        DonorName:     req.DonorName,
        Message:       req.Message,
        Amount:        req.Amount,
        PaymentMethod: req.PaymentMethod,
    })
    
    if err != nil {
        h.logger.Error("failed to create donation", "error", err)
        respondError(w, http.StatusInternalServerError, "Failed to process donation")
        return
    }

    respondJSON(w, http.StatusCreated, dto.DonationResponse{
        DonationID:     donation.ID,
        Amount:         donation.Amount,
        PaymentMethod:  payment.Method,
        QRCodeURL:      payment.QRCodeURL,      // for QRIS
        VANumber:       payment.VANumber,       // for VA
        DeepLink:       payment.DeepLink,       // for e-wallet
        ExpiresAt:      payment.ExpiresAt,
    })
}
```

#### 2. Payment Provider Adapter
```go
// internal/provider/interface.go
type PaymentProvider interface {
    CreatePayment(ctx context.Context, req CreatePaymentRequest) (*PaymentResponse, error)
    VerifyWebhook(payload []byte, signature string) error
    GetPaymentStatus(ctx context.Context, externalID string) (*PaymentStatus, error)
}

// internal/provider/midtrans/client.go
type MidtransAdapter struct {
    serverKey  string
    clientKey  string
    apiURL     string
    httpClient *http.Client
}

func (m *MidtransAdapter) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*PaymentResponse, error) {
    // Build request payload based on payment method
    payload := map[string]interface{}{
        "transaction_details": map[string]interface{}{
            "order_id":     req.OrderID,
            "gross_amount": req.Amount,
        },
        "customer_details": map[string]interface{}{
            "first_name": req.CustomerName,
        },
    }

    // Add payment method specific fields
    switch req.PaymentMethod {
    case "qris":
        payload["payment_type"] = "qris"
    case "gopay":
        payload["payment_type"] = "gopay"
        payload["gopay"] = map[string]interface{}{
            "enable_callback": true,
            "callback_url":    req.CallbackURL,
        }
    case "va_bca":
        payload["payment_type"] = "bank_transfer"
        payload["bank_transfer"] = map[string]interface{}{
            "bank": "bca",
        }
    }

    // Make API call to Midtrans
    resp, err := m.doRequest(ctx, "POST", "/v2/charge", payload)
    if err != nil {
        return nil, err
    }

    return m.parsePaymentResponse(resp, req.PaymentMethod)
}
```

#### 3. Webhook Handler with Signature Verification
```go
// internal/http/handler/webhook_handler.go
func (h *WebhookHandler) HandleMidtransWebhook(w http.ResponseWriter, r *http.Request) {
    // Read raw body for signature verification
    body, err := io.ReadAll(r.Body)
    if err != nil {
        respondError(w, http.StatusBadRequest, "Cannot read request body")
        return
    }
    defer r.Body.Close()

    // Verify signature
    signature := r.Header.Get("X-Signature-Key")
    if err := h.midtransProvider.VerifyWebhook(body, signature); err != nil {
        h.logger.Warn("invalid webhook signature", "error", err)
        respondError(w, http.StatusUnauthorized, "Invalid signature")
        return
    }

    // Validate IP whitelist
    clientIP := getClientIP(r)
    if !h.isWhitelistedIP(clientIP, h.config.MidtransIPWhitelist) {
        h.logger.Warn("webhook from non-whitelisted IP", "ip", clientIP)
        respondError(w, http.StatusForbidden, "Forbidden")
        return
    }

    // Parse webhook payload
    var webhook dto.MidtransWebhook
    if err := json.Unmarshal(body, &webhook); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid JSON")
        return
    }

    // Validate timestamp (anti-replay attack)
    webhookTime, _ := time.Parse(time.RFC3339, webhook.TransactionTime)
    if time.Since(webhookTime) > 5*time.Minute {
        h.logger.Warn("webhook timestamp too old", "timestamp", webhook.TransactionTime)
        respondError(w, http.StatusBadRequest, "Timestamp too old")
        return
    }

    // Check idempotency
    idempotencyKey := fmt.Sprintf("webhook:%s:%s", webhook.OrderID, webhook.TransactionID)
    if exists := h.cache.Exists(r.Context(), idempotencyKey); exists {
        h.logger.Info("duplicate webhook ignored", "order_id", webhook.OrderID)
        respondJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
        return
    }

    // Process webhook
    ctx := r.Context()
    if err := h.paymentService.ProcessWebhook(ctx, service.ProcessWebhookParams{
        Provider:      "midtrans",
        OrderID:       webhook.OrderID,
        TransactionID: webhook.TransactionID,
        Status:        mapMidtransStatus(webhook.TransactionStatus),
        PaidAt:        webhookTime,
        RawPayload:    body,
    }); err != nil {
        h.logger.Error("failed to process webhook", "error", err)
        respondError(w, http.StatusInternalServerError, "Processing failed")
        return
    }

    // Set idempotency key with 24h TTL
    h.cache.Set(ctx, idempotencyKey, "processed", 24*time.Hour)

    respondJSON(w, http.StatusOK, map[string]string{"status": "success"})
}
```

#### 4. Signature Verification Implementation
```go
// internal/crypto/hmac.go
func VerifyHMAC(message, signature, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(message))
    expectedMAC := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expectedMAC), []byte(signature))
}

// internal/provider/midtrans/signature.go
func (m *MidtransAdapter) VerifyWebhook(payload []byte, signature string) error {
    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return err
    }

    // Build signature string: order_id + status_code + gross_amount + server_key
    signatureString := fmt.Sprintf("%s%s%s%s",
        data["order_id"],
        data["status_code"],
        data["gross_amount"],
        m.serverKey,
    )

    hash := sha512.Sum512([]byte(signatureString))
    expectedSignature := hex.EncodeToString(hash[:])

    if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
        return errors.New("invalid signature")
    }

    return nil
}
```

#### 5. Real-time Event Push (WebSocket)
```go
// internal/realtime/websocket/hub.go
type Hub struct {
    clients    map[*Client]bool
    broadcast  chan *DonationEvent
    register   chan *Client
    unregister chan *Client
    mu         sync.RWMutex
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()
            
        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }
            h.mu.Unlock()
            
        case event := <-h.broadcast:
            h.mu.RLock()
            for client := range h.clients {
                select {
                case client.send <- event:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
            h.mu.RUnlock()
        }
    }
}

func (h *Hub) BroadcastDonation(donation *domain.Donation) {
    event := &DonationEvent{
        Type: "new_donation",
        Data: DonationData{
            ID:        donation.ID,
            DonorName: donation.DonorName,
            Message:   donation.Message,
            Amount:    donation.Amount,
            PaidAt:    donation.PaidAt,
        },
    }
    h.broadcast <- event
}

// internal/realtime/websocket/handler.go
func (h *WSHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
    // Validate overlay token
    token := r.URL.Query().Get("token")
    if !h.validateToken(token) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    conn, err := h.upgrader.Upgrade(w, r, nil)
    if err != nil {
        h.logger.Error("websocket upgrade failed", "error", err)
        return
    }

    client := &Client{
        hub:  h.hub,
        conn: conn,
        send: make(chan *DonationEvent, 256),
    }
    
    h.hub.register <- client

    go client.writePump()
    go client.readPump()
}
```

#### 6. Redis Pub/Sub Integration
```go
// internal/service/notification_service.go
func (s *NotificationService) PublishDonation(ctx context.Context, donation *domain.Donation) error {
    event := map[string]interface{}{
        "type": "new_donation",
        "data": donation,
    }
    
    payload, err := json.Marshal(event)
    if err != nil {
        return err
    }
    
    return s.redis.Publish(ctx, "donations:new", payload).Err()
}

// Subscribe in WebSocket hub
func (h *Hub) SubscribeToRedis(ctx context.Context) {
    pubsub := h.redis.Subscribe(ctx, "donations:new")
    defer pubsub.Close()
    
    ch := pubsub.Channel()
    for msg := range ch {
        var event DonationEvent
        if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
            h.logger.Error("failed to unmarshal event", "error", err)
            continue
        }
        
        h.broadcast <- &event
    }
}
```

---

## Database Schema

### PostgreSQL Tables

```sql
-- migrations/000001_init.up.sql

-- Donations table
CREATE TABLE donations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    donor_name VARCHAR(100) DEFAULT 'Anonymous',
    donor_email VARCHAR(255),
    message TEXT,
    amount BIGINT NOT NULL CHECK (amount >= 5000),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    metadata JSONB,
    paid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_donations_status ON donations(status);
CREATE INDEX idx_donations_created_at ON donations(created_at DESC);
CREATE INDEX idx_donations_paid_at ON donations(paid_at DESC) WHERE paid_at IS NOT NULL;

-- Payments table
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    donation_id UUID NOT NULL REFERENCES donations(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    payment_method VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    qr_code_url TEXT,
    va_number VARCHAR(100),
    deep_link TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    paid_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(provider, external_id)
);

CREATE INDEX idx_payments_donation_id ON payments(donation_id);
CREATE INDEX idx_payments_external_id ON payments(provider, external_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_expires_at ON payments(expires_at) WHERE status = 'pending';

-- Webhook logs table (for debugging & reconciliation)
CREATE TABLE webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(100),
    external_id VARCHAR(255),
    status_code INTEGER,
    raw_payload JSONB NOT NULL,
    signature VARCHAR(512),
    ip_address INET,
    processed BOOLEAN DEFAULT FALSE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_logs_provider ON webhook_logs(provider);
CREATE INDEX idx_webhook_logs_external_id ON webhook_logs(external_id);
CREATE INDEX idx_webhook_logs_created_at ON webhook_logs(created_at DESC);
CREATE INDEX idx_webhook_logs_processed ON webhook_logs(processed) WHERE NOT processed;

-- Admin users table (untuk dashboard)
CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    is_active BOOLEAN DEFAULT TRUE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Overlay tokens table
CREATE TABLE overlay_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token VARCHAR(64) NOT NULL UNIQUE,
    description VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_overlay_tokens_token ON overlay_tokens(token) WHERE is_active = TRUE;

-- Audit logs table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES admin_users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

-- Updated at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply triggers
CREATE TRIGGER update_donations_updated_at BEFORE UPDATE ON donations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payments_updated_at BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_admin_users_updated_at BEFORE UPDATE ON admin_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Redis Data Structures

```
# Idempotency keys (TTL: 24 hours)
webhook:{provider}:{external_id}:{transaction_id} -> "processed"

# Rate limiting (TTL: 60 seconds)
ratelimit:ip:{ip_address} -> counter

# Session storage (TTL: 30 minutes)
session:{session_id} -> JSON user data

# Overlay tokens (no TTL, manual revoke)
overlay_token:{token} -> UUID

# Payment status cache (TTL: 5 minutes)
payment:status:{payment_id} -> JSON payment data

# Pub/Sub channels
donations:new -> donation events broadcast
```

---

## API Specifications

### Public API Endpoints

#### Create Donation
```http
POST /api/v1/donations
Content-Type: application/json

{
  "donor_name": "John Doe",
  "donor_email": "john@example.com",
  "message": "Keep up the great work!",
  "amount": 50000,
  "payment_method": "qris"
}

Response 201 Created:
{
  "donation_id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": 50000,
  "payment_method": "qris",
  "qr_code_url": "https://api.midtrans.com/qr/...",
  "expires_at": "2025-12-31T23:59:59Z",
  "status": "pending"
}
```

#### Get Donation Status
```http
GET /api/v1/donations/{donation_id}

Response 200 OK:
{
  "donation_id": "550e8400-e29b-41d4-a716-446655440000",
  "donor_name": "John Doe",
  "message": "Keep up the great work!",
  "amount": 50000,
  "status": "completed",
  "paid_at": "2025-12-31T12:34:56Z",
  "created_at": "2025-12-31T12:30:00Z"
}
```

### Webhook Endpoints

#### Midtrans Webhook
```http
POST /api/v1/webhooks/midtrans
X-Signature-Key: {signature}
Content-Type: application/json

{
  "order_id": "DONATION-550e8400",
  "transaction_id": "midtrans-tx-123",
  "transaction_status": "settlement",
  "transaction_time": "2025-12-31T12:34:56Z",
  "gross_amount": "50000.00",
  "payment_type": "qris"
}

Response 200 OK:
{
  "status": "success"
}
```

### Admin API Endpoints

#### List Donations
```http
GET /api/v1/admin/donations?status=completed&page=1&limit=50
Authorization: Bearer {jwt_token}

Response 200 OK:
{
  "data": [
    {
      "donation_id": "...",
      "donor_name": "John Doe",
      "amount": 50000,
      "status": "completed",
      "paid_at": "2025-12-31T12:34:56Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 1000
  }
}
```

#### Manual Reconciliation
```http
POST /api/v1/admin/payments/{payment_id}/reconcile
Authorization: Bearer {jwt_token}
Content-Type: application/json

{
  "action": "mark_as_paid",
  "reason": "Manual verification from bank statement"
}

Response 200 OK:
{
  "status": "success",
  "payment_status": "paid"
}
```

### WebSocket API

#### OBS Overlay Connection
```javascript
// Client-side
const ws = new WebSocket('wss://yourdomain.com/ws/overlay?token=YOUR_OVERLAY_TOKEN');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  // Event format:
  {
    "type": "new_donation",
    "data": {
      "donation_id": "550e8400-e29b-41d4-a716-446655440000",
      "donor_name": "John Doe",
      "message": "Keep up the great work!",
      "amount": 50000,
      "paid_at": "2025-12-31T12:34:56Z"
    }
  }
};
```

---

## Deployment Strategy

### Infrastructure Requirements

**Minimum VPS Specifications**:
- CPU: 2 vCPU
- RAM: 4GB
- Storage: 50GB SSD
- Bandwidth: 2TB/month
- OS: Ubuntu 22.04 LTS

**Recommended Providers**: DigitalOcean, Vultr, Linode, Hetzner

---

### Docker Configuration

#### Dockerfile (Multi-stage build)
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /app/server ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .
COPY --from=builder /app/web-donor ./web-donor
COPY --from=builder /app/web-overlay ./web-overlay

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

CMD ["./server"]
```

#### docker-compose.yml
```yaml
version: '3.8'

services:
  app:
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile
    container_name: reveegate-app
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - REDIS_URL=${REDIS_URL}
      - JWT_SECRET=${JWT_SECRET}
      - MIDTRANS_SERVER_KEY=${MIDTRANS_SERVER_KEY}
      - MIDTRANS_CLIENT_KEY=${MIDTRANS_CLIENT_KEY}
      - XENDIT_API_KEY=${XENDIT_API_KEY}
      - APP_ENV=production
    depends_on:
      - postgres
      - redis
    networks:
      - reveegate-network
    volumes:
      - ./logs:/app/logs
    read_only: true
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=100m

  postgres:
    image: postgres:15-alpine
    container_name: reveegate-db
    restart: unless-stopped
    environment:
      - POSTGRES_DB=${POSTGRES_DB}
      - POSTGRES_USER=${POSTGRES_USER}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./db/migrations:/docker-entrypoint-initdb.d
    networks:
      - reveegate-network
    ports:
      - "127.0.0.1:5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: reveegate-redis
    restart: unless-stopped
    command: redis-server --requirepass ${REDIS_PASSWORD} --maxmemory 256mb --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data
    networks:
      - reveegate-network
    ports:
      - "127.0.0.1:6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

  nginx:
    image: nginx:alpine
    container_name: reveegate-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./deployments/nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./deployments/nginx/ssl:/etc/nginx/ssl:ro
      - ./web-dashboard/dist:/usr/share/nginx/html/admin:ro
    depends_on:
      - app
    networks:
      - reveegate-network

volumes:
  postgres-data:
  redis-data:

networks:
  reveegate-network:
    driver: bridge
```

---

### NGINX Configuration

```nginx
# deployments/nginx/nginx.conf

user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
    use epoll;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';

    access_log /var/log/nginx/access.log main;

    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;
    server_tokens off;

    # Security headers
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript application/json application/javascript application/xml+rss;

    # Rate limiting zones
    limit_req_zone $binary_remote_addr zone=donation_limit:10m rate=10r/m;
    limit_req_zone $binary_remote_addr zone=api_limit:10m rate=100r/m;

    # Upstream backend
    upstream reveegate_backend {
        server app:8080;
        keepalive 32;
    }

    # HTTP redirect to HTTPS
    server {
        listen 80;
        server_name yourdomain.com www.yourdomain.com;
        return 301 https://$server_name$request_uri;
    }

    # HTTPS server
    server {
        listen 443 ssl http2;
        server_name yourdomain.com www.yourdomain.com;

        # SSL configuration
        ssl_certificate /etc/nginx/ssl/fullchain.pem;
        ssl_certificate_key /etc/nginx/ssl/privkey.pem;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers on;
        ssl_session_cache shared:SSL:10m;
        ssl_session_timeout 10m;

        # HSTS
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

        # Max body size
        client_max_body_size 1M;
        client_body_buffer_size 128k;

        # Timeouts
        client_body_timeout 10s;
        client_header_timeout 10s;
        send_timeout 10s;

        # Root for static files
        root /usr/share/nginx/html;

        # Public donation page
        location / {
            proxy_pass http://reveegate_backend;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection 'upgrade';
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_cache_bypass $http_upgrade;
        }

        # API endpoints
        location /api/v1/donations {
            limit_req zone=donation_limit burst=20 nodelay;
            
            proxy_pass http://reveegate_backend;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        location /api/v1/ {
            limit_req zone=api_limit burst=50 nodelay;
            
            proxy_pass http://reveegate_backend;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        # Webhook endpoints (no rate limit for provider traffic)
        location /api/v1/webhooks/ {
            proxy_pass http://reveegate_backend;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        # WebSocket for overlay
        location /ws/overlay {
            proxy_pass http://reveegate_backend;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_read_timeout 86400;
        }

        # Admin dashboard (React SPA)
        location /admin {
            alias /usr/share/nginx/html/admin;
            try_files $uri $uri/ /admin/index.html;
            
            # Admin-specific security headers
            add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' wss://yourdomain.com;" always;
        }

        # Health check endpoint
        location /health {
            proxy_pass http://reveegate_backend;
            access_log off;
        }
    }
}
```

---

### Environment Variables (.env.example)

```bash
# Application
APP_ENV=production
APP_PORT=8080
APP_URL=https://yourdomain.com

# Database
DATABASE_URL=postgres://reveegate:password@postgres:5432/reveegate?sslmode=disable
POSTGRES_DB=reveegate
POSTGRES_USER=reveegate
POSTGRES_PASSWORD=your_secure_password_here

# Redis
REDIS_URL=redis://:your_redis_password@redis:6379/0
REDIS_PASSWORD=your_redis_password_here

# JWT
JWT_SECRET=your_jwt_secret_key_here_min_32_characters
JWT_EXPIRY=900  # 15 minutes

# Midtrans
MIDTRANS_SERVER_KEY=your_midtrans_server_key
MIDTRANS_CLIENT_KEY=your_midtrans_client_key
MIDTRANS_API_URL=https://app.midtrans.com  # or sandbox
MIDTRANS_IP_WHITELIST=103.127.16.0/23,103.208.23.0/24

# Xendit (optional)
XENDIT_API_KEY=your_xendit_api_key
XENDIT_WEBHOOK_TOKEN=your_xendit_webhook_token
XENDIT_IP_WHITELIST=18.139.71.0/24,13.229.120.0/24

# Overlay
OVERLAY_TOKEN=your_secure_overlay_token_here

# CORS
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://admin.yourdomain.com

# Rate Limiting
RATE_LIMIT_DONATION=10  # requests per minute
RATE_LIMIT_API=100      # requests per minute
```

---

### Deployment Steps

#### 1. Initial Server Setup
```bash
# SSH ke VPS
ssh root@your-vps-ip

# Update system
apt update && apt upgrade -y

# Install Docker & Docker Compose
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
apt install docker-compose -y

# Create app directory
mkdir -p /opt/reveegate
cd /opt/reveegate
```

#### 2. Clone Repository & Setup
```bash
# Clone repository
git clone https://github.com/yourusername/ReveeGate.git .

# Copy environment file
cp .env.example .env

# Edit environment variables
nano .env

# Setup SSL certificates (Let's Encrypt)
apt install certbot -y
certbot certonly --standalone -d yourdomain.com
cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem deployments/nginx/ssl/
cp /etc/letsencrypt/live/yourdomain.com/privkey.pem deployments/nginx/ssl/
```

#### 3. Build & Deploy
```bash
# Build and start containers
docker-compose up -d --build

# Check logs
docker-compose logs -f app

# Run database migrations
docker-compose exec app ./server migrate up
```

#### 4. Create Admin User
```bash
# Execute SQL to create first admin user
docker-compose exec postgres psql -U reveegate -d reveegate
INSERT INTO admin_users (username, password_hash, email) VALUES 
('admin', '$2a$10$...', 'admin@yourdomain.com');
```

#### 5. Verify Deployment
```bash
# Check all services running
docker-compose ps

# Test endpoints
curl https://yourdomain.com/health
curl https://yourdomain.com/api/v1/health

# Check SSL
curl -I https://yourdomain.com
```

---

### Monitoring & Maintenance

#### Health Checks
```bash
# Check container health
docker-compose ps

# View logs
docker-compose logs -f app
docker-compose logs -f nginx

# Database backup
docker-compose exec postgres pg_dump -U reveegate reveegate > backup_$(date +%Y%m%d).sql
```

#### Auto-backup Script (cron)
```bash
#!/bin/bash
# /opt/reveegate/scripts/backup.sh

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/opt/reveegate/backups"

# PostgreSQL backup
docker-compose exec -T postgres pg_dump -U reveegate reveegate | gzip > $BACKUP_DIR/db_$DATE.sql.gz

# Redis backup
docker-compose exec -T redis redis-cli --rdb /data/dump_$DATE.rdb

# Keep only last 30 days
find $BACKUP_DIR -type f -mtime +30 -delete
```

```bash
# Add to crontab
0 2 * * * /opt/reveegate/scripts/backup.sh
```

---

## Production Readiness Checklist

### Phase 1: Foundation (Week 1-2)
- [ ] Project structure setup
- [ ] Database schema design & migrations
- [ ] Core domain models & interfaces
- [ ] Repository layer implementation (PostgreSQL + Redis)
- [ ] Configuration management
- [ ] Logging infrastructure (structured logging)

### Phase 2: Payment Integration (Week 3-4)
- [ ] Provider adapter interface design
- [ ] Midtrans adapter implementation
  - [ ] Create payment (QRIS, VA, E-Wallet)
  - [ ] Webhook handler
  - [ ] Signature verification
  - [ ] Payment status polling
- [ ] Xendit adapter implementation (optional backup)
- [ ] Payment service layer
- [ ] Idempotency mechanism
- [ ] Payment expiry worker

### Phase 3: Core Features (Week 5-6)
- [ ] Donation service implementation
- [ ] Create donation endpoint
- [ ] Get donation status endpoint
- [ ] Webhook endpoints (Midtrans, Xendit)
- [ ] Real-time notification service
- [ ] WebSocket hub implementation
- [ ] Redis Pub/Sub integration
- [ ] Error handling & retry logic

### Phase 4: Frontend Development (Week 7-8)
- [ ] Public donation page
  - [ ] Responsive design (mobile-first)
  - [ ] Payment method selection
  - [ ] Real-time status updates
  - [ ] Error handling & UX
- [ ] OBS overlay page
  - [ ] WebSocket connection
  - [ ] Auto-reconnect mechanism
  - [ ] Donation animation
  - [ ] Performance optimization
- [ ] Admin dashboard (React)
  - [ ] Authentication & session management
  - [ ] Transaction monitoring
  - [ ] Webhook logs viewer
  - [ ] Manual reconciliation
  - [ ] Analytics dashboard

### Phase 5: Security Hardening (Week 9)
- [ ] Rate limiting implementation
- [ ] CORS configuration
- [ ] Security headers
- [ ] Content Security Policy
- [ ] Input validation & sanitization
- [ ] SQL injection prevention audit
- [ ] XSS prevention audit
- [ ] CSRF protection
- [ ] IP whitelist for webhooks
- [ ] Secret rotation mechanism
- [ ] Audit logging

### Phase 6: Testing & Quality Assurance (Week 10-11)
- [ ] Unit tests (min 70% coverage)
  - [ ] Service layer tests
  - [ ] Repository tests
  - [ ] Provider adapter tests
  - [ ] Crypto/signature tests
- [ ] Integration tests
  - [ ] API endpoint tests
  - [ ] Database integration tests
  - [ ] Redis integration tests
  - [ ] Webhook flow tests
- [ ] E2E tests
  - [ ] Complete donation flow
  - [ ] Payment provider integration
  - [ ] Real-time notification flow
- [ ] Load testing
  - [ ] API endpoints (target: 100 req/s)
  - [ ] WebSocket connections (target: 1000 concurrent)
  - [ ] Database queries (< 50ms avg)
- [ ] Security testing
  - [ ] OWASP Top 10 audit
  - [ ] Penetration testing
  - [ ] Signature verification testing

### Phase 7: DevOps & Deployment (Week 12)
- [ ] Dockerfile optimization
- [ ] Docker Compose configuration
- [ ] NGINX configuration & optimization
- [ ] SSL/TLS certificate setup (Let's Encrypt)
- [ ] Environment variable management
- [ ] Database backup strategy
- [ ] Monitoring setup
  - [ ] Application logs
  - [ ] Error tracking
  - [ ] Performance metrics
- [ ] Alerting configuration
- [ ] Deployment automation
- [ ] Rollback strategy

### Phase 8: Pre-Production (Week 13)
- [ ] Staging environment setup
- [ ] End-to-end testing di staging
- [ ] Provider sandbox testing
  - [ ] Midtrans sandbox integration
  - [ ] Xendit sandbox integration
- [ ] Performance benchmarking
- [ ] Security audit
- [ ] Documentation completion
  - [ ] API documentation
  - [ ] Deployment guide
  - [ ] Troubleshooting guide
  - [ ] Runbook
- [ ] Disaster recovery plan

### Phase 9: Production Launch (Week 14)
- [ ] Production environment setup
- [ ] DNS configuration
- [ ] SSL certificate installation
- [ ] Production database setup
- [ ] Redis production setup
- [ ] Environment variables configuration
- [ ] Initial deployment
- [ ] Smoke testing
- [ ] Payment provider production setup
  - [ ] Production credentials
  - [ ] Webhook URL registration
  - [ ] IP whitelist configuration
- [ ] Monitoring verification
- [ ] Create admin user
- [ ] Generate overlay token
- [ ] First test donation (real money, small amount)

### Phase 10: Post-Launch (Week 15+)
- [ ] Monitor system health (24-48 jam)
- [ ] Verify webhook delivery
- [ ] Check real-time notifications
- [ ] Database performance monitoring
- [ ] Log analysis
- [ ] Bug fixes (if any)
- [ ] Performance optimization
- [ ] User feedback collection
- [ ] Documentation updates

---

## Performance Targets

### Latency
- **API Response Time**: < 200ms (p95)
- **Database Queries**: < 50ms (avg)
- **Webhook Processing**: < 100ms
- **WebSocket Event Delivery**: < 50ms
- **End-to-End Donation Flow**: < 1 second (webhook → overlay)

### Throughput
- **API Requests**: 100 req/s
- **Concurrent WebSocket Connections**: 1,000+
- **Webhook Processing**: 500 req/s

### Availability
- **Uptime Target**: 99.9% (< 43 minutes downtime/month)
- **Database Availability**: 99.95%
- **Redis Availability**: 99.9%

### Scalability
- **Donations per Day**: 10,000+
- **Storage Growth**: ~1GB per 100,000 donations
- **Database Size**: Plan for 1 million donations (~10GB)

---

## Risk Mitigation

### Technical Risks

**Risk 1: Webhook Delivery Failure**
- **Mitigation**: Implement payment status polling sebagai fallback
- **Frequency**: Poll setiap 30 detik untuk PENDING payments
- **Timeout**: Stop polling setelah payment expired

**Risk 2: Double Notification**
- **Mitigation**: Idempotency key dengan Redis (24h TTL)
- **Verification**: Check before processing webhook
- **Logging**: Log semua duplicate attempts

**Risk 3: WebSocket Connection Drop**
- **Mitigation**: Auto-reconnect mechanism dengan exponential backoff
- **Heartbeat**: Ping setiap 30 detik
- **Fallback**: SSE implementation sebagai alternative

**Risk 4: Payment Provider Downtime**
- **Mitigation**: Multi-provider support (Midtrans + Xendit)
- **Monitoring**: Health check provider APIs
- **Fallback**: Auto-switch ke backup provider

**Risk 5: Database Connection Pool Exhaustion**
- **Mitigation**: Connection pool tuning (max 25 connections)
- **Monitoring**: Track active connections
- **Alert**: Trigger saat connections > 80%

### Security Risks

**Risk 6: Webhook Signature Bypass**
- **Mitigation**: Multi-layer verification (signature + IP + timestamp)
- **Logging**: Log all failed verifications
- **Alert**: Trigger pada multiple failures

**Risk 7: DDoS Attack**
- **Mitigation**: Rate limiting per IP, NGINX request limiting
- **Protection**: Cloudflare (optional) untuk additional DDoS protection
- **Monitoring**: Track request patterns

**Risk 8: Secret Exposure**
- **Mitigation**: Never commit secrets to git, use .env files
- **Rotation**: Rotate secrets quarterly
- **Audit**: Regular secret scan dengan tools

---

## Maintenance Plan

### Daily Tasks
- [ ] Monitor system health dashboard
- [ ] Check error logs
- [ ] Verify webhook delivery success rate
- [ ] Check database performance metrics

### Weekly Tasks
- [ ] Review webhook logs for anomalies
- [ ] Manual reconciliation (if needed)
- [ ] Database backup verification
- [ ] Performance metrics review
- [ ] Security log review

### Monthly Tasks
- [ ] Database optimization (VACUUM, REINDEX)
- [ ] Log rotation & cleanup
- [ ] SSL certificate renewal check
- [ ] Dependency updates review
- [ ] Security patches application
- [ ] Backup restore testing
- [ ] Analytics & reporting

### Quarterly Tasks
- [ ] Secret rotation (JWT, API keys)
- [ ] Security audit
- [ ] Performance benchmarking
- [ ] Capacity planning review
- [ ] Disaster recovery drill

---

## Success Metrics

### Technical KPIs
- **API Success Rate**: > 99.9%
- **Webhook Processing Success**: > 99.5%
- **WebSocket Delivery Success**: > 99%
- **Payment Success Rate**: > 95% (successful payments / total attempts)
- **Average Latency**: < 200ms

### Business KPIs
- **Donation Completion Rate**: > 70% (completed / initiated)
- **Average Donation Amount**: Track trend
- **Daily Active Donors**: Track growth
- **Payment Method Distribution**: QRIS vs VA vs E-Wallet
- **Peak Concurrent Viewers**: Correlate with donation volume

### Operational KPIs
- **Mean Time to Detection (MTTD)**: < 5 minutes
- **Mean Time to Resolution (MTTR)**: < 30 minutes
- **Deployment Frequency**: Weekly (post-launch)
- **Change Failure Rate**: < 5%
- **Database Query Performance**: < 50ms average

---

## Future Enhancements (Post-MVP)

### Phase 11: Advanced Features
- [ ] Subscription/recurring donations
- [ ] Goal tracking & progress bars
- [ ] Donor leaderboard (public opt-in)
- [ ] Custom donation alerts & sounds
- [ ] Multi-language support
- [ ] Mobile app (React Native)

### Phase 12: Analytics & Insights
- [ ] Advanced analytics dashboard
- [ ] Revenue forecasting
- [ ] Donor behavior analysis
- [ ] A/B testing framework
- [ ] Export functionality (CSV, PDF)

### Phase 13: Integration & Automation
- [ ] Discord bot integration (donation notifications)
- [ ] Telegram bot integration
- [ ] Email notifications (donor receipts)
- [ ] SMS notifications (optional)
- [ ] Automated thank you messages
- [ ] Tax receipt generation

### Phase 14: Scalability
- [ ] Database read replicas
- [ ] Redis cluster
- [ ] CDN for static assets
- [ ] Multi-region deployment
- [ ] Kubernetes migration (if needed)

---

## Conclusion

ReveeGate adalah sistem **production-grade payment gateway** yang dirancang dengan prinsip:

✅ **Security First**: Multi-layer security, signature verification, rate limiting  
✅ **Performance**: < 1 detik end-to-end latency  
✅ **Reliability**: 99.9% uptime target, comprehensive error handling  
✅ **Maintainability**: Modular monolith, clean architecture, extensive logging  
✅ **Scalability**: Dapat handle 10,000+ donations/day  

Dengan mengikuti plan ini secara sistematis, sistem akan siap untuk **production deployment** dalam **14 minggu** dengan kualitas enterprise-grade.

**Next Steps**:
1. Setup development environment
2. Initialize Go project & dependencies
3. Create database schema
4. Implement core domain layer
5. Build payment provider adapters
6. Develop API endpoints
7. Create frontend interfaces
8. Deploy to staging
9. Testing & hardening
10. Production launch

**Philosophy**: *Build it right the first time. Plan carefully, code deliberately, test thoroughly, deploy confidently.*

---

**Document Version**: 1.0  
**Last Updated**: December 31, 2025  
**Author**: ReveeGate Team  
**Status**: Ready for Implementation
