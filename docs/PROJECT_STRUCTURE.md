# Project Structure

This document provides an annotated overview of every directory and key file in the repository.

---

```
AI-Marketplace/
│
├── apps/                          # Application services (monorepo style)
│   │
│   ├── gateway/                   # ✓ API Gateway — Go service (port 8080)
│   │   ├── cmd/
│   │   │   └── gateway/
│   │   │       └── main.go        # Entrypoint: wires all services, registers routes
│   │   ├── internal/
│   │   │   ├── ledger/            # Append-only transaction ledger (in-memory)
│   │   │   ├── payments/          # Payment provider abstraction (demo + real)
│   │   │   ├── provider/          # AI provider adapters (OpenAI, Gemini)
│   │   │   │   ├── provider.go    # Core: Provider interface, HTTPAdapter, Registry
│   │   │   │   └── *_test.go      # Provider request/error tests
│   │   │   ├── session/           # Session lifecycle engine (reserve→stream→settle)
│   │   │   │   └── session.go     # Core: stages, state machine, failover logic
│   │   │   ├── settlement/        # Settlement driver (records provider payouts)
│   │   │   ├── store/             # x402 channel store (nonce + balance tracking)
│   │   │   ├── wallet/            # INR wallet service (reserve/charge/refund)
│   │   │   └── x402/              # HTTP 402 protocol (challenge/proof encoding)
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   ├── controlplane/              # ✓ Control Plane — Go service (port 8081)
│   │   ├── cmd/
│   │   │   └── controlplane/
│   │   │       └── main.go        # Entrypoint: admin API
│   │   ├── internal/
│   │   │   ├── api/               # HTTP handlers for admin endpoints
│   │   │   └── store/             # PostgreSQL data access layer
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   └── web/                       # ✓ Frontend — Next.js 15 (port 3000)
│       ├── app/                   # Next.js App Router pages
│       │   ├── layout.tsx         # Root layout (shell, navigation)
│       │   ├── page.tsx           # Landing / home page
│       │   ├── marketplace/       # Model discovery and browsing
│       │   ├── playground/        # Interactive AI chat + payment flow
│       │   └── wallet/            # Balance management and top-up
│       ├── src/
│       │   ├── components/        # Shared React components
│       │   │   ├── Shell.tsx              # App shell (nav + layout wrapper)
│       │   │   ├── PageTransition.tsx     # Framer Motion page transitions
│       │   │   ├── SessionMeter.tsx       # Live session cost meter
│       │   │   ├── QuantumTokenLedger.tsx # Animated ledger display
│       │   │   ├── X402FlowVisualizer.tsx # HTTP 402 flow diagram (animated)
│       │   │   ├── ProviderRoutingAnimation.tsx # Provider routing visualization
│       │   │   ├── TopUpPaymentAnimation.tsx    # Payment flow animation
│       │   │   └── ui.tsx                 # Base UI primitives (shadcn/ui wrappers)
│       │   └── lib/
│       │       ├── api.ts         # API client (all backend calls)
│       │       ├── format.ts      # Currency and number formatters
│       │       └── stores/
│       │           └── useWallet.ts # Zustand wallet state store
│       ├── styles/
│       │   └── globals.css        # Global Tailwind CSS
│       ├── next.config.ts
│       ├── tsconfig.json
│       └── package.json
│
├── shared/                        # ✓ Shared Go modules
│   └── catalog/                   # Canonical marketplace catalog
│       ├── catalog.go             # Provider/Model types, pricing, routing
│       ├── catalog_test.go        # Catalog unit tests
│       └── go.mod
│
├── infra/                         # ✓ Infrastructure configuration
│   ├── docker/
│   │   ├── gateway.Dockerfile     # Multi-stage Go build for gateway
│   │   ├── controlplane.Dockerfile # Multi-stage Go build for control plane
│   │   └── postgres/
│   │       └── init/
│   │           └── 0001_init.sql  # Database schema initialization
│   └── docker-compose/
│       └── dev.yml                # Local development stack
│
├── scripts/                       # ⚠ Utility scripts
│   └── verify_models.go           # Model slug validation against live Gemini API
│
├── docs/                          # ✓ Project documentation
│   ├── ARCHITECTURE.md            # System design and component overview
│   ├── CONTRIBUTING.md            # Contribution guide
│   ├── PROJECT_STRUCTURE.md       # This file
│   ├── SETUP.md                   # Local development setup
│   ├── API.md                     # Complete API reference
│   ├── DEPLOYMENT.md              # Deployment guide
│   ├── CHANGELOG.md               # Version history
│   └── ROADMAP.md                 # Future development plans
