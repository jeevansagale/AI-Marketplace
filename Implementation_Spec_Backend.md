# OmniRoute AI Marketplace — Implementation Specification: Part 1 (Backend, Data, Blockchain)

**Version:** 1.0.0 — Staff-Engineer Implementation Bible. Companion to `Engineering_Design_Document.md` (architecture) and `Implementation_Roadmap.md` (schedule). Part 2 (`Implementation_Spec_Frontend_Ops.md`) covers Dashboard, Testing, Monorepo tree, Workflow, Config, CI/CD.

---

## 1. GATEWAY (`apps/gateway` — Go, Fiber, BE1)

### 1.1 Folder Structure & Package Purpose

```
apps/gateway/
├── cmd/gateway/main.go          # entrypoint: config → deps → server → signals
├── internal/
│   ├── app/app.go               # dependency graph (manual constructors; no codegen DI)
│   ├── config/config.go         # env→typed config; Validate() fail-fast
│   ├── server/server.go         # Fiber app, route registration, graceful shutdown
│   ├── middleware/              # recover, trace, requestid, ratelimit, idempotency, bodylimit
│   ├── x402/                    # challenge mint; proof verify; caveat parse; header names
│   ├── crypto/                  # ed25519 sign/verify; sha512/256 canonical hashing; macaroon HMAC
│   ├── nonce/                   # redis SETNX wrapper (idempotency + replay)
│   ├── channel/                 # redis hash cache {locked, spent, verified_at}; budget lock/unlock
│   ├── meter/                   # token counting: usage-block extract + byte heuristic
│   ├── parser/                  # SSE \n\n state machine (pure function, no I/O)
│   ├── proxy/                   # ReverseProxy, http2 mTLS transport, host pool, header rewrite
│   ├── router/                  # routing table store, provider selection, circuit breaker, EWMA
│   ├── receipt/                 # RequestReceipt domain + signer + NATS/Redis emitter
│   ├── ratelimit/               # token bucket via Redis INCR/EXPIRE
│   ├── grpclient/               # Registry/Payment/Auth service clients + reconnect backoff
│   ├── otel/                    # tracer + meter providers; span name constants
│   └── recover/                 # panic handler (log stack, never body)
├── test/e2e/                    # integration harness (testcontainers + LocalNet)
└── configs/config.yaml          # defaults; env overrides
```

### 1.2 Dependency Injection
- Manual `app.Deps` struct built in `cmd/gateway/main.go`; constructors take interfaces, not concrete types. No `wire`/codegen (team of 4; explicit > magic).
- Interface ownership rule: **consumer declares the interface** (Go idiom). e.g. `proxy` defines `NonceStore interface { SetNX(ctx, key, ttl) (bool, error) }`; `nonce.RedisNonceStore` satisfies it in `app`.

### 1.3 Middleware Order (fixed, non-negotiable)
`recover → otel.trace → requestid → ratelimit(ip) → idempotency → bodylimit(10MB) → x402.gate → ratelimit(wallet) → router.select → proxy`.

### 1.4 Routing Table
| Path | Handler group | Stream? |
|---|---|---|
| `/v1/chat/completions` | chat | yes/no (JSON `stream`) |
| `/v1/embeddings` | embeddings | no |
| `/v1/images/generations`, `/v1/images/edits` | images | no |
| `/v1/audio/transcriptions`, `/v1/audio/speech` | audio | yes (speech) / no |
| `/v1/models` | models listing (no proof needed) | no |
| `/healthz`, `/readyz` | ops | no |

401/404/405 use RFC 9457 problem+json.

### 1.5 Request Lifecycle (exact ordering)
1. TLS origin (Cloudflare) → ALB → Fiber.
2. Middleware chain §1.3.
3. x402 gate: no `X-402-Payment-Proof` → compute `C_max = base_fee + prompt_tokens*Pp + max_tokens*Pc` from routing-table price → mint challenge (nonce=16B random, escrow_id, C_max, expiry=+120s) → `402` + `X-402-Payment-Required` JSON challenge.
4. Proof present → parse canonical bytes → ed25519 verify (50µs) → caveat checks (escrow, model, max_cost, expiry) → body-hash match.
5. `SETNX nonce:{sha256(proof)}` TTL 300s. Fail → `409 nonce-replay`.
6. `HGETALL ch:{channel_id}` → require `balance = locked - settled ≥ C_max`; reject `403 insufficient-funds` otherwise.
7. `HINCRBY ch:{id} locked +C_max` (budget lock).
8. Router.select (see §1.9) → provider URL + circuit state.
9. Proxy with ctx: non-stream 120s, stream 600s. Connect/TLS ≤500ms.
10. Meter + emit receipt → NATS `omniroute.receipts` (flush 100ms/1000 msgs). Non-stream: `HINCRBY locked -C_max`, `HINCRBY spent +actual`.
11. Respond; buffers returned to `sync.Pool` in `defer`.

