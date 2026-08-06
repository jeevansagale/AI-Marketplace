# OmniRoute AI Marketplace — Implementation Roadmap

**Version:** 1.0.0
**Status:** Ready for Sprint 0
**Date:** August 6, 2026
**Depends On:** `Engineering_Design_Document.md` (architecture approved)

---

## 1. Guiding Principles

1. **Contract-first progression:** TEAL interface → protobufs → OpenAPI → Gateway stubs → SDKs → UI. This keeps 4 engineers integrating on stable contracts from Sprint 1.
2. **Thin vertical slices:** Every milestone delivers a *working end-to-end flow* (testnet), not "50% of every service."
3. **Two independent tracks:** The Gateway/Blockchain track (BE1) and the Marketplace/Control Plane track (BE2 + FE2) can progress in parallel; FE1 depends only on OpenAPI, which stabilizes early.
4. **The riskiest unknowns first (weeks 1–4):** (a) TEAL v8 box-storage escrow correctness, (b) SSE token-metering parser performance, (c) OpenAI wire conformance. Validate all three before building dashboards.
5. **Everything deployable to `make dev` in Sprint 1** — docker-compose + LocalNet + hot reload.

---

## 2. Milestones Overview

| Milestone | Theme | Sprints | Exit Criteria |
|---|---|---|---|
| **M0** | Foundations & Dev Environment | Sprint 0–1 | `make dev` boots; CI green; contracts drafted |
| **M1** | x402 Gateway + LocalNet Escrow | Sprint 2–3 | CLI client completes 402 → deposit → prove → non-streaming completion on LocalNet |
| **M2** | Streaming Billing + Settlement | Sprint 4–5 | SSE stream billed per-token; batched settlement to LocalNet; receipts journaled |
| **M3** | Control Plane APIs + OpenAI Conformance | Sprint 6–7 | Provider registration, model catalog, usage APIs; OpenAI SDK `base_url` drop-in works on staging (Testnet) |
| **M4** | Dashboards + Marketplaces (Web) | Sprint 8–10 | Consumer & Provider dashboards; marketplace search; wallet connect; analytics |
| **M5** | Production Hardening | Sprint 11–13 | Multi-AZ EKS, HSM keys, SLOs, load tests at 10k streams, blue/green, mainnet launch checklist |

Each sprint = 2 weeks. Total ≈ 26 weeks (6 months) to mainnet soft-launch.

---

## 3. Engineer Assignments (per project brief)

| Engineer | Track A (Backend 1) | Track B (Backend 2) | Track C (Frontend 1) | Track D (Frontend 2) |
|---|---|---|---|---|
| **BE1 — Gateway/Payments/Blockchain/Auth** | Gateway (Go), x402, macaroons, SSE metering, receipts | — | — | — |
| **BE2 — Marketplace/Provider APIs/Database/Settlement** | — | Control Plane (Go), Postgres schema, registry APIs, settlement worker, webhooks | — | — |
| **FE1 — Landing/Marketplace/Consumer Dashboard** | — | — | Next.js app shell, marketplace search, consumer dashboard | — |
| **FE2 — Provider Dashboard/Wallet/Analytics** | — | — | — | Provider dashboard, wallet connect, analytics charts, admin panel |

**Shared responsibilities:**
- Contracts (TEAL/protobuf/OpenAPI): BE1 + BE2 joint reviews, BE1 owns TEAL, BE2 owns OpenAPI surface, both own proto.
- SDKs (Python/TS x402 client): BE1 designs crypto flow; BE2 implements Python; FE2 implements TypeScript (or a shared DSP/contracts artifact + thin wrappers).
- Infra/CI: BE1 owns Gateway deployment + Cloudflare; BE2 owns DB/Redis/workers; both co-own EKS GitOps. FE owns web deploy (Vercel/Cloudflare Pages).

---

## 4. Sprint-By-Sprint Plan

### M0 — Foundations (Sprints 0–1)

| Sprint | BE1 | BE2 | FE1 | FE2 | Joint |
|---|---|---|---|---|---|
| **S0 (1 wk setup)** | Repo skeleton, `make dev` docker-compose, LocalNet sandbox, CI: lint+unit | Postgres schema v1 (`sqlc`), Redis compose, migrations runner | Next.js 15 scaffold (Tailwind, shadcn/ui, React Query, Zustand), Storybook | Wallet kit exploration (Pera/Defly + algokit), charts lib spike | Draft TEAL interface + OpenAPI skeleton; proto skeleton |
| **S1** | TEAL v8 `escrow_open/deposit/close` on LocalNet (+ MBR box logic); CLI test script | Control Plane module skeleton; auth (JWT + API key hashing); `/v1/health`, `/v1/models` stub | Landing page (marketing) with framing motion; design tokens | Provider onboarding form (mock); wallet connect modal stub | **Contract freeze v0.1** (proto + OpenAPI + TEAL ABI); conformance test harness scaffold |

