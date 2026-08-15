# Roadmap

This document outlines planned improvements and future capabilities.

Items marked **[MVP]** are critical for production readiness.
Items marked **[Post-MVP]** are valuable but not blocking.

---

## Infrastructure & Production Readiness

- **[MVP]** Replace in-memory wallet with PostgreSQL-backed implementation
- **[MVP]** Distributed session state via Redis
- **[MVP]** Authentication (API keys or JWT for multi-tenant usage)
- **[MVP]** Rate limiting per wallet / per API key
- **[MVP]** Structured logging (JSON, with request IDs)
- **[MVP]** Metrics export (Prometheus / OpenTelemetry)
- **[Post-MVP]** Kubernetes deployment manifests (Helm chart)
- **[Post-MVP]** CI/CD pipeline (GitHub Actions: test → build → deploy)

---

## Payment System

- **[MVP]** Razorpay integration for real INR payments
- **[MVP]** Webhook verification for payment events
- **[Post-MVP]** Stripe integration for international payments
- **[Post-MVP]** Prepaid subscription tiers
- **[Post-MVP]** Real HTTP 402 on-chain settlement (USDC/stablecoin)

---

## Provider Ecosystem

- **[MVP]** Anthropic Claude integration
- **[MVP]** Meta Llama (via Groq or Together AI)
- **[Post-MVP]** Mistral AI
- **[Post-MVP]** Stable Diffusion (image generation)
- **[Post-MVP]** Provider SLA monitoring and automatic circuit breakers
- **[Post-MVP]** Cost-optimized routing (select cheapest provider for model category)

---

## API

- **[MVP]** API key authentication middleware
- **[MVP]** Per-session spending limits
- **[Post-MVP]** OpenAI-compatible `/v1/` surface (full compatibility)
- **[Post-MVP]** Streaming image generation (progressive JPEG)
- **[Post-MVP]** Function calling / tool use pass-through
- **[Post-MVP]** Embeddings endpoint

---

## Marketplace

- **[Post-MVP]** Provider-submitted model listings
- **[Post-MVP]** Model benchmarking page (real latency/throughput data)
- **[Post-MVP]** User reviews and ratings
- **[Post-MVP]** Model comparison tool

---

## Developer Experience

- **[Post-MVP]** TypeScript SDK (`@ai-marketplace/client`)
- **[Post-MVP]** Python SDK
- **[Post-MVP]** CLI tool (`ai-marketplace chat --model gemini-2.0-flash`)
- **[Post-MVP]** OpenAPI / Swagger spec generation
- **[Post-MVP]** Postman collection

---

## Observability

- **[Post-MVP]** Request tracing (OpenTelemetry → Jaeger/Tempo)
- **[Post-MVP]** Cost dashboard with provider breakdown
- **[Post-MVP]** Anomaly detection on spending spikes
- **[Post-MVP]** Provider health status page

---

## Multi-tenancy

- **[Post-MVP]** Organization and project support
- **[Post-MVP]** Per-project wallet isolation
- **[Post-MVP]** Team member access control
- **[Post-MVP]** Audit log per organization