### 1.6 Stream Lifecycle & SSE Parser
- Stream: open → pump goroutine reads provider `http.Response.Body` → parser → per-chunk: meter, budget check, forward to client (`fiber.Ctx` writable via `c.Response().BodyWriter()`), eventually `[DONE]`.
- Parser spec: states `scanEvent → scanData → scanDone`; split strictly on `\n\n`; strip single leading BOM; reject lone `\r` (treat as `\n`); event size cap 1 MB; malformed events dropped + `parser.malformed_total` metric, never forwarded.
- Meter: capture trailing `usage` block on normal `[DONE]`; fallback byte-heuristic `≈ tokens = bytes/4` (per-model configurable). Authoritative = Gateway count; if provider usage deviates >30% from heuristic, use min(GW, heuristic*1.3) and flag `meter.suspect_total`.
- Budget enforcement: if cumulative cost ≥ locked → `event: x402_reauth_required` (3s grace) then synthetic `event: error` (402 problem+json) + RST upstream.
- Client disconnect: ctx cancel → `Transport` aborts upstream → receipt for last forwarded chunk emitted anyway.

### 1.7 Reverse Proxy Spec
- `httputil.ReverseProxy` + custom `http.Transport`: HTTP/2 (mTLS via cert-manager-issued client cert), `MaxIdleConns=512`, `MaxIdleConnsPerHost=200`, `IdleConnTimeout=90s`, `TLSHandshakeTimeout=500ms`, `DialContext 2s`, `ResponseHeaderTimeout=30s` (stream: 0).
- `Rewrite` middleware: strip `Authorization`, `X-402-*`; inject `X-Request-ID`, `traceparent`, `X-Provider-Secret` (per-provider, from routing table).
- Request body tee: single read into `[]byte` (sync.Pool, ≤10MB) for hash + retry-free forwarding. Copy once, forward via `bytes.Reader` (avoids double-drain).

### 1.8 Goroutines, Worker Pools, Memory
- 1 goroutine/connection. Receipt emit path: `chan RequestReceipt` cap 1024 + 1 writer goroutine batching to NATS (or Redis XADD initial fallback).
- Buffers: `sync.Pool` 16KB start; returned in defer; streamed SSE chunks copied out of pool (never retained).
- GC: `GOGC=200`, `GOMEMLIMIT=1GiB` per pod. FD `ulimit` 1M via EKS node sysctls.

### 1.9 Routing Engine & Provider Selection
- Score = `0.3*price_norm + 0.3*latency_norm + 0.25*uptime + 0.15*reputation`. Filter: active, circuit closed, region-adjacent, budget ≤ channel. Weighted random within top-K=3 (anti-herd). EWMA latency α=0.1 per provider. Circuit: 5 consecutive 5xx → open 30s → half-open probe.
- Retry policy: no blind retries on paid path (nonce consumed). Provider 502 → release lock, respond `502` + fresh challenge (new nonce) so client can retry safely.

### 1.10 Telemetry, Panic Recovery, Config, Lifecycle
- Metrics (Prometheus): `gateway_requests_total{status,model,provider}`, `gateway_proxy_latency_seconds{quantile}`, `gateway_ttft_seconds`, `gateway_active_streams`, `gateway_challenges_minted_total`, `gateway_budget_truncations_total`, `gateway_receipts_emitted_total`, `gateway_nonce_replays_total`.
- Logs: `slog` JSON; fields `trace_id, request_id, channel_id, model, provider, cost_micro, status`; debug sampled 5%; **payloads never logged** (CI grep gate).
- Config: env → config struct; `Validate()` required: REDIS_ADDR, NATS_URL, CTRL_GRPC_ADDR, GATEWAY_PORT, CERT paths, ROUTING_TABLE_REFRESH_MS(60000).
- Startup: load config → otel → redis → NATS → routing snapshot → gRPC watch start → HTTP serve → `readyz` true.
- Shutdown: SIGTERM → stop ingress (drain 30s) → flush receipt channel → close pools → exit 0. Kubernetes preStop hook sleeps 5s.

