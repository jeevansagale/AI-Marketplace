# OmniRoute AI Marketplace — Hackathon MVP Plan (Demo: Aug 10)

**Role:** Lead Software Architect / Principal Engineer
**Team:** BE1 (Gateway, x402, Streaming, Algorand) · BE2 (Marketplace APIs, Dashboards APIs, DB, Auth) · FE1 (Landing, Marketplace, Playground, Animations) · FE2 (Dashboards, Wallet, Charts, Admin)
**Window:** Aug 6 (start) → Aug 10 (demo). 4 days.
**North star:** *"An AI agent autonomously discovers, pays for, and consumes an AI API using HTTP 402 + Algorand"* — demonstrated live, reliably, on a laptop.

---

## 0. MVP Strategy & Guardrails

### 0.1 Priorities (in order)
1. **The 402 moment must work live, every time.** Signature verification is pure Ed25519 — it can run offline with a bundled Demo Wallet. The demo will never depend on Testnet connectivity for the core loop.
2. **OpenAI-compatible chat + real streaming.** A working playground that streams tokens is 50% of the win.
3. **Dashboards that look alive.** Seeded data, live-updating after each demo request.
4. **Premium dark-first UI** — the visual polish judges remember.

### 0.2 What must be REAL vs MOCKED (interfaces preserved)

| Component | Status | Real implementation | Mock/fallback (same interface) |
|---|---|---|---|
| HTTP 402 handshake | **REAL** | Gateway returns `402` + challenge; client resubmits with proof; nonce replay blocked | — |
| Signature verification | **REAL** | Ed25519 verify (Algorand key scheme) performed in-process — no node required | — |
| Nonce/replay protection | **REAL** | Postgres `receipts(nonce UNIQUE)` + in-memory SETNX-style cache; fail-closed | — |
| AI streaming | **REAL** | Gateway SSE proxy with token pacing | Embedded `MockProvider` serving scripted realistic completions (also OpenAI-compatible) |
| Wallet | **REAL (optional)** | Pera/Defly via WalletConnect on Testnet (funded demo account) | **Demo Wallet**: bundled local keypair, prefunded 100 USDC channel, auto-approve |
| Escrow/channel state | **REAL (modeled)** | Postgres `channels` with locked/settled/latest_nonce (exact production schema shape) | On-chain Testnet escrow txn only when a real wallet is connected |
| Settlement batching | MOCKED (UI) | Queue UI + "Settlement Queued" state; batch runner stub | Admin dashboard shows batch flow; receipt timestamps advance |
| Provider routing | MOCKED | Weighted-selection function exists | Registry seed data; selection by price/latency from DB |
| Marketplace, Dashboards, Auth | **REAL** | Go APIs + Postgres + JWT demo auth | — |
| NATS / Redis / Kafka / K8s | MOCKED | — | Not deployed. Logged as `demo-mode`. No code smells: interfaces stay |
| Landing/Architecture pages | **REAL (static)** | Next.js | — |

### 0.3 Offline-first rule
The demo runs on a laptop with **zero external dependencies**: local Postgres (Docker) or SQLite-backed demo seed, bundled fonts, Demo Wallet, mock provider. Deployed URLs (Vercel + Railway) are a secondary lane; a **pre-recorded video backup** is the last lane.

---

## 1. MVP Architecture (Slim)

```
Browser (Next.js 15)  ──HTTP──▶  Gateway :8080 (Go/Fiber)
                         │          │  /v1/chat/completions (OpenAI wire-compat)
                         │          │  /v1/embeddings (stub)
                         │          ▼
                         │      x402 Engine: challenge → proof verify (Ed25519) → nonce check
                         │          ▼
                         │      Router → Provider (real or MockProvider SSE)
                         ▼
Browser ◀──SSE stream── Gateway ◀──stream── Provider
Browser ──HTTP/JSON──▶ Control Plane :8081 (Go/Fiber)
                          /v1/models · /v1/consumer/* · /v1/provider/* · /v1/admin/* · /v1/auth/*
                          ▼
                       PostgreSQL (providers, models, wallets, channels, receipts, api_keys, settlements)
```