**Exit:** `make dev` green. CLI can open an escrow on LocalNet. Web app shell renders.

---

### M1 — x402 Gateway + LocalNet Escrow (Sprints 2–3)

| Sprint | BE1 | BE2 | FE1 | FE2 | Joint |
|---|---|---|---|---|---|
| **S2** | Gateway boot (Fiber), middleware chain, 402 challenge minting, ed25519 proof verification, nonce SETNX | Payment module: channel model + `escrow` indexing on LocalNet; `PaymentService.HydrateChannel` gRPC | Marketplace listing page against `/v1/models` (mock data) | Wallet connect: sign message → JWT login; create channel UI (deposit amount) | **First E2E:** CLI 402 → fund → prove → non-streaming completion |
| **S3** | Routing cache (`rs:{model}:{region}`), provider selection stub, `httputil.ReverseProxy` non-streaming path | Registry API: provider registration, model CRUD, pricing; routing-table publisher (gRPC watch) | Marketplace filter/sort (price/latency/reputation) w/ real data | Consumer dashboard shell: wallet balance, channel list | Load test: 1k non-streaming requests through Gateway; conformance: non-streaming chats |

**Exit:** A developer using the raw HTTP API can complete a paid non-streaming `/v1/chat/completions` on LocalNet with provider routing.

---

### M2 — Streaming Billing + Settlement (Sprints 4–5)

| Sprint | BE1 | BE2 | FE1 | FE2 | Joint |
|---|---|---|---|---|---|
| **S4** | SSE state machine (custom `\n\n` parser), meter (usage-block + byte heuristic), budget lock, re-auth injection, client-disconnect RST | Settlement worker: drain `stream:receipts`, persist journal, DLQ; batch builder (atomic group: finalize + provider payout + fee) | Model detail page (pricing, latency, provider score) | Consumer dashboard: per-model spend chart; channel top-up/reauth flow | **E2E streaming:** 402 → prove → SSE stream billed per token → settlement batch on LocalNet |
| **S5** | Circuit breaker + EWMA latency routing; failover; provider health checks integration | L1 watcher (escrow_close race) → invalidate channel cache; reconciliation worker (Postgres vs L1 box) | Streaming UX: live token/cost ticker (demo) | Provider dashboard: model health, request logs (metadata only), payout history | Load test: 1k concurrent streams; conformance: streaming chats vs OpenAI golden |

**Exit:** Streaming is billed accurately with settlement + reconciliation working end-to-end on LocalNet. **This is the Beta gate.**

---

### M3 — Control Plane APIs + OpenAI Conformance (Sprints 6–7)

| Sprint | BE1 | BE2 | FE1 | FE2 | Joint |
|---|---|---|---|---|---|
| **S6** | x402 SDK Python + TS (402 loop, auto-fund, reauth) | Full REST surface: usage analytics, webhooks (HMAC-signed, retry), disputes API (24h hold), provider telemetry | Connect web app to real APIs (React Query server state) | Analytics: burn-rate, latency, reputation scorecards | Conformance suite expanded: embeddings, images, audio, error shapes |
| **S7** | Non-streaming pre-auth release; max-cost caveats; multi-model routing table publishing | Testnet (Algorand Testnet) deployment of control plane + settlement; staging EKS GitOps | Marketplace search (pg_trgm) with facets | Admin panel: providers, disputes queue, settlement batches | **OpenAI SDK drop-in verified:** `openai` Python library pointed at `base_url=https://staging.omniroute.ai/v1` completes a paid stream with our SDK hook |

**Exit:** Full API surface live on staging (Testnet). OpenAI SDK compatible.

---

### M4 — Dashboards + Marketplace (Sprints 8–10)