---

## 2. CONTROL PLANE (`apps/controlplane` — Go, BE2)

### 2.1 Package Organization & Layering
```
apps/controlplane/
├── cmd/controlplane/main.go    # API server
├── cmd/workers/main.go         # background workers (single binary, subcommand per worker)
├── internal/
│   ├── api/           # controllers only (thin; validate → call service → write response)
│   ├── auth/          # jwt issue/verify, apikey hash/verify, macaroon mint; RBAC middleware
│   ├── payment/       # channel & receipt domain services
│   ├── settlement/    # scheduler, batch builder, submitter, DLQ consumer
│   ├── registry/      # provider/model/pricing services
│   ├── webhooks/      # dispatcher + HMAC signer + retry queue
│   ├── disputes/      # dispute lifecycle service
│   ├── analytics/     # reputation, usage aggregation, materialized view refresher
│   ├── reconciliation/# escrow diff worker
│   ├── store/         # sqlc repositories + pgx; WithTx helper
│   ├── db/migrations/ # SQL files 0001..NNNN
│   ├── nats/          # consumers (receipts, webhooks), publishers (routing.updated)
│   ├── grpcserver/    # RegistryService, PaymentService, AuthService impls
│   ├── algorand/      # algod+indexer clients, tx group builder, box reader
│   └── workers/       # entrypoints: health, reconcile, retention, mview, settlement
```
**Rule:** controllers never touch `store`; services own transactions via `store.WithTx(ctx, fn(pgx.Tx) error)` — one tx per service method; no nested txs.

### 2.2 Domain Services & Responsibilities

| Service | Key methods | Data owned |
|---|---|---|
| `registry.ProviderService` | Register, VerifyWalletChallenge, Approve(gate), Suspend, List | providers |
| `registry.ModelService` | CreateModel (uniqueness: provider+name+type), UpdatePrice, ToggleActive, PublishRouting | models, model_capabilities, routing_rules → NATS `omniroute.routing.updated` |
| `auth.APIKeyService` | Create(sk-… returned once), Hash, Verify, Rotate, Revoke | api_keys (hash only) |
| `auth.JWTService` | Issue (15m), Refresh (7d, rotate), Blacklist on logout (Redis 5m) | — |
| `payment.ChannelService` | Open, HydrateFromL1, Reauth, Close(timelock), GetStatus | channels; hydrates `ch:{id}` in Redis |
| `payment.ReceiptService` | Record (unique nonce, journal insert), AggregateByChannel, MarkSettled | receipts |
| `settlement.BatchService` | Build (fold→latest cumulative, threshold ≥0.05 USDC, group ≤16 txns), Submit (algod), Confirm, Fail→DLQ | settlement_batches, _items |
| `disputes.DisputeService` | Open (24h hold), FreezePayout, Resolve, RefundConsumer, ReleasePayout | disputes; holds in settlement state |
| `webhooks.EventService` | Enqueue, Dispatch (HMAC-SHA256 sig header `X-Omniroute-Signature`), Retry (5x exp backoff 1s→60s), Expire→DLQ | webhooks, deliveries |
| `analytics.AnalyticsService` | RefreshMV, ComputeReputation (geomean weights), SafeHealthSnapshot | health_snapshots, analytics mviews |
| `reconciliation.Service` | DiffPostgresVsL1 (per channel), PendingPayoutRelease (24h) | channels, settlement state |

### 2.3 Settlement Scheduler (cron spec)
- `settlement` worker: every 5 min (or channel.close event / 80% depleted). Sequence: XREADGROUP `omniroute.receipts` → journal → batch by channel → skip dust → build atomic group (`escrow_finalize` app call + provider USDC transfer (net 1% fee) + fee collector transfer) → submit via algod (KMS-signed, nonce-managed single leader via Redis lock) → on confirm mark settled; on fail → `omniroute.dlq.receipts` + P1 metric `settlement_failures_total`.
- **Leader election:** Redis lock `ctl:settlement:leader` TTL 60s refreshed 30s. Only leader builds/submits (L1 account nonce ordering).