- **Two Go services, one module** (`services/gateway`, `services/controlplane`, shared `pkg/`). Same boundaries as production → later swap-in is trivial.
- **Postgres required** (Docker Compose). Seed script idempotent (`psql`/migrate runner) — data never appears empty.
- **No Redis, NATS, Kafka, K8s in the demo.** Documented as "demo-mode"; acceptance: code compiles against production-shaped interfaces (`store`, `meter`, `nonce` packages).
- **Mock provider is an embedded handler** in the Gateway serving `/provider/{id}/v1/chat/completions` — identical contract to a real provider so the router code path is real.

### 1.1 Frozen MVP API contract (agreed Day 0; no breaking changes after)
| Method | Path | Purpose |
|---|---|---|
| POST | `/v1/chat/completions` | OpenAI-compatible; requires `X-402-Payment-Proof` or returns `402` |
| GET | `/v1/models` | OpenAI-style model list (public) |
| GET | `/v1/marketplace/models?q=&category=&sort=` | Marketplace search (BE2) |
| POST | `/v1/auth/wallet/connect` | Demo JWT exchange (wallet addy → JWT) |
| GET | `/v1/consumer/overview` · `/usage` · `/transactions` · `/channels` | Consumer dashboard |
| GET | `/v1/provider/overview` · `/models` · `/revenue` · `/requests` · `/payouts` | Provider dashboard |
| GET | `/v1/admin/providers` · `/models` · `/settlements` · `/health` · `/metrics` · `/transactions` | Admin dashboard |
| POST | `/v1/marketplace/state-channels/reauth` | Mid-stream top-up proof (SDK parity) |

Errors: RFC 9457 `application/problem+json` (`type`, `title`, `status`, `detail`, `instance`). 402 body carries challenge JSON; headers both standard (`WWW-Authenticate: x402 …`) and aliases (`X-402-Payment-Required`, `X-402-Payment-Proof`) — keeps production interop path.

### 1.2 Slim DB schema (production-shaped subset)
Providing in seed/migration order:
`users` → `providers` → `models` → `wallets` → `channels` → `receipts` → `settlements` → `api_keys`.
Key columns mirror the production spec (`channels.locked_micro_usdc`, `channels.settled_micro_usdc`, `channels.latest_nonce`, `receipts.nonce UNIQUE`, `receipts.cumulative_spend_micro_usdc`, `settlements.status`). Indexes: `models(category, is_active)`, `receipts(channel_id, created_at)`, `receipts(nonce UNIQUE)`.

### 1.3 Frontend structure (`apps/web`)
```
app/
  layout.tsx            (dark-first, fonts, Theme/Query/Toaster providers)
  page.tsx              Landing (hero, problem, solution, features, architecture, 402 explainer, CTA)
  marketplace/page.tsx  search + filter + cards + "Try Model"
  playground/page.tsx   OpenAI-style chat + pipeline stepper + PaymentModal
  consumer/page.tsx     balance, spend, requests, transactions, charts, channel state
  provider/page.tsx     revenue, models, latency, health, payouts, requests
  admin/page.tsx        providers, models, settlement queue, gateway metrics, tx history
  architecture/page.tsx animated flow
components/  (ui/ shadcn; marketing/; playground/; dashboard/; wallet/ PaymentWalletModal; charts/; shared/)
lib/         api client (fetch + demo JWT), React Query hooks, Zustand stores (ui, wallet, payment), utils (formatUSDC, timeAgo)
```

---

## 2. Demo Narrative (3-minute judge script)

