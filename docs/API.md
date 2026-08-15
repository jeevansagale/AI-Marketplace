# API Reference

Base URL: `http://localhost:8080` (development)

All monetary values are **integer paise** (₹1 = 100 paise).

---

## Health

### `GET /livez`

Liveness probe. Returns `200 OK` with `"ok"`.

### `GET /healthz`

Health check with service metadata.

**Response:**

```json
{
  "status": "ok",
  "mode": "demo",
  "wallet_id": "wallet-demo",
  "version": "1.0.0"
}
```

---

## Catalog

### `GET /api/catalog`

Returns all active providers and models with INR pricing.

**Response:**

```json
{
  "providers": [
    {
      "id": "gemini",
      "name": "Google Gemini",
      "slug": "gemini",
      "base_url": "https://generativelanguage.googleapis.com",
      "kind": "gemini",
      "status": "active"
    }
  ],
  "models": [
    {
      "id": "gemini-2.0-flash",
      "provider_id": "gemini",
      "name": "Gemini 2.0 Flash",
      "slug": "gemini-2.0-flash",
      "category": "chat",
      "description": "Fast, efficient model for chat and reasoning",
      "context_window": 1048576,
      "price_base_paise": 0,
      "price_prompt_paise": 8,
      "price_completion_paise": 32,
      "provider": "gemini",
      "p90_latency_ms": 1200,
      "throughput_tps": 120,
      "is_active": true,
      "supports_chat": true,
      "supports_streaming": true
    }
  ]
}
```

---

## Wallet

### `GET /api/wallet`

Returns the current wallet balance.

**Response:**

```json
{
  "wallet_id": "wallet-demo",
  "balance_paise": 500000,
  "reserved_paise": 12000,
  "available_paise": 488000
}
```

### `GET /api/wallet/transactions`

Returns transaction history (newest first).

**Response:**

```json
{
  "transactions": [
    {
      "id": "tx-1",
      "timestamp": "2024-01-15T10:30:00Z",
      "kind": "reserve",
      "amount_paise": -12000,
      "reservation": "R-1",
      "reference": "session:sess-abc123"
    }
  ]
}
```

**Transaction kinds:**

- `topup` — wallet credited
- `reserve` — funds held for a session
- `charge` — actual session cost deducted
- `refund` — unused reserved funds returned
- `reserve_void` — reservation cancelled without charge

---

## Top-Up

### `POST /api/topup`

Creates a payment order to add funds to the wallet.

**Request:**

```json
{
  "amount_inr": 500
}
```

**Response:**

```json
{
  "order_id": "order_abc123",
  "amount_paise": 50000,
  "status": "created",
  "checkout_url": "https://checkout.razorpay.com/..."
}
```

### `POST /api/topup/verify`

Verifies a completed payment and credits the wallet.

**Request:**

```json
{
  "order_id": "order_abc123"
}
```

**Response:**

```json
{
  "status": "credited",
  "credited_paise": 50000
}
```

---

## Sessions

### `POST /api/sessions`

Creates a session: validates model, estimates cost, reserves funds.

**Request:**

```json
{
  "model": "gemini-2.0-flash",
  "max_tokens": 1024
}
```

**Response:**

```json
{
  "id": "sess-abc123",
  "wallet_id": "wallet-demo",
  "model_id": "gemini-2.0-flash",
  "model_name": "Gemini 2.0 Flash",
  "provider_id": "gemini",
  "provider_name": "Google Gemini",
  "reservation_id": "R-1",
  "stage": "reserved",
  "estimated_paise": 12000,
  "actual_cost_paise": 0,
  "refunded_paise": 0,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### `POST /api/sessions/{id}/stream`

Streams an AI response over SSE, then settles the session.

**Request:**

```json
{
  "messages": [
    { "role": "user", "content": "Explain quantum entanglement in one paragraph." }
  ],
  "max_tokens": 1024
}
```

**Response:** `text/event-stream`

```
data: {"choices":[{"delta":{"content":"Quantum"}}]}

data: {"choices":[{"delta":{"content":" entanglement"}}]}

data: {"summary":{"session_id":"sess-abc123","reserved_paise":12000,"actual_cost_paise":8400,"refunded_paise":3600,"prompt_tokens":12,"completion_tokens":87,"latency_ms":1843},"usage":{...}}

data: [DONE]
```

### `POST /api/sessions/{id}/image`

Generates an image (for image-capable models) and settles the session.

**Request:**

```json
{
  "prompt": "A futuristic city with flying cars at sunset"
}
```

**Response:**

```json
{
  "session_id": "sess-abc123",
  "reserved_paise": 50000,
  "actual_cost_paise": 50000,
  "refunded_paise": 0,
  "image_url": "https://..."
}
```

### `GET /api/sessions`

Returns all sessions for the current wallet.

**Response:**

```json
{
  "sessions": [
    {
      "id": "sess-abc123",
      "stage": "closed",
      "model_name": "Gemini 2.0 Flash",
      "estimated_paise": 12000,
      "actual_cost_paise": 8400,
      "usage": {
        "prompt_tokens": 12,
        "completion_tokens": 87,
        "latency_ms": 1843,
        "request_count": 1
      }
    }
  ]
}
```

---

## Ledger

### `GET /api/ledger`

Returns the immutable transaction ledger.

**Response:**

```json
{
  "entries": [
    {
      "id": 1,
      "timestamp": "2024-01-15T10:30:05Z",
      "kind": "settle",
      "wallet_id": "wallet-demo",
      "provider_id": "gemini",
      "model_id": "gemini-2.0-flash",
      "reservation": "R-1",
      "amount": 8400,
      "reference": "sess-abc123",
      "status": "ok"
    }
  ],
  "len": 1
}
```

---

## Legacy x402 Surface

### `GET /v1/models`

OpenAI-compatible model listing. Returns marketplace models in OpenAI format.

### `POST /v1/chat/completions`

HTTP 402 payment-gated inference endpoint. Issues a payment challenge on the first request; retries with `X-402-Payment-Proof` header are validated and proxied to the provider.

---

## Provider Discovery

### `GET /api/provider/gemini/models`

Returns live Gemini model list from the Google API (requires valid API key).

---

## Error Format

All errors follow this structure:

```json
{
  "error": {
    "message": "human-readable error description",
    "code": "optional_error_code"
  }
}
```

Common HTTP status codes:

- `400` — invalid request body or parameters
- `402` — payment required (x402 flow)
- `404` — model or session not found
- `409` — session already in wrong state
- `500` — internal error