### 2.4 Receipt Journal & Reconciliation
- Insert `receipts` with `ON CONFLICT (nonce) DO NOTHING` — replay-safe even if Redis was flushed.
- Reconciliation worker: 1-min cadence; per open channel compare `channels.settled_micro_usdc` vs L1 box `settled`; delta>0 → `reconcile_delta_micro_usdc` gauge + P1 alert. Auto-release provider holds after 24h if no dispute.

### 2.5 Background Workers (cron table)
| Worker | Schedule | Job |
|---|---|---|
| health | 30s | ping provider `/v1/models`; EWMA update |
| synthetic | 60s | 5-token completion per model; write health_snapshots |
| reconcile | 1m | §2.4 |
| mview-refresh | 1m | `REFRESH MATERIALIZED VIEW CONCURRENTLY` |
| retention | daily 03:00 | prune raw receipts >90d (keep aggregates+on-chain), health raw >7d |
| webhook-sweep | 1m | redeliver retryable deliveries |
| settlement | 5m | §2.3 |
| audit-rebase | 24h | copy audit_log >30d to cold storage, mark archived |

### 2.6 Admin System
- RBAC roles `consumer|provider|admin`; admin endpoints: provider approve/suspend, dispute ruling, settlement batch manual replay, channel freeze (contract-level freeze call), fee config. Multi-sig approval (2-of-3 admin keys) required for: freeze, manual refund, fee change. Everything → `audit_log` (immutable, no UPDATE/DELETE grants).

### 2.7 gRPC Server (Registry/Payment/Auth)
Protos in `proto/`; impls in `grpcserver/`. `RegistryService.WatchRoutingTable` server-stream: subscribes NATS `omniroute.routing.updated` + current snapshot. `PaymentService.HydrateChannel`: reads L1 box (indexer cache 5s) → writes `ch:{id}`. `AuthService.MintChallenge`: mint + sign challenge (hot key in memory, rotated hourly; cold in KMS).

---

## 3. PROVIDER SDK (`packages/sdk-provider` — Go + optional Python)
- **Architecture:** convenience CLI + thin library wrapping Control Plane REST. Never proxies traffic (Gateway does).
- **Registration:** `omniroute-provider register --url https://… --model llama-3-70b --price-usdc-1m 2.50` → returns provider API key (shown once) + mTLS CSR flow via cert-manager (EC cert). Wallet verification: sign challenge → POST verify.
- **Heartbeat:** 30s `POST /v1/provider/telemetry` (uptime, queue depth, active batches, tokens/s). Health endpoint spec: `GET /v1/models` must return OpenAI schema list; synthetic completion must return ≤5 tokens <2s.
- **Auth:** header `Authorization: Bearer sk-provider-…`; TLS client cert pinning in transport.
- **Stream handling:** SDK does not proxy; wraps vLLM/TGI via OpenAI-compatible endpoint only (documented env mapping: `OLLAMA_BASE`, `VLLM_MODEL`, etc).

## 4. CONSUMER SDK (`packages/sdk-python`, `packages/sdk-typescript`)
- **Architecture:** wraps official OpenAI SDK by swapping `base_url` and injecting an httpx/axios-backed 402 interceptor; all crypto in one module (`x402/`) shared via a small WASM or thin re-implementation (ed25519 + sha512/256 only — no heavy deps).
- **402 interceptor:** on `402` → parse challenge JSON → `ensure_channel()` (open+fund escrow if none/insufficient, config `min_balance_micro_usdc` default 10M = $10) → sign proof (canonical bytes §EDD 9.3) → retry with `X-402-Payment-Proof`. Backoff 1s→2s→4s, max 3 attempts, only after `402`; other statuses pass through.
- **Stream reconnect:** SSE `x402_reauth_required` event → sign top-up proof → `POST /v1/marketplace/state-channels/reauth` → resume by re-issuing request with same nonce+new budget (server replay-tolerant: same nonce but new locked C_max; nonce SETNX expires after 300s — **must complete reauth within TTL**; else mint new nonce).
- **Wallet abstraction:** interface `Wallet { Sign(msg []byte) (sig, addr, error); OptInUSDC(txParams) }`; impls: local keypair (env), Pera/Defly via WalletConnect (TS only), vault KMS (server agents).
- **Configuration:** `OMNI_BASE_URL`, `OMNI_WALLET_*`, `OMNI_CHANNEL_MIN_USDC`, `OMNI_AUTO_TOPUP_AMOUNT_USDC`, `OMNI_MAX_RETRIES`. Middleware hooks: `on_402`, `on_reauth`, `on_settlement` for observability.