1. **Landing (15s):** "AI agents can't pay other AIs. They can't open bank accounts or get credit cards."
2. **Marketplace (20s):** "Here's a marketplace of AI models — chat, vision, OCR — each with transparent USDC pricing. Completely OpenAI-compatible — devs only change the `base_url`."
3. **Playground — THE MOMENT (75s):** Type prompt → **Generate with NO API key**. Gateway answers **HTTP 402 Payment Required**. Payment modal appears: model, estimated cost, wallet, escrow channel. **Approve**. Animated pipeline: *402 → Wallet Signature → Payment Verified → Provider Routed → Streaming*. Tokens stream live. Then: *Settlement Queued → Completed*. "That stream just cost 0.0012 USDC. No subscription, no credit card — a machine paid a machine."
4. **Dashboards (40s):** Consumer dashboard — balance debited, transaction appeared, usage chart ticked. Provider dashboard — revenue +0.0012, latency live. Admin — settlement queue shows the receipt batch.
5. **Architecture page (30s):** Animated diagram explains the loop; mention "state channels + batched on-chain settlement on Algorand — the production design is in `docs/`".

---

## 3. Milestones

### M0 — Day 0 (Aug 6, half-day): Scaffold, Contracts, Data
**Objectives:** Everything compiles; API & UI contracts frozen; DB seed visible; four engineers unblocked in parallel.
**Tasks**
- [BE1] Init monorepo (`services/gateway`, `pkg/x402`, `pkg/store`), Go workspace, Docker Compose (postgres + gateway + controlplane). JWT-ed demo auth stub. (3h)
- [BE2] Init `services/controlplane`; migrations 0001–0008; seed script (10 models, 6 providers, demo wallet 100 USDC, 250 seeded transactions/receipts, 3 channels, latency series); `/v1/models` + marketplace search. (4h)
- [FE1] Init `apps/web` (Next 15, TS, Tailwind, shadcn/ui, Framer Motion, React Query, Zustand, dark theme tokens, fonts); page shells for `/`, `/marketplace`, `/playground` with empty states. (4h)
- [FE2] `apps/web` dashboard shells (`/consumer`, `/provider`, `/admin`, `/architecture`); shared component library (StatCard, DataTable, Chart wrappers, PageHeader); PaymentModal + WalletModal visual prototype. (4h)
**Deliverables:** monorepo on GitHub, `make dev` = compose + migrate + seed + both servers + web. `GET /v1/models` returns seeded data. All pages render dark-premium shells.
**Dependencies:** none (all start from a README + Makefile created by BE1 first 30 min).
**Acceptance:** `make dev` on a clean clone in < 5 min; seed data visible via curl and in `<web>/api` stubs; contracts doc committed and frozen (changes require lead sign-off).
**Time:** 4–5h incl. setup.

### M1 — Day 1 (Aug 7): Core Loop (402 → Verify → Stream)
**Objectives:** The centerpiece works end-to-end with Demo Wallet and MockProvider — no real wallet, no Testnet needed.
**Tasks**
- [BE1] x402 engine: challenge mint (canonical hash of method/path/body/nonce/model/C_max), proof parse, **Ed25519 verify** (algosdk-compatible), nonce replay (in-memory + DB unique), channel budget lock (`C_max` at start, decrement per streamed byte→token), receipt on completion, `402`/`403`/`409` problem+json responses. (6h)
- [BE1] Gateway chat endpoint: OpenAI request/response schema, SSE proxy + token pacing, MockProvider embedded (scripted completions per model, 40–80 tok/s), `stream:false` JSON path, mid-stream budget truncation → `x402_reauth_required` event. (6h)
- [BE2] Store layer for channels/receipts/settlements (production-shaped); `/v1/marketplace/state-channels/reauth`; end-to-end smoke script (`curl` 402 → craft demo proof → 200 stream). (4h)
- [FE1] Playground chat UI: message list, model selector, temperature/max-token controls, stream toggle, generate button that POSTs and reads the SSE response stream (fetch ReadableStream). (5h)
- [FE2] **PaymentModal**: triggered on 402, shows estimated cost/model/wallet/escrow, Approve button → Demo Wallet "signs" (real Ed25519 sig via WebCrypto or bundled keypair) → resubmit; animated pipeline stepper component (6 stages) reusable on playground + architecture pages. (5h)
**Deliverables:** `curl`-able 402 flow; playground streams tokens; payment modal completes the loop with Demo Wallet.
**Dependencies:** FE2 PaymentModal consumes BE1 402 contract; FE1 playground depends on BE1 streaming endpoint (they can use the shipped MockProvider from BE1 by midday).
**Acceptance:** automated smoke `make test-core` (402 → proof → 200 stream → receipt row) green; replaying same nonce returns `409`; budget truncation fires `x402_reauth_required`; playground completes a full loop in < 5s TTFT.
**Time:** 8–9h.

