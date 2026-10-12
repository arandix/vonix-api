# Vonix Business API

[![CI](https://github.com/vonix/api/actions/workflows/ci.yml/badge.svg)](https://github.com/vonix/api/actions/workflows/ci.yml)
[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

The **Vonix Business API** lets you integrate fiat payments, crypto deposits & withdrawals, and real-time event webhooks into your products — all through a single, secure REST interface.

**Base URL:** `https://api.getvonix.com`  
**Docs:** [docs.getvonix.com/api](https://docs.getvonix.com/api)  
**OpenAPI spec:** [`docs/openapi.yaml`](docs/openapi.yaml)

---

## Features

| Category | What you can do |
|---|---|
| **Accounts** | View fiat and crypto account balances |
| **Fiat Transfers** | Send via SEPA, SWIFT, or Faster Payments |
| **Crypto** | Generate deposit addresses · Send BTC, ETH, USDT, BNB |
| **Webhooks** | Receive signed real-time events for transfers and deposits |
| **API Keys** | Create scoped keys with optional expiry |
| **Security** | HMAC-SHA256 webhook signatures · Per-key scope enforcement · Rate limiting |

---

## Authentication

All requests require your API key in the `Authorization` header:

```
Authorization: Bearer vx_live_...
```

Alternatively, use the `X-API-Key` header.

### Scopes

| Scope | Description |
|---|---|
| `accounts:read` | View balances, accounts, and transactions |
| `transfers:write` | Initiate fiat transfers |
| `crypto:write` | Send crypto, generate deposit addresses |
| `webhooks:write` | Create and delete webhooks and API keys |

---

## Quick Start

### 1. Get your balances

```bash
curl https://api.getvonix.com/v1/balance \
  -H "Authorization: Bearer vx_live_YOUR_KEY"
```

```json
{
  "business_id": "biz_01j9...",
  "balances": [
    { "currency": "USD", "available": "24850.00", "pending": "1200.00", "total": "26050.00" },
    { "currency": "USDT", "available": "8400.000000", "pending": "0.000000", "total": "8400.000000" },
    { "currency": "BTC", "available": "0.14820000", "pending": "0.00000000", "total": "0.14820000" }
  ],
  "as_of": "2024-01-15T14:32:00Z"
}
```

### 2. Send a fiat payment

```bash
curl -X POST https://api.getvonix.com/v1/transfers \
  -H "Authorization: Bearer vx_live_YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "currency": "USD",
    "amount": "500.00",
    "recipient": {
      "name": "ACME Corp",
      "iban": "GB29NWBK60161331926819"
    },
    "reference": "INV-2024-007"
  }'
```

### 3. Get a USDT deposit address

```bash
curl "https://api.getvonix.com/v1/crypto/deposit-address?currency=USDT&network=ethereum" \
  -H "Authorization: Bearer vx_live_YOUR_KEY"
```

```json
{
  "currency": "USDT",
  "network": "ethereum",
  "address": "0x742d35Cc6634C0532925a3b8D4C9b3E5b3a1234",
  "qr_code": "https://api.qrserver.com/v1/create-qr-code/?data=0x742d35..."
}
```

### 4. Send BTC

```bash
curl -X POST https://api.getvonix.com/v1/crypto/send \
  -H "Authorization: Bearer vx_live_YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "currency": "BTC",
    "network": "bitcoin",
    "amount": "0.01000000",
    "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }'
```

---

## Webhooks

Register an endpoint and receive signed event payloads in real time.

### Register

```bash
curl -X POST https://api.getvonix.com/v1/webhooks \
  -H "Authorization: Bearer vx_live_YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://yourapp.com/hooks/vonix",
    "events": ["transfer.completed", "transfer.failed", "deposit.confirmed"]
  }'
```

> **Important:** The `secret` in the response is shown **once only**. Store it securely.

### Available events

| Event | Trigger |
|---|---|
| `transfer.created` | Payment initiated |
| `transfer.completed` | Payment settled |
| `transfer.failed` | Payment rejected or timed out |
| `deposit.received` | Crypto deposit detected on-chain |
| `deposit.confirmed` | Crypto deposit fully confirmed |

### Verify signatures

Every delivery includes two headers:

- `Vonix-Timestamp` — Unix timestamp
- `Vonix-Signature` — `sha256=HMAC_SHA256(secret, timestamp + "." + body)`

```javascript
// Node.js / Express
const crypto = require("crypto");

app.post("/hooks/vonix", express.raw({ type: "application/json" }), (req, res) => {
  const ts  = req.headers["vonix-timestamp"];
  const sig = req.headers["vonix-signature"];

  const expected = "sha256=" + crypto
    .createHmac("sha256", process.env.VONIX_WEBHOOK_SECRET)
    .update(`${ts}.${req.body.toString()}`)
    .digest("hex");

  if (!crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(sig))) {
    return res.status(401).send("Bad signature");
  }

  const event = JSON.parse(req.body);
  // handle event.event, event.data ...
  res.sendStatus(200);
});
```

```python
# Python / Flask
import hashlib, hmac

@app.post("/hooks/vonix")
def vonix_webhook():
    ts  = request.headers["Vonix-Timestamp"]
    sig = request.headers["Vonix-Signature"]
    body = request.get_data()

    expected = "sha256=" + hmac.new(
        WEBHOOK_SECRET.encode(), f"{ts}.".encode() + body, hashlib.sha256
    ).hexdigest()

    if not hmac.compare_digest(expected, sig):
        abort(401)

    event = request.json
    # handle event["event"], event["data"] ...
    return "ok"
```

---

## API Keys

Create scoped keys programmatically:

```bash
curl -X POST https://api.getvonix.com/v1/api-keys \
  -H "Authorization: Bearer vx_live_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Payments service",
    "scopes": ["accounts:read", "transfers:write"]
  }'
```

> The full key is returned **once only** in the `key` field. Store it immediately.

---

## Supported Crypto Assets

| Currency | Networks |
|---|---|
| BTC | bitcoin |
| ETH | ethereum |
| USDT | ethereum · tron · bnb_smart_chain |
| BNB | bnb_smart_chain |

---

## Error Codes

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `invalid_request` | Malformed JSON body |
| 401 | `missing_api_key` | No API key provided |
| 401 | `invalid_api_key` | Key invalid, revoked, or expired |
| 403 | `insufficient_scope` | Key lacks required scope |
| 404 | `not_found` | Resource not found |
| 422 | `validation_error` | Field validation failed |
| 429 | `rate_limited` | 100 requests per minute exceeded |
| 500 | `internal_error` | Server-side error |

---

## Code Examples

| Language | Location |
|---|---|
| Node.js | [`examples/node/index.js`](examples/node/index.js) |
| Python | [`examples/python/main.py`](examples/python/main.py) |
| cURL | [`examples/curl/quickstart.sh`](examples/curl/quickstart.sh) |

---

## Development

### Prerequisites

- Go 1.23+
- Docker (optional)

### Run locally

```bash
git clone https://github.com/vonix/api.git
cd api
cp .env.example .env
make run
```

The server starts on `http://localhost:8080`. A test API key is printed to stdout on startup.

### Test

```bash
make test
```

### Build Docker image

```bash
make docker-build
make docker-run
```

---

## Architecture

```
vonix-api/
├── cmd/server/          # Entry point — router, middleware, server startup
├── internal/
│   ├── auth/            # API key generation, hashing, scope middleware
│   ├── handlers/        # HTTP handlers (accounts, transfers, crypto, webhooks, keys)
│   ├── models/          # Shared types and request/response schemas
│   └── webhook/         # Webhook store, HMAC-signed delivery with retries
├── docs/
│   └── openapi.yaml     # OpenAPI 3.1 specification
└── examples/            # Node.js, Python, cURL quickstarts
```

**Production checklist before going live:**
- Replace in-memory stores with PostgreSQL
- Integrate a custody provider (BitGo or Fireblocks) for crypto
- Connect to payment rails for fiat (SEPA/SWIFT)
- Add Redis for distributed rate limiting and idempotency keys
- Set up structured logging and tracing (OpenTelemetry)

---

## License

MIT © [Vonix Financial Group Ltd](https://getvonix.com)