| Sprint | BE1 | BE2 | FE1 | FE2 | Joint |
|---|---|---|---|---|---|
| **S8** | HSM/Vault integration for settlement key; KMS key rotation runbook | Usage materialized views; dashboard APIs v1; pagination/perf passes | Consumer dashboard final: channels, spend, disputes, API keys, webhooks | Provider dashboard final: payouts, health, reputation, model pricing | Design review pass on all dashboards |
| **S9** | Gateway multi-replica + ALB; Blue/Green deploy; canary | Redis Cluster (sharded by `{channel_id}`); PgBouncer; alerting (Prometheus rules) | Marketplace ranking formula tuning (price×uptime×latency×reputation) | Wallet flows: withdraw, close channel (timelock), dust policy UI | E2E disputes flow demo |
| **S10** | Load test 5k streams; GC tuning; pprof pass | Reconciliation soak; settlement stress (10k receipts) | A11y + i18n pass; performance budget (Lighthouse) | Analytics realtime (SSE) on dashboards | Security audit review (STRIDE checklist sign-off) |

**Exit:** Feature-complete Beta on staging. Load/SLO benchmarks documented.

---

### M5 — Production Hardening (Sprints 11–13)

| Sprint | BE1 | BE2 | FE1 | FE2 | Joint |
|---|---|---|---|---|---|
| **S11** | Mainnet smoke; escrow contract audit (external) findings triage | Staging → prod migration (RDS Multi-AZ, Redis multi-AZ, EKS taints) | Final copy/docs (developer docs, SDK quickstart) | Payouts: real USDC (Mainnet) rollout for first providers | Audit remediation; GoSec/semgrep zero-blocker pass |
| **S12** | Synthetic monitoring (blackbox buy-every-60s); on-call runbooks | SLO dashboards; PagerDuty integration; runbook drills | — | — | **Soft launch (invite-only, Testnet→Mainnet bridge)** |
| **S13** | Post-launch fixes; scaling to 10k streams; feature-flag cleanup | Reconciliation 24h soak; quarterly budget review | Support/docs polish | Marketing site final | Retro → Phase 4 protocol roadmap (open-source Gateway) |

---

## 5. GitHub Issues (generated — create in repo)

Labels: `P0` (blocker), `P1` (high), `P2` (normal), `P3` (low), `track:gateway`, `track:controlplane`, `track:frontend`, `track:contracts`, `track:sdk`, `track:infra`, `~sprintNN`, `epic:M0..M5`.

