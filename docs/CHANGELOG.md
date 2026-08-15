# Changelog

All notable changes to AI Marketplace are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### Added

- Comprehensive documentation suite (ARCHITECTURE, CONTRIBUTING, API, SETUP, DEPLOYMENT, ROADMAP)
- Annotated PROJECT_STRUCTURE.md for onboarding
- Improved .gitignore (excludes build artifacts and research dumps)

---

## [0.2.0] — 2026-08-15

### Added

- Image generation support via `POST /api/sessions/{id}/image`
- Automatic provider failover on quota exhaustion
- `FallbackProvider()` routing in shared catalog
- `GET /api/provider/gemini/models` — live Gemini model discovery
- `QuantumTokenLedger` animated ledger display component
- `X402FlowVisualizer` — interactive HTTP 402 flow diagram
- `ProviderRoutingAnimation` — provider selection visualization
- `TopUpPaymentAnimation` — payment top-up flow animation
- Gemini native SSE format support (distinct from OpenAI SSE)

### Changed

- Ledger entries now include `provider_id` and `model_id` for cost attribution
- Session timeline events added for observability
- Catalog extended with `p90_latency_ms` and `throughput_tps` fields

### Fixed

- Session cost estimation now correctly handles image models (flat base fee)
- Nonce claim store correctly expires entries after TTL

---

## [0.1.0] — 2026-08-10

### Added

- HTTP 402 payment flow (`/v1/chat/completions`)
- INR prepaid wallet with reserve/charge/refund
- Session lifecycle engine (reserve → stream → settle → close)
- Shared catalog module with pricing model
- AI provider abstraction (Gemini + OpenAI compatible)
- Marketplace frontend (browse models, INR pricing)
- Playground frontend (interactive chat with live cost meter)
- Wallet frontend (balance display, top-up flow)
- Immutable ledger (`/api/ledger`)
- Demo mode (no real payment required)
- Docker Compose development stack
- PostgreSQL schema for control plane