---

## 5. DATABASE SPECIFICATION (PostgreSQL)

### 5.1 Tables, Columns, Constraints, Indexes (exact)

| Table | Columns (type) | Constraints / FKs | Indexes |
|---|---|---|---|
| `users` | id uuid PK default gen_random_uuid(); role text check in ('consumer','provider','admin'); email citext UK; created_at timestamptz | email unique | idx_users_role |
| `providers` | id uuid PK; user_id uuid FK→users; wallet_address text UK; base_url text; status text check ('active','suspended','inactive'); reputation_score numeric(7,2) default 0; created_at | FK users; UK wallet_address | idx_providers_status |
| `models` | id uuid PK; provider_id uuid FK→providers; name text; type text check ('chat','embedding','image','audio'); price_per_1m_tokens_usdc bigint ≥0; base_fee_micro_usdc bigint ≥0; is_active boolean default true; capabilities jsonb; context_window int; created_at | FK providers; UNIQUE(provider_id,name,type) | idx_models_name_active (name,is_active) GIN pg_trgm(name,capabilities) |
| `model_capabilities` | id uuid; model_id FK; capability text check (...) | FK models; UNIQUE(model_id,capability) | — |
| `routing_rules` | id uuid; model_id FK; weight numeric(5,4) ≥0; is_primary bool | FK models | idx_routing_model |
| `api_keys` | id uuid; user_id FK; key_hash text UK (sha256 hex); prefix text (first 8 chars, display only); role text; scopes text[]; expires_at timestamptz | FK users; UK key_hash | idx_apikeys_user |
| `wallets` | id uuid; user_id FK; algorand_address text UK; asa_id bigint default 31566704; nickname text | FK users; UK algorand_address | idx_wallets_user |
| `channels` | id uuid; wallet_id FK; app_id bigint; locked_micro_usdc bigint default 0; settled_micro_usdc bigint default 0; latest_nonce bigint; status text check ('open','closing','closed','disputed'); opened_at; closed_at; box_sha text UK | FK wallets; UK box_sha | idx_channels_status |
| `receipts` | id uuid; channel_id FK; nonce text UK; delta_micro_usdc bigint; cumulative_spend_micro_usdc bigint; prompt_tokens int; completion_tokens int; model_id uuid FK→models; provider_id uuid FK→providers; gateway_signature text; status text check ('pending','settled','dead'); created_at | FK channel/models/providers; UK nonce | PARTITION BY RANGE(created_at); idx_receipts_channel_created (channel_id, created_at DESC); UK nonce |
| `settlement_batches` | id uuid; algo_tx_group_id text UK; status check ('submitted','confirmed','failed'); submitted_at; confirmed_at | UK algo_tx_group_id | idx_batches_status |
| `settlement_batch_items` | id uuid; batch_id FK; provider_id FK; channel_id FK; amount_micro_usdc bigint; fee_micro_usdc bigint; released_at timestamptz (payout hold) | FK batch/provider/channel | idx_items_batch |
| `health_snapshots` | id bigserial; provider_id FK; ts timestamptz default now(); p50_latency_ms float; p99_latency_ms float; uptime_24h float; error_rate float | FK providers | idx_health_provider_ts (provider_id, ts DESC) — **hypertable (TimescaleDB)** |
| `disputes` | id uuid; receipt_id FK→receipts; claimant_id uuid FK→users; reason text; evidence_ref text (S3 key); status check ('open','under_review','ruled','refunded'); created_at; resolved_at | FK receipts/users | idx_disputes_status |
| `webhooks` | id uuid; user_id FK; url text; event_types text[]; secret text (HMAC key, encrypted); is_active bool; created_at | FK users | idx_webhooks_user |
| `deliveries` | id bigserial; webhook_id FK; event_type text; payload_hash text; status check ('pending','delivered','failed','expired'); attempt int; next_retry_at; last_http_status int | FK webhooks | idx_deliveries_retry (status, next_retry_at) WHERE status IN ('pending','failed') |
| `audit_log` | id bigserial; actor_id uuid nullable; action text; entity_type text; entity_id uuid; before jsonb; after jsonb; ip inet; created_at | no FK (append-only) | idx_audit_entity (entity_type, entity_id, created_at DESC) |

