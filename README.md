# AI Marketplace

**Pay-per-use AI infrastructure. Discover, pay for, and consume AI models through a unified API.**

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-15-000000?logo=next.js)](https://nextjs.org)

---

## Overview

AI Marketplace is a platform for developers and autonomous AI agents to discover, pay for, and stream responses from multiple AI providers through a unified API.

The core payment primitive is an HTTP 402 challenge flow. The gateway can request payment authorization before serving an inference request, while tracking usage and settlement.

> Payment behavior depends on the configured mode and provider integration. The repository includes a demo mode; demo-mode behavior should not be represented as live payment settlement.

## Architecture

```text
AI Consumer / Agent
        |
        v
   API Gateway
        |
   +----+----+
   |         |
   v         v
Catalog   AI Providers
   |
   v
Control Plane
```

See [architecture documentation](docs/ARCHITECTURE.md) for component details and data flow.

## Core Capabilities

- HTTP 402 payment-challenge flow.
- Unified API surface for supported AI providers.
- Wallet balance and session reservation logic.
- Streamed inference with usage metering.
- Image-generation request flow.
- Provider fallback handling.
- Append-only transaction ledger.
- Model catalog and interactive playground.
- Demo mode for local exploration without a live payment provider.

## Technology

| Area | Technologies |
|---|---|
| Gateway and control plane | Go, `net/http` |
| Web interface | Next.js, React, TypeScript |
| Infrastructure | Docker, PostgreSQL |
| AI providers | Google Gemini, OpenAI |

## Project Structure

```text
AI-Marketplace/
├── apps/
│   ├── gateway/
│   ├── controlplane/
│   └── web/
├── shared/
│   └── catalog/
├── infra/
├── docs/
└── scripts/
```

## Quick Start

### Requirements

- Go 1.22+
- Node.js 18+
- Docker

### Configure

```bash
git clone https://github.com/jeevansagale/AI-Marketplace.git
cd AI-Marketplace
cp .env.example .env
```

Add provider credentials to the local `.env` file as required. Never commit real API keys.

### Start infrastructure

```bash
docker compose -f infra/docker-compose/dev.yml up -d db redis
```

### Start the gateway

```bash
cd apps/gateway
go run ./cmd/gateway
```

### Start the web interface

In another terminal:

```bash
cd apps/web
npm install
npm run dev
```

The default development endpoints are documented in the project configuration. The gateway health endpoint is `/healthz`.

## Configuration

See [`.env.example`](.env.example) for supported variables. Keep local credentials out of version control.

## API and Engineering Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Setup](docs/SETUP.md)
- [Deployment](docs/DEPLOYMENT.md)
- [Project Structure](docs/PROJECT_STRUCTURE.md)
- [Roadmap](docs/ROADMAP.md)
- [Changelog](docs/CHANGELOG.md)

## Contributing

See [contribution guidelines](docs/CONTRIBUTING.md).

## License

No root-level license file is currently included. Unless a license is added, the repository should be treated as source available for viewing, not as granting general permission to reuse or redistribute the code.
## Security and Demo Boundaries

The HTTP 402 demonstration verifies the Ed25519 signature against the public key encoded in the Algorand sender address, checks the server-issued nonce and its expiry, rejects replayed proofs, and binds the seeded demo channel to `OMNI_DEMO_SENDER_ADDRESS`. Set that variable to the public address corresponding to the signing account; the private key must remain client-side.

This is still a **local/hackathon demo, not production payment infrastructure**:

- The channel, wallet, nonce registry, session state and ledger are in-memory and reset on restart.
- The seeded channel is a demo balance, not an on-chain escrow account or real USDC balance.
- Top-ups use a simulated payment provider; a successful demo top-up is not a real payment.
- The wallet/session API uses a single demo wallet and does not provide production user authentication or account isolation.
- Do not use this system to process money, hold customer balances or make security guarantees.

Production use requires persistent transactional storage, authenticated users, a real payment-provider webhook, chain/escrow verification, channel ownership and authorization, rate limiting, audit controls and a separate security review.