### Epic: M0 Foundations
- `#1` [P0][track:infra] Monorepo skeleton: apps/{gateway,controlplane,web}, packages/*, Makefile.dev  (~sprint0)
- `#2` [P0][track:infra] Docker Compose: gateway, controlplane, redis, postgres, LocalNet sandbox (`make dev`)  (~sprint0)
- `#3` [P0][track:infra] CI: lint + unit + build + trivy on PRs (GitHub Actions)  (~sprint0)
- `#4` [P0][track:contracts] TEAL v8 scaffold (AlgoKit): escrow app skeleton + deploy script + LocalNet smoke  (~sprint1)
- `#5` [P0][track:contracts] Proto skeleton: RegistryService + PaymentService + AuthService  (~sprint1)
- `#6` [P0][track:contracts] OpenAPI skeleton v0.1: `/v1/models`, `/v1/chat/completions`, error catalog  (~sprint1)
- `#7` [P1][track:frontend] Next.js 15 scaffold: Tailwind, shadcn/ui, React Query, Zustand, Storybook  (~sprint0)
- `#8` [P1][track:frontend] Landing page: hero, features, architecture section (Framer Motion)  (~sprint1)
- `#9` [P2][track:frontend] Wallet kit spike: Pera/Defly + WalletConnect + signature login  (~sprint0)

### Epic: M1 x402 Gateway
- `#10` [P0][track:gateway] Gateway boot: Fiber server, middleware chain (trace→ratelimit→idempotency→x402)  (~sprint2)
- `#11` [P0][track:gateway] 402 challenge minting (nonce, escrow, C_max, expiry) + `X-402-Payment-Required`  (~sprint2)
- `#12` [P0][track:gateway] Ed25519 proof verification + body-hash binding + caveat checks  (~sprint2)
- `#13` [P0][track:gateway] Nonce SETNX wrapper (Redis) + fail-closed behavior  (~sprint2)
- `#14` [P1][track:gateway] Routing cache + provider selection (weighted score stub)  (~sprint2)
- `#15` [P0][track:gateway] Reverse proxy (non-streaming) with mTLS + per-provider secret header  (~sprint2)
- `#16` [P0][track:contracts] TEAL: `escrow_open` (atomic deposit + box MBR), `escrow_close` (timelock), refund logic  (~sprint2)
- `#17` [P0][track:gateway] E2E CLI: 402 → fund → prove → non-streaming completion  (~sprint3)
- `#18` [P1][track:controlplane] Payment module: channel model, escrow indexing, `HydrateChannel` gRPC  (~sprint2)
- `#19` [P1][track:controlplane] Registry API: provider register, model CRUD, pricing  (~sprint3)
- `#20` [P1][track:controlplane] Routing-table publisher (gRPC watch to Gateway)  (~sprint3)
- `#21` [P2][track:gateway] ID token/health endpoint `/v1/models` passthrough  (~sprint3)

### Epic: M2 Streaming + Settlement
- `#22` [P0][track:gateway] SSE state machine: custom `\n\n` parser, event size bound  (~sprint4)
- `#23` [P0][track:gateway] Meter: usage-block extraction + byte-heuristic fallback  (~sprint4)
- `#24` [P0][track:gateway] Budget lock per request + re-auth SSE event + truncation  (~sprint4)
- `#25` [P0][track:gateway] Client-disconnect → provider RST; partial receipt emit  (~sprint4)
- `#26` [P0][track:gateway] RequestReceipt builder + Gateway signature  (~sprint4)
- `#27` [P0][track:controlplane] Settlement worker: drain `stream:receipts` → journal → DLQ  (~sprint4)
- `#28` [P0][track:controlplane] Batch builder: aggregate per channel → atomic group (finalize+payout+fee)  (~sprint4)
- `#29` [P1][track:controlplane] L1 watcher: escrow_close race → invalidate channel cache  (~sprint5)
- `#30` [P1][track:controlplane] Reconciliation worker: Postgres vs L1 box diff + P1 alert  (~sprint5)
- `#31` [P1][track:gateway] Circuit breaker + EWMA latency routing + provider failover  (~sprint5)
- `#32` [P1][track:controlplane] Provider health worker: ping every 30 s + synthetic completion  (~sprint5)
- `#33` [P1][track:gateway] Load test: 1k concurrent streams; pprof/GC tuning  (~sprint5)

### Epic: M3 APIs + Conformance
- `#34` [P0][track:sdk] Python x402 SDK: 402 loop, auto-fund, reauth, metadata  (~sprint6)
- `#35` [P0][track:sdk] TypeScript x402 SDK  (~sprint6)
- `#36` [P1][track:controlplane] Usage analytics APIs (+ materialized views)  (~sprint6)
- `#37` [P1][track:controlplane] Webhooks: HMAC-SHA256 signature, retry/backoff, queue  (~sprint6)
- `#38` [P1][track:controlplane] Disputes API: 24h hold, freeze payout, resolution flow  (~sprint6)
- `#39` [P1][track:controlplane] Provider telemetry endpoint  (~sprint6)
- `#40` [P0][track:gateway] Non-streaming pre-auth release + max-cost caveat  (~sprint7)
- `#41` [P0][track:infra] Staging EKS: control plane + Redis + Postgres on Testnet  (~sprint7)
- `#42` [P0][track:contracts] Conformance suite: chat non-stream/stream, embeddings, images, audio, error shapes vs OpenAI golden  (~sprint7)
- `#43` [P0][track:sdk] OpenAI Python SDK drop-in verification (`base_url` swap)  (~sprint7)

### Epic: M4 Dashboards
- `#44` [P1][track:controlplane] Dashboard APIs v1: channels, spend, disputes, api-keys, webhooks  (~sprint8)
- `#45` [P1][track:frontend] Consumer dashboard: channels, spend charts, API keys, webhooks, disputes  (~sprint8)
- `#46` [P1][track:frontend] Provider dashboard: models, health, payouts, reputation  (~sprint8)
- `#47` [P2][track:frontend] Admin panel: providers, disputes queue, settlement batches  (~sprint7)
- `#48` [P1][track:frontend] Marketplace search + facets (pg_trgm) + ranking tuning  (~sprint9)
- `#49` [P1][track:frontend] Wallet flows: deposit, withdraw, close (timelock), dust policy  (~sprint9)
- `#50` [P1][track:frontend] Realtime: burn-rate + settlement status via SSE  (~sprint10)
- `#51` [P2][track:infra] HSM/Vault: settlement operator key + macaroon root rotation  (~sprint8)

### Epic: M5 Production
- `#52` [P0][track:infra] Redis Cluster sharding `{channel_id}` + multi-AZ  (~sprint9)
- `#53` [P0][track:infra] RDS Multi-AZ + PgBouncer  (~sprint9)
- `#54` [P0][track:gateway] Gateway multi-replica + ALB + Blue/Green + canary  (~sprint9)
- `#55` [P0][track:infra] Prometheus rules + Grafana dashboards + PagerDuty  (~sprint9)
- `#56` [P0][track:contracts] Smart contract external audit (scope: escrow, settle, dispute)  (~sprint11)
- `#57` [P0][track:infra] Mainnet smoke + rollback runbook  (~sprint11)
- `#58` [P0][track:infra] Synthetic blackbox monitor (buy-every-60s)  (~sprint12)
- `#59` [P0][track:infra] SLO dashboards + runbook drills  (~sprint12)
- `#60` [P1][track:gateway] Load test 10k streams; post-launch fixes  (~sprint13)

---

## 6. TODO Lists (per engineer)

### BE1 — Gateway/Payments/Auth/Blockchain
- [ ] Sprint 0: repo skeleton, `make dev`, Go module init, LocalNet sandbox
- [ ] S1: TEAL escrow open/close/deposit (+MBR), CLI smoke
- [ ] S2: Gateway boot, 402 challenge, proof verification, nonce SETNX
- [ ] S2: Reverse proxy non-streaming (mTLS), routing cache
- [ ] S3: Provider selection stub + routing table watch client
- [ ] S4: SSE state machine + meter + budget lock + re-auth
- [ ] S4: Receipt builder + Gateway signature
- [ ] S5: Circuit breaker + EWMA + failover
- [ ] S5: Client-disconnect RST + partial billing
- [ ] S6: Python/TS SDK crypto core (402 loop)
- [ ] S7: Non-streaming pre-auth release
- [ ] S8: HSM/Vault settlement key integration
- [ ] S9: Gateway multi-replica ALB, Blue/Green
- [ ] S11: Mainnet smoke + audit triage
- [ ] S12: Synthetic blackbox monitor
- [ ] S13: 10k stream load tuning

### BE2 — Marketplace/Provider APIs/DB/Settlement
- [ ] Sprint 0: Postgres schema v1 (sqlc), Redis compose, migration runner
- [ ] S1: Control Plane skeleton, auth (JWT + API key hashing), `/v1/models` stub
- [ ] S2: Payment module (channel model, escrow indexing, HydrateChannel gRPC)
- [ ] S3: Registry API (provider/model/pricing)
- [ ] S4: Settlement worker (drain, journal, DLQ) + batch builder (atomic group)
- [ ] S5: L1 watcher (close race) + reconciliation worker
- [ ] S5: Provider health worker
- [ ] S6: Usage analytics APIs + materialized views; webhooks (HMAC, retry)
- [ ] S6: Disputes API (24h hold)
- [ ] S7: Staging EKS GitOps (control plane + data services + Testnet)
- [ ] S8: Dashboard APIs v1 + pagination/perf
- [ ] S9: Redis Cluster sharding, PgBouncer, Prometheus rules
- [ ] S11: RDS Multi-AZ prod migration
- [ ] S12: SLO dashboards + PagerDuty + runbook drills
- [ ] S13: Reconciliation soak + budget review

### FE1 — Landing/Marketplace/Consumer Dashboard
- [ ] Sprint 0: Next.js scaffold (Tailwind, shadcn/ui, RQ, Zustand, Storybook)
- [ ] S1: Landing page (Framer Motion)
- [ ] S2: Marketplace listing (mock → real `/v1/models`)
- [ ] S3: Marketplace filter/sort (price/latency/reputation)
- [ ] S4: Model detail page (pricing, latency, provider score)
- [ ] S5: Streaming live token/cost ticker demo
- [ ] S6: Real API wiring (React Query server state)
- [ ] S7: Marketplace search (pg_trgm facets)
- [ ] S8: Consumer dashboard final
- [ ] S9: Ranking formula tuning
- [ ] S10: A11y + i18n + Lighthouse budget
- [ ] S11: Final developer docs/copy

### FE2 — Provider Dashboard/Wallet/Analytics
- [ ] Sprint 0: Wallet kit spike (Pera/Defly/WalletConnect), charts lib
- [ ] S1: Provider onboarding form (mock) + wallet connect modal
- [ ] S2: Wallet connect → JWT login; create-channel UI
- [ ] S3: Consumer dashboard shell (balance, channel list)
- [ ] S4: Per-model spend chart; channel top-up/reauth UI
- [ ] S5: Provider dashboard: health, request logs (metadata), payouts
- [ ] S6: Analytics scorecards (burn-rate, latency, reputation)
- [ ] S7: Admin panel (providers, disputes, settlement batches)
- [ ] S8: Provider dashboard final
- [ ] S9: Wallet flows: withdraw, close (timelock), dust policy UI
- [ ] S10: Realtime SSE on dashboards
- [ ] S12: Payouts UI (mainnet USDC) for first providers

---

## 7. Implementation Order (Critical Path)

```
Phase A (Weeks 1–2) — Contracts & Env
  TEAL escrow ABI ─▶ Proto v0.1 ─▶ OpenAPI v0.1
       │
  Docker Compose + LocalNet ─▶ make dev ─▶ CI green

Phase B (Weeks 3–6) — Gateway Core (BE1) ∥ Control Plane Core (BE2)
  Non-streaming 402 proof flow (CLI)      Registry + channel modules
       │                                        │
  └──────────────▶ M1 Exit: paid non-streaming completion

Phase C (Weeks 7–10) — Streaming + Settlement
  SSE parser+meter (BE1)
  Settlement worker + batch + reconcile (BE2)
       │
  └──────────────▶ M2 Exit: billed SSE stream + settlement on LocalNet

Phase D (Weeks 11–14) — APIs + SDKs + Staging
  SDKs (BE1/BE2/FE2) + full REST surface (BE2)
  Conformance suite (joint) + OpenAI drop-in
       │
  └──────────────▶ M3 Exit: OpenAI SDK works on staging/Testnet

Phase E (Weeks 15–20) — Dashboards
  Consumer (FE1) ∥ Provider (FE2) ∥ Admin (FE2)
  Marketplace search (FE1) + wallet flows (FE2)
       │
  └──────────────▶ M4 Exit: feature-complete Beta

Phase F (Weeks 21–26) — Production
  Infra (BE1/BE2): Redis/RDS/EKS multi-AZ, HSM, alerts
  Audit + Mainnet smoke + SLO dashboards
       │
  └──────────────▶ M5 Exit: soft launch
```

**Strict dependencies:**
- `#22 SSE parser` cannot start before `#15 reverse proxy` structural lines are in place (BE1 owns both).
- `#28 batch builder` cannot start before `#16 TEAL settle/finalize` ABI exists (BE2 can mock the ABI until then).
- `#40 conformance suite` requires `#34/#35 SDKs` and `#27 journal` (joint).
- FE tracks only depend on OpenAPI (`#6`) and dashboard APIs (`#44`) — both promise to be stable by Sprint 6–7.

---

## 8. Definition of Done (shared)

- Code merged passes: `golangci-lint`, `go test ./...`, `vitest`, `trivy`, `semgrep/gosec`.
- Pr changed schema → migration file + rollback tested; no destructive DDL in prod.
- Financial behaviors covered by ≥ 1 integration test (LocalNet).
- Metrics emitted for the component (per §29 of EDD).
- No prompt/response content in logs (lint rule enforced by CI grep).
- ADR updated when a decision deviates from `Engineering_Design_Document.md`.

---

## 9. Risk Watchlist

| Risk | Owner | Mitigation | Trigger |
|---|---|---|---|
| TEAL box storage bugs | BE1 | LocalNet test matrix + external audit (Sprint 11) | Any box read/write mismatch |
| SSE parser throughput < 10k streams | BE1 | pprof + custom state machine; fallback gnet | P99 parser > 1 ms at 5k streams |
| OpenAI conformance drift | Joint | Golden test suite in CI; oasdiff | Any schema diff vs OpenAI |
| Redis outage in prod | BE1/BE2 | Fail-closed + rehydration from Postgres; drills | SETNX availability < 99.99% |
| Settlement key compromise | BE2 | HSM/KMS, rotation, contract freeze | Any unauthorized tx |
| FE waiting on BE | FE1/FE2 | OpenAPI-first; mock servers from Sprint 2 | Spec churn > 10% week-over-week |
| Provider liquidity (cold start) | Product | Seed 2–3 providers during Beta; subsidize first payouts | < 3 active providers by Sprint 12 |

---

## 10. How to Start (First 5 Pull Requests)

1. `chore: monorepo skeleton + make dev` (#1, #2)
2. `feat(contracts): TEAL escrow skeleton + LocalNet deploy` (#4)
3. `feat(gateway): Fiber boot + health + middleware chain` (#10)
4. `feat(controlplane): auth + /v1/models stub + schema v1` (#5, #6, #7)
5. `feat(web): Next.js scaffold + landing page` (#7, #8)

After these five merge, the three tracks are decoupled and parallel work begins per §4.