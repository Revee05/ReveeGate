# ReveeGate

<p align="center">
  <img src="docs/logo.png" alt="ReveeGate Logo" width="200">
</p>

<p align="center">
  <strong>Real-Time Live Streaming Donation Payment Gateway</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#documentation">Documentation</a> •
  <a href="#deployment">Deployment</a> •
  <a href="#contributing">Contributing</a>
</p>

---

## 🎯 Overview

ReveeGate is a production-ready, real-time donation payment gateway designed specifically for live streamers in Indonesia. It provides seamless integration with popular Indonesian payment providers (Midtrans & Xendit) and delivers instant donation notifications through WebSocket connections.

### Key Highlights

- **⚡ Real-Time**: < 1 second end-to-end latency from payment to overlay notification
- **🔒 Secure**: Bank-grade security with HTTPS, webhook signature verification, and rate limiting
- **🇮🇩 Indonesia-Focused**: Supports QRIS, GoPay, OVO, DANA, ShopeePay, and bank transfers
- **📱 Mobile-First**: Responsive donor page optimized for mobile payments
- **🎨 Customizable**: Beautiful overlay with easy customization options

## ✨ Features

### Payment Processing
- Multiple payment providers (Midtrans, Xendit)
- QRIS (universal QR code)
- E-Wallets: GoPay, OVO, DANA, ShopeePay, LinkAja
- Virtual Accounts: BCA, BNI, BRI, Mandiri, Permata
- Automatic webhook handling and verification
- Idempotent payment processing

### Real-Time Notifications
- WebSocket-based instant notifications
- Redis Pub/Sub for scalable event distribution
- Connection heartbeat and auto-reconnection
- Support for multiple overlay instances

### Admin Dashboard
- JWT-based authentication
- Donation statistics and reporting
- Manual payment reconciliation
- Webhook log viewer
- Overlay token management

### Security
- HTTPS encryption
- Webhook signature verification
- Rate limiting with Redis
- CORS protection
- Security headers (CSP, HSTS, etc.)
- Input validation and sanitization

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Redis 7+
- Docker & Docker Compose (optional)

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/reveegate/reveegate.git
cd reveegate

# Copy environment file
cp .env.example .env

# Edit .env with your configuration
# Important: Set your payment provider API keys

# Start all services
docker-compose up -d

# Run migrations
docker-compose --profile migrate run --rm migrate

# Check logs
docker-compose logs -f app
```

### Manual Installation

```bash
# Clone the repository
git clone https://github.com/reveegate/reveegate.git
cd reveegate

# Install dependencies
go mod download

# Copy and configure environment
cp .env.example .env
# Edit .env with your settings

# Run migrations
make migrate-up

# Build and run
make run
```

## 📖 Documentation

### API Endpoints

#### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/donations` | Create new donation |
| GET | `/api/v1/donations/{id}` | Get donation details |
| GET | `/api/v1/donations/{id}/status` | Check payment status |

#### Webhook Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/webhooks/midtrans` | Midtrans webhook callback |
| POST | `/api/v1/webhooks/xendit` | Xendit webhook callback |

#### Admin Endpoints (Protected)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/admin/login` | Admin authentication |
| POST | `/api/v1/admin/refresh` | Refresh access token |
| GET | `/api/v1/admin/dashboard` | Dashboard statistics |
| GET | `/api/v1/admin/donations` | List all donations |
| POST | `/api/v1/admin/reconcile` | Manual reconciliation |
| POST | `/api/v1/admin/overlay-token` | Generate overlay token |

#### WebSocket Endpoints

| Endpoint | Description |
|----------|-------------|
| `/ws/overlay?token={token}` | Overlay connection |
| `/ws/admin` | Admin real-time updates |

### Create Donation Request

```json
{
  "donor_name": "John Doe",
  "donor_email": "john@example.com",
  "message": "Keep up the great streams!",
  "amount": 50000,
  "payment_method": "qris"
}
```

