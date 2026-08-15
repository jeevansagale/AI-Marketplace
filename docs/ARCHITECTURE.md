# Architecture

This document describes the system architecture of AI Marketplace.

---

## Overview

AI Marketplace is a **pay-per-use AI infrastructure platform**. It allows developers and autonomous AI agents to discover, pay for, and consume AI services through a unified API — without managing multiple subscriptions or provider-specific SDKs.

The core innovation is the **HTTP 402 payment flow**: instead of requiring pre-purchased credits, the gateway issues a cryptographic payment challenge on every unauthenticated request. The client authorizes payment and retries.

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────┐
│                 AI Consumer / Agent                 │
│          (browser, curl, AI agent, SDK)             │
└──────────────────────┬──────────────────────────────┘
                       │  HTTPS
                       ▼
┌─────────────────────────────────────────────────────┐
│                   API Gateway                       │
│                  (Go · :8080)                       │
│                                                     │
│  ┌────────────┐  ┌──────────┐  ┌────────────────┐  │
│  │  x402 Flow │  │ Sessions │  │ Provider Router│  │
│  │ (402/proof)│  │ (wallet) │  │ (Gemini/OpenAI)│  │
│  └────────────┘  └──────────┘  └────────────────┘  │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌────────────────┐    │
│  │  Wallet  │  │  Ledger  │  │   Payments     │    │
│  │ Service  │  │(immutable│  │ (Demo/Razorpay)│    │
│  └──────────┘  └──────────┘  └────────────────┘    │
└──────────┬──────────────────────────┬───────────────┘
           │                          │
           ▼                          ▼
  ┌─────────────────┐      ┌──────────────────────┐
  │  shared/catalog │      │    AI Providers       │
  │ (models+pricing)│      │  Gemini  │  OpenAI    │
  └─────────────────┘      └──────────────────────┘
           │
           ▼
  ┌─────────────────┐
  │  Control Plane  │
  │  (Go · :8081)   │
  └─────────────────┘
           │
           ▼
  ┌─────────────────┐
  │    PostgreSQL   │
  │   (infra/db)    │
  └─────────────────┘
```

---

## Components

### API Gateway (`apps/gateway`)

The primary entry point for all AI requests. Responsibilities:

| Responsibility | Implementation |
| --- | --- |
| HTTP 402 payment challenge | `internal/x402` |
| Session lifecycle (reserve → stream → settle) | `internal/session` |
| AI provider abstraction | `internal/provider` |
| Wallet balance management | `internal/wallet` |
| Immutable transaction ledger | `internal/ledger` |
| Payment order creation/verification | `internal/payments` |
| Settlement recording | `internal/settlement` |
| Payment channel state | `internal/store` |

**Routes:**

```
GET  /livez                         — liveness probe
GET  /healthz                       — health check with metadata

# Legacy x402 surface (OpenAI-compatible)
GET  /v1/models                     — list marketplace models
POST /v1/chat/completions           — 402-gated inference

# v2 API
GET  /api/catalog                   — marketplace catalog (INR pricing)
GET  /api/wallet                    — wallet balance
GET  /api/wallet/transactions       — transaction history
POST /api/topup                     — create payment order
POST /api/topup/verify              — verify payment + credit wallet
POST /api/sessions                  — create session (reserve funds)
POST /api/sessions/{id}/stream      — stream AI response + settle
POST /api/sessions/{id}/image       — generate image + settle
GET  /api/sessions                  — list sessions
GET  /api/ledger                    — immutable ledger entries
GET  /api/provider/gemini/models    — live Gemini model discovery
```

### Shared Catalog (`shared/catalog`)

A Go module imported by both the gateway and control plane. Contains:

- `Provider` and `Model` types
- INR pricing model (paise)
- `EstimateSessionCost()` — session cost pre-calculation
- `ProviderForModel()` — routing logic
- `FallbackProvider()` — automatic failover on quota exhaustion
- `NormalizeModelSlug()` — slug normalization and legacy alias resolution
- `FormatINR()` — consistent currency formatting

### Control Plane (`apps/controlplane`)

Administrative service for marketplace management. Connects to PostgreSQL. Currently handles catalog and store management for the demo environment.

### Frontend (`apps/web`)

Next.js 15 application. Features:

- **Marketplace** — browse and filter AI models
- **Playground** — interactive model testing with real payment flow
- **Wallet** — balance management and top-up

---

## Payment Flow (HTTP 402)

```
Client                    Gateway                   AI Provider
  │                          │                           │
  │── POST /v1/chat/completions ──►                       │
  │                          │                           │
  │◄── 402 Payment Required ─┤                           │
  │    X-402-Payment-Required│                           │
  │    (challenge + nonce)   │                           │
  │                          │                           │
  │── POST /v1/chat/completions ──►                       │
  │   X-402-Payment-Proof    │                           │
  │   (signed proof)         │                           │
  │                          │                           │
  │                          │── validateProof()         │
  │                          │── claimNonce()            │
  │                          │── debitChannel()          │
  │                          │                           │
  │                          │── Stream request ────────►│
  │                          │◄── SSE chunks ────────────│
  │◄── SSE chunks ───────────│                           │
  │                          │                           │
  │◄── [DONE] + summary ─────│                           │
```

---

## Session Lifecycle (v2 API)

```
POST /api/sessions
  ↓ Validate model slug
  ↓ Estimate session cost (prompt tokens + max output tokens)
  ↓ Reserve funds from wallet
  ↓ Return session ID + estimated cost

POST /api/sessions/{id}/stream
  ↓ Transition: reserved → streaming
  ↓ Call provider adapter (Gemini or OpenAI)
  ↓ Stream SSE chunks to client
  ↓ On quota exhaustion: automatic failover to alternate provider
  ↓ Transition: streaming → completed
  ↓ Charge actual cost (metered from token counts)
  ↓ Refund unused reserved amount
  ↓ Append ledger entries (settle + refund)
  ↓ Transition: settled → closed
  ↓ Return session summary (cost, tokens, latency)
```

### Session States

```
requested → authorized → reserved → streaming → completed → settled → closed
                                              ↘ generating ↗
                                         (on error) → failed
```

---

## Provider Abstraction

All AI providers implement the `provider.Provider` interface:

```go
type Provider interface {
    Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error)
    GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, Usage, error)
    Kind() catalog.ProviderKind
}
```

The `HTTPAdapter` implements this interface for both OpenAI-compatible APIs (Bearer token auth) and the Gemini REST API (query parameter auth, different SSE format).

**Automatic Failover:** If the primary provider returns a `quota_exceeded` error, the session service attempts to route to an alternate provider of the same model category automatically.

---

## Pricing Model

All prices are stored in **paise** (Indian Rupee, 1 INR = 100 paise).

| Field | Meaning |
| --- | --- |
| `price_base_paise` | Flat per-request fee |
| `price_prompt_paise` | Cost per 1,000 prompt tokens |
| `price_completion_paise` | Cost per 1,000 completion tokens |

Session cost estimation:

```
estimated = base + (promptTokens × pricePrompt / 1000) + (maxTokens × priceCompletion / 1000)
```

Actual cost is calculated from real token counts after generation, capped at the estimated reservation.

---

## Security Design

| Concern | Mechanism |
| --- | --- |
| Replay attacks | Nonce claim store with TTL |
| Payment authorization | Ed25519 signature verification |
| API key protection | Keys read from environment, never logged or serialized |
| Provider error sanitization | Raw provider JSON replaced with structured `ProviderError` |
| Channel debit | Atomic balance check + deduct |
| Wallet operations | Mutex-guarded in-memory state (production: PostgreSQL) |

---

## Infrastructure

| Component | Technology | Port |
| --- | --- | --- |
| API Gateway | Go (net/http) | 8080 |
| Control Plane | Go (net/http) | 8081 |
| Frontend | Next.js 15 | 3000 |
| Database | PostgreSQL 16 | 5432 |

See `infra/docker-compose/dev.yml` for local development configuration.

---

## Technology Decisions

| Decision | Rationale |
| --- | --- |
| Go standard `net/http` (not Fiber) | Minimal dependencies, production-proven, pattern-based routing in Go 1.22+ |
| SSE over WebSockets | Simpler, unidirectional, HTTP/1.1 compatible |
| In-memory wallet (demo) | Zero infrastructure for hackathon; interface preserved for PostgreSQL migration |
| Shared catalog module | Prevents pricing/routing drift between gateway and control plane |
| INR pricing | Target market; avoids float precision issues with paise integers |