### M2 — Day 2 (Aug 8): Integration, Dashboards, Data Lifecycle
**Objectives:** Real wallet option (Pera on Testnet, best-effort), all four dashboards live and wired to real APIs, transactions update live after each playground request.
**Tasks**
- [BE1] Real Algorand lane: algosdk sign+verify for real Testnet account; Pera/Defly WalletConnect handshake endpoint; channel open/fund via Testnet USDC (best-effort; graceful degrade to Demo Wallet). (5h)
- [BE2] Consumer/Provider/Admin dashboard APIs (overview aggregates, usage series, revenue, health snapshots, settlement queue, gateway metrics from in-memory counters); API-key table; seed refresh script. (6h)
- [FE2] Wire dashboards to React Query: Consumer (balance, daily spend, requests, transactions, model usage, channel balance + charts); Provider (revenue, models, avg latency, health, payout queue, recent requests); Admin (providers, models, settlement queue with live "queued→completed" transitions, system health, gateway metrics gauge cards, tx history). (6h)
- [FE1] Marketplace: search/filter/category/sort + "Try Model" → playground prefill; model cards with price/latency/context/availability; landing page sections complete (problem, solution, features, 402 explainer, supported providers, CTA). (5h)
**Deliverables:** all pages fully functional; after every playground completion, consumer spend/receipts/provider revenue/admin queue tick up automatically (poll via React Query 5s invalidation).
**Dependencies:** FE2 APIs from BE2; FE1 uses `/v1/marketplace/models` from BE2.
**Acceptance:** manual QA script (`docs/mvp/QA.md`): 10-step walkthrough covering every page with live data mutations visible; no page shows empty/unstyled state.
**Time:** 8–9h.

### M3 — Day 3 (Aug 9): Polish, Architecture Viz, Deploy, Rehearsal
**Objectives:** Presentation-ready. Animations, rich demo data, deployed lanes, rehearsal with the judge script.
**Tasks**
- [FE1] Framer Motion polish (hero micro-interactions, scroll reveals, card hovers, payment modal choreography, pipeline stepper animation); responsiveness pass; architecture page interactive animated diagram (Consumer → 402 → Wallet → Escrow → Provider → Stream → Settlement). (6h)
- [FE2] Chart polish (sparklines, tooltips, dark theming); number ticker animations; admin "settlement batch" visual with progress; demo wallet balance card; loading/error skeletons everywhere. (4h)
- [BE1] `make demo` single command (compose + migrate + seed + run); graceful failure modes (provider 500 → retry/refund flow visible); Docker images; CORS config for deployed web. (3h)
- [BE2] Deploy lane: web → Vercel; controlplane+gateway → Railway/Fly (optional); seeded remote DB; smoke both URLs. Write `docs/mvp/RUNBOOK.md` (demo day script, rollback = local mode, standby video). (4h)
- [Lead] 3 full rehearsals of the 3-minute script; fix pacing; **record OBS backup video** of the whole demo (local mode). (2h)
**Deliverables:** polished UI; deployed URLs (if reachable); `RUNBOOK.md`; `demo.mp4` backup; freeze feature list (everything else = backlog).
**Dependencies:** none beyond M2.
**Acceptance:** 3 consecutive full-loop demos in local mode with zero human intervention; one deployed-URL demo; video backup renders; no broken/missing UI on 1440p + 1280px widths.
**Time:** 8h.