### 5.2 Migrations (order & rules)
- `db/migrations/0001_users_providers_models.sql` → `0002_api_keys_wallets.sql` → `0003_channels.sql` → `0004_receipts_partitioned.sql` (create parent + monthly partitions) → `0005_settlement.sql` → `0006_health_timescale.sql` (CREATE EXTENSION timescaledb; convert) → `0007_disputes.sql` → `0008_webhooks.sql` → `0009_audit.sql` → `0010_analytics_mviews.sql` (+ trigger function `receipt_insert_rt()` auto-creates next month partition).
- Rules: forward-only; expand-contract; no destructive DDL in prod PR (two-PR pattern: add→migrate→drop in later release); every migration has `down` for local, no down in prod.
- **Partitioning:** `receipts` by RANGE(created_at) monthly, partition name `receipts_y2026m08`; indexes per-partition via template.
- **Retention:** raw receipts 90d (prune task moves to `receipts_archive` cold table or Parquet via S3 then drop partition); health 7d raw + 1h downsample; audit 7y; settlement tables infinite (on-chain reference).
- **Backup:** RDS PITR (7d), automated daily snapshot + WAL to S3; weekly restore test to `restore-validate` cluster; manual pre-migration snapshot gate in deploy job.

---

## 6. REDIS SPECIFICATION

Two logical instances (same cluster, separate logical DBs for cache vs queues if single cluster):

| Instance | Purpose | maxmemory | eviction |
|---|---|---|---|
| `redis-cache` | nonces, rate limits, routing cache, channel cache, idempotency, API-key cache | 6 GiB | `allkeys-lru` |
| `redis-queue` (MVP fallback) | settlement/webhook streams | 4 GiB | `noeviction` (streams must not evict) |

Keys (exact):
| Key | Type | TTL | Notes |
|---|---|---|---|
| `nonce:{sha256(proof)}` | string SETNX | 300s | replay guard |
| `rl:ip:{ip}` / `rl:wallet:{addr}` | string counter | 60s window | INCR + EXPIRE; Lua token bucket |
| `idem:{key}` | string (status+body) | 24h | 202 in-flight lock 60s |
| `rs:{model}:{region}` | sorted set (provider_id→score) | 60s refresh | routing table |
| `ch:{channel_id}` | hash {locked, spent, verified_at} | 15m refresh | channel balance cache |
| `rep:{provider}` | string | 60s | cached reputation |
| `ak:{sha(key)}` | hash {user_id, role} | 5m | API key cache |
| `ctl:settlement:leader` | string | 60s (extend 30s) | settlement leader lock |

Streams (only if MVP Redis-queue mode, else NATS — see §7): `stream:receipts` (consumer group `settlement-worker`), `dlq:settlement`. All Gateway Redis ops use `EX`, pipelining where safe; **no blocking commands in gateway path**.

## 7. NATS JETSTREAM SPECIFICATION

| Subject | Stream | Consumers (durable, ack wait 30s, max_deliver 5 → DLQ) | Payload |
|---|---|---|---|
| `omniroute.receipts` | `RECEIPTS` (Raft: 3) | `settlement-worker` | RequestReceipt JSON (idempotent via nonce) |
| `omniroute.routing.updated` | `ROUTING` | `gateway-*` (each gateway; receive ephemeral) | routing delta |
| `omniroute.channels.events` | `CHAN_EVENTS` | `controlplane-api` | open/close/reauth/close-race |
| `omniroute.webhooks` | `WEBHOOKS` | `webhook-worker` | delivery job |
| `omniroute.dlq.receipts` | `DLQ` | `ops-console` (no auto-ack) | dead receipts |

- Retry: JetStream redelivery (1s→5s→30s→2m→5m), then DLQ. Publish acks (JetStream PublishAsync with ack flush 100ms). **Queue abstraction interface** in `internal/nats/queue.go` so Redis Streams can stand in during dev (LocalNet) with zero code elsewhere.

