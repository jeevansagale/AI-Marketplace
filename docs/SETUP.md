# Local Development Setup

## Prerequisites

| Tool | Minimum Version | Purpose |
| --- | --- | --- |
| Go | 1.22 | Backend services |
| Node.js | 18+ | Frontend |
| Docker | 24+ | Infrastructure (PostgreSQL, Redis) |
| Git | any | Source control |

---

## Quick Start

### 1. Clone

```bash
git clone https://github.com/jeevansagale/AI-Marketplace.git
cd AI-Marketplace
```

### 2. Environment

```bash
cp .env.example .env
```

Edit `.env` and set at minimum:

```
OMNI_GEMINI_API_KEY=your-gemini-key
OMNI_OPENAI_API_KEY=your-openai-key   # optional for demo
OMNI_MODE=demo
```

### 3. Start infrastructure

```bash
docker compose -f infra/docker-compose/dev.yml up -d db redis
```

### 4. Run the gateway

```bash
cd apps/gateway
go run ./cmd/gateway
# Listening on :8080
```

### 5. Run the frontend

```bash
cd apps/web
npm install
npm run dev
# Available at http://localhost:3000
```

---

## Running Tests

```bash
# Gateway tests
cd apps/gateway && go test ./...

# Catalog tests
cd shared/catalog && go test ./...
```

---

## Environment Variables

See `.env.example` for the full list with descriptions.

| Variable | Default | Description |
| --- | --- | --- |
| `OMNI_HTTP_ADDR` | `:8080` | Gateway listen address |
| `OMNI_GEMINI_API_KEY` | — | Google Gemini API key |
| `OMNI_OPENAI_API_KEY` | — | OpenAI API key |
| `OMNI_MODE` | `demo` | `demo` skips real payment verification |
| `OMNI_DEMO_WALLET_BALANCE` | `1000000` | Demo wallet balance in paise |
| `NEXT_PUBLIC_GATEWAY_URL` | `http://localhost:8080` | Gateway URL for frontend |

---

## Makefile Targets

```bash
make run          # Start gateway
make test         # Run all tests
make fmt          # Format Go code
make lint         # Lint Go code
make build        # Build all Go binaries
```

---

## Demo Mode

Set `OMNI_MODE=demo` to run without a real payment provider.

In demo mode:

- The wallet is pre-seeded with `OMNI_DEMO_WALLET_BALANCE` paise
- Payment verification is bypassed (any payment proof is accepted)
- All other functionality (sessions, streaming, ledger) works normally

This is the default mode for local development.