### M4 — Day 4 (Aug 10, demo day): Ship & Standby
**Objectives:** Zero-risk delivery.
- Morning: re-run `make demo` on the demo machine; verify seed; confirm Demo Wallet works offline; test 1280px projector mode; standby laptop booted with parallel checkout + `make demo` ready.
- Pre-arrival: copy `demo.mp4` to desktop + phone; print QR to deployed URLs; ensure wifi fallback (phone hotspot) tested once.
- At demo: follow `RUNBOOK.md`; if anything fails → pre-recorded video on the big screen with live narration (seamless).
**Acceptance:** demo delivered; repo tagged `hackathon-aug10`.

---

## 4. Engineer Parallelization Matrix

| Track | M0 | M1 | M2 | M3 | Blocked-by |
|---|---|---|---|---|---|
| **BE1** | repo init, compose, gateway scaffold | x402 engine, streaming, MockProvider | real wallet lane, CORS, `make demo` | failure-mode polish | M0 scaffold (self-created) |
| **BE2** | controlplane, migrations, seed, models API | store layer, reauth, smoke script | dashboard APIs, metrics counters | deploy lane, RUNBOOK | M0 scaffold |
| **FE1** | web scaffold, page shells | playground chat + SSE reader | marketplace full, landing complete | motion polish, architecture viz | BE1 streaming endpoint (midday D1) |
| **FE2** | UI kit, modal prototypes | PaymentModal + stepper | dashboards wired | chart polish, skeleton pass | BE2 APIs (D2), BE1 402 contract (D1) |

**Critical path:** BE1 core loop (D1) → FE1/FE2 integration (D2) → full rehearsal (D3). BE2 and FE1/FE2 have significant unblocked runway on D0–D1; lead reviews contracts at D0 close.

---

## 5. Top-5 Hackathon Risks & Contingencies

1. **Wallet connect fails live** → Demo Wallet auto-approves (primary). Pera lane is a bonus, never required.
2. **No internet / projector issues** → local mode + SD of video backup; all fonts/assets bundled (no Google Fonts CDN at venue).
3. **Streaming feels slow** → MockProvider paces 40–80 tok/s; TTFT < 1s on localhost; stage-per-second pacing tuned during rehearsal.
4. **Demo data looks fake** → seeded realistic data (model names like `omniroute/llama-3.3-70b-instruct`, provider names, plausible latencies/token pricing in micro-USDC); each playground run *mutates* real rows (live demo).
5. **Scope creep** → freeze list on D3: anything not in this plan does not ship to demo. Extras go to `docs/adr/` notes for production.

---

## 6. Demo-Day QA Checklist (final gate)

- [ ] `make demo` clean-boot on demo laptop · seed visible (10 models, 6 providers, 100 USDC wallet)
- [ ] Playground: no-key request → 402 modal → approve → streaming → receipt (full loop < 10s)
- [ ] Replay same proof → 409 (show once if time)
- [ ] Consumer dashboard ticks after one generation (balance, tx list, usage chart)
- [ ] Provider dashboard revenue increments
- [ ] Admin settlement queue shows new receipt → completed
- [ ] Architecture page animation plays smoothly
- [ ] 1280px projector rendering OK · dark theme consistent
- [ ] `demo.mp4` + deployed URLs on standby
- [ ] Repo tagged `hackathon-aug10`

---

## 7. Relationship to Production Design

The MVP is deliberately the **slimmed spine** of the production architecture: same service boundaries (`gateway` vs `controlplane`), same OpenAI wire contract, same x402 headers (standard + alias), same DB shapes (`channels.locked/settled/latest_nonce`, `receipts.nonce UNIQUE`), same routing/meter/nonce package decomposition. The consumer-signed cumulative-state settlement model (Engineering Audit F-01 resolution) is respected: the demo proof is the consumer's Ed25519 signature over the canonical challenge — exactly what production `escrow_finalize` verifies on-chain. Swap-out path: MockProvider → real provider SDK; Demo Wallet → Pera/local keypair; in-memory nonce → Redis; `settlements` rows → real Algorand atomic-group submissions. **Nothing built here is throwaway.**