## 8. OPEN TELEMETRY SPECIFICATION

- **Trace names (span):** `gateway.ingress`, `gateway.auth.challenge`, `gateway.auth.verify` (attrs: alg, ok), `gateway.nonce.check`, `gateway.channel.lock`, `gateway.route.select`, `gateway.proxy.dial`, `gateway.proxy.request`, `gateway.stream.parse`, `gateway.meter.count`, `gateway.receipt.emit`, `controlplane.api.{method}`, `controlplane.settlement.batch`, `controlplane.settlement.submit`, `controlplane.reconcile.diff`, `controlplane.webhooks.dispatch`.
- **Attributes (mandatory):** `channel_id`, `model`, `provider_id`, `nonce_prefix` (first 8 hex), `cost_micro_usdc`, `tokens_prompt`, `tokens_completion`, `http.status`, `region`. **Never:** prompt/response/body.
- **Propagation:** W3C `traceparent` injected to providers + NATS messages + Postgres via `pg_tracelink` (optional; skip MVP).
- **Metrics:** gateway RED (§1.10) + `settlement_batches_total{status}`, `settlement_latency_seconds`, `reconcile_delta_micro_usdc`, `escrow_tvl_micro_usdc`, `webhook_delivery_total{status}`, `redis_*` (from exporter). Exporters: OTLP → Tempo/Tempo-OSS or Grafana Cloud; Prometheus scrape `/metrics`.
- **Logs:** `slog` JSON ↔ OTLP log correlation via `trace_id`; Loki retention 30d.

## 9. BLOCKCHAIN / STATE CHANNEL SPEC (TEAL v8 — AlgoKit, BE1)

### 9.1 Contract (`OmniRouteEscrow.approval.teal`) — ABI methods
| Method | Args | Logic (exact) |
|---|---|---|
| `escrow_open` | pay USDC, pay ALGO (MBR), consumer_addr, provider_allowlist[] | create Box(sha256(consumer‖gateway)) = [locked, settled=0, latest_nonce=0]; assert MBR covered; enable `escrow_add`/`escrow_close` only from creator or consumer via `sender` |
| `escrow_deposit` | pay USDC, nonce | locked += amount; nonce must increase |
| `escrow_reauth` | new_max cost, consumer sig | verify sig over latest cumulative; store intent |
| `escrow_finalize` | cumulative_spend, gw_sig, provider_addr, recipient_net | verify gw sig + **consumer-signed cumulative proof chain**; settled = cumulative_spend (≤ locked); group txn: asset_transfer → provider (net) + marketplace fee; box update; fails atomically if any leg fails |
| `escrow_dispute` | receipt_id, bond | freeze; after 24h+resolve via admin multi-sig |
| `escrow_close` | — | timelock 24h; refund `locked - settled` + MBR ALGO refund; box deleted |
- **Box layout:** `Box0 = [u64 locked, u64 settled, u64 latest_nonce, byte[32] root_hash]` (serialized via `extract`/`replace`). Fee math in micro-USDC, integer only; fee rate passed as arg (e.g., 10000 = 1%).
- **Payments:** USDC ASA 31566704 (mainnet), testnet uses Testnet USDC app id configured per env; `escrow_open` requires opt-in by contract app (global/local).
- **Algorand nodes:** internal NLB → algod (archival) + indexer; KMS-backed fee-sink/settlement key (never in pod env). Group limit ≤16 txns enforced by batch builder.

## 10. GATEWAY & CONTROL PLANE TESTING (per package)
- **Unit:** parser (fuzz table: split/CR/BOM/oversize), meter (usage vs heuristic matrix), crypto (known vectors), settlement math (fee/dust/folding), circuit breaker states.
- **Integration (testcontainers + LocalNet):** full 402 flow; nonce replay; budget truncation; provider 502 → fresh challenge; close-race.
- **Gateway conformance:** replay OpenAI golden payloads (stream/non-stream, error shapes) in CI PR gate.

— End Part 1. Part 2: Dashboard, Testing matrix, complete monorepo tree, developer workflow, .env spec, Docker/K8s/CI, docs, coding standards, commit/branch/versioning/release strategy.