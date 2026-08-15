# AI Marketplace

**Pay-per-use AI infrastructure. Discover, pay, and consume AI models through a single API.**

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-15-000000?logo=next.js)](https://nextjs.org)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

---

## Overview

AI Marketplace is a platform that lets developers and autonomous AI agents discover, pay for, and stream responses from multiple AI providers — through a single, unified API.

The core primitive is the **HTTP 402 payment flow**: instead of managing subscriptions, the gateway issues a cryptographic payment challenge on each request. The client authorizes a micropayment and the inference is served immediately.

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│              AI Consumer / Agent / Browser           │
└──────────────────────┬───────────────────────────────┘
                       │ HTTPS
                       ▼
┌──────────────────────────────────────────────────────┐
│                   API Gateway  (:8080)               │
│   x402 Flow · Sessions · Wallet · Ledger · Payments  │
└──────────┬───────────────────────────┬───────────────┘
           │                           │
           ▼                           ▼
  ┌──────────────────┐       ┌──────────────────────┐
  │  shared/catalog  │       │    AI Providers       │
  │ (models, pricing)│       │  Gemini  ·  OpenAI   │
  └──────────────────┘       └──────────────────────┘
           │
           ▼
  ┌──────────────────┐
  │  Control Plane   │
  │    (:8081)       │
  └──────────────────┘
```

Full architecture documentation: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)

---

## Tech Stack

| Layer | Technology |
| --- | --- |
| Backend | Go 1.22, `net/http` |
| Frontend | Next.js 15, React 19, TypeScript |
| Styling | Tailwind CSS v4 |
| State | Zustand, TanStack Query |
| Animations | Framer Motion |
| Infrastructure | Docker, PostgreSQL 16 |
| AI Providers | Google Gemini, OpenAI |

---

## Features

- **HTTP 402 Payment Flow** — cryptographic per-request payment challenges; no subscriptions required
- **Unified AI API** — single endpoint for Gemini, OpenAI, and more; OpenAI-compatible surface
- **INR Wallet** — prepaid wallet with session reservations, metered charging, and automatic refunds
- **Streaming Inference** — SSE-based token streaming with live cost metering
- **Image Generation** — image generation with automatic cost settlement
- **Provider Failover** — automatic retry on alternate provider when quota is exhausted
- **Immutable Ledger** — append-only transaction log for every cost event
- **Marketplace** — browse and filter AI models by category, provider, price, and capability
- **Playground** — interactive chat UI with live cost display and session timeline
- **Demo Mode** — full functionality without a real payment provider

---

## Project Structure

```
AI-Marketplace/
├── apps/
│   ├── gateway/          # Go API Gateway (port 8080)
│   ├── controlplane/     # Go Control Plane (port 8081)
│   └── web/              # Next.js Frontend (port 3000)
├── shared/
│   └── catalog/          # Shared Go module: models, pricing, routing
├── infra/
│   ├── docker/           # Dockerfiles
│   └── docker-compose/   # Dev stack
├── docs/                 # Project documentation
└── scripts/              # Utility scripts
```

Full annotated structure: [`docs/PROJECT_STRUCTURE.md`](docs/PROJECT_STRUCTURE.md)

---

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 18+
- Docker

### 1. Clone and configure

```bash
git clone https://github.com/jeevansagale/AI-Marketplace.git
cd AI-Marketplace
cp .env.example .env
# Edit .env: set OMNI_GEMINI_API_KEY
```

### 2. Start services

```bash
# Infrastructure
docker compose -f infra/docker-compose/dev.yml up -d db redis

# Gateway
cd apps/gateway && go run ./cmd/gateway

# Frontend (new terminal)
cd apps/web && npm install && npm run dev
```

### 3. Open

- Frontend: <http://localhost:3000>
- API: <http://localhost:8080/healthz>

### Windows

```bat
Run.bat
```

### Linux / macOS

```bash
./Run.sh
```

---

## Docker

Build and run everything with Docker Compose:

```bash
docker compose -f infra/docker-compose/dev.yml up --build
```

---

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `OMNI_HTTP_ADDR` | `:8080` | Gateway listen address |
| `OMNI_GEMINI_API_KEY` | — | Google Gemini API key |
| `OMNI_OPENAI_API_KEY` | — | OpenAI API key (optional) |
| `OMNI_MODE` | `demo` | `demo` bypasses payment verification |
| `OMNI_DEMO_WALLET_BALANCE` | `1000000` | Demo wallet starting balance (paise) |
| `NEXT_PUBLIC_GATEWAY_URL` | `http://localhost:8080` | Gateway URL for frontend |

Full variable reference: [`.env.example`](.env.example)

---

## API

Full API reference: [`docs/API.md`](docs/API.md)

Key endpoints:

```
GET  /api/catalog                    — Browse AI models with INR pricing
GET  /api/wallet                     — Wallet balance
POST /api/sessions                   — Create session (reserve funds)
POST /api/sessions/{id}/stream       — Stream AI response (SSE)
POST /api/sessions/{id}/image        — Generate image
GET  /api/ledger                     — Transaction history
POST /v1/chat/completions            — OpenAI-compatible (HTTP 402 gated)
```

---

## Documentation

| Document | Description |
| --- | --- |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | System design, components, data flow |
| [`docs/API.md`](docs/API.md) | Complete API reference with examples |
| [`docs/SETUP.md`](docs/SETUP.md) | Local development setup guide |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | Docker and production deployment |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | How to contribute |
| [`docs/PROJECT_STRUCTURE.md`](docs/PROJECT_STRUCTURE.md) | Annotated directory tree |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | Planned features and improvements |
| [`docs/CHANGELOG.md`](docs/CHANGELOG.md) | Version history |

---

## Contributing

See [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md).

---

## License

MIT