### Donation Response

```json
{
  "id": "uuid",
  "donor_name": "John Doe",
  "message": "Keep up the great streams!",
  "amount": 50000,
  "status": "pending",
  "payment_info": {
    "payment_id": "uuid",
    "provider": "midtrans",
    "method": "qris",
    "qr_code_url": "https://...",
    "expires_at": "2024-01-01T12:00:00Z"
  }
}
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         ReveeGate                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────┐     ┌──────────┐     ┌──────────────────────┐   │
│   │  Donor   │────▶│   HTTP   │────▶│   Donation Service   │   │
│   │   Page   │     │  Server  │     │                      │   │
│   └──────────┘     └──────────┘     └──────────┬───────────┘   │
│                          │                      │               │
│                          │                      ▼               │
│   ┌──────────┐     ┌─────▼────┐     ┌──────────────────────┐   │
│   │ Payment  │────▶│ Webhook  │────▶│   Payment Provider   │   │
│   │ Provider │◀────│ Handler  │     │   (Midtrans/Xendit)  │   │
│   └──────────┘     └──────────┘     └──────────────────────┘   │
│                          │                      │               │
│                          ▼                      │               │
│   ┌──────────┐     ┌──────────┐     ┌──────────▼───────────┐   │
│   │ Overlay  │◀────│WebSocket │◀────│    Redis Pub/Sub     │   │
│   │   Page   │     │   Hub    │     │                      │   │
│   └──────────┘     └──────────┘     └──────────────────────┘   │
│                                                  │               │
│                                           ┌──────▼──────┐       │
│                                           │  PostgreSQL │       │
│                                           └─────────────┘       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_PORT` | Server port | 8080 |
| `APP_ENVIRONMENT` | Environment (development/production) | development |
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PASSWORD` | PostgreSQL password | - |
| `REDIS_ADDR` | Redis address | localhost:6379 |
| `JWT_SECRET` | JWT signing secret | - |
| `MIDTRANS_SERVER_KEY` | Midtrans server key | - |
| `MIDTRANS_IS_PRODUCTION` | Use production Midtrans | false |
| `XENDIT_SECRET_KEY` | Xendit secret key | - |

See [.env.example](.env.example) for all available options.

## 🚢 Deployment

### Production Checklist

- [ ] Set `APP_ENVIRONMENT=production`
- [ ] Configure strong `JWT_SECRET` (32+ characters)
- [ ] Set production payment provider keys
- [ ] Enable HTTPS (via reverse proxy)
- [ ] Configure proper CORS origins
- [ ] Set up database backups
- [ ] Configure monitoring and alerting
- [ ] Review rate limit settings

### Using Docker

```bash
# Build production image
docker build -t reveegate:latest .

# Run with Docker Compose
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### VPS Deployment

1. Install Docker and Docker Compose
2. Clone repository and configure `.env`
3. Run `docker-compose up -d`
4. Set up Nginx reverse proxy with SSL
5. Configure firewall (allow ports 80, 443)

## 🧪 Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run benchmarks
make bench
```

## 📊 Monitoring

ReveeGate exposes health endpoints:

- `GET /health` - Basic health check
- `GET /ready` - Readiness check (DB & Redis connectivity)
- `GET /api/v1/admin/health` - Detailed health (authenticated)

### Recommended Monitoring Stack

- **Prometheus** for metrics collection
- **Grafana** for visualization
- **Loki** for log aggregation
- **Alertmanager** for alerts

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Midtrans](https://midtrans.com) for payment processing
- [Xendit](https://xendit.co) for payment processing
- [chi](https://github.com/go-chi/chi) for HTTP routing
- [pgx](https://github.com/jackc/pgx) for PostgreSQL driver
- [gorilla/websocket](https://github.com/gorilla/websocket) for WebSocket

---

<p align="center">
  Made with ❤️ for Indonesian Streamers
</p>
