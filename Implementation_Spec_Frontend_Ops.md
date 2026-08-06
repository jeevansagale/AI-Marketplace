# OmniRoute AI Marketplace — Implementation Specification: Part 2 (Frontend, Testing, Workflow, Ops)

**Version:** 1.0.0. Companion to Part 1 (`Implementation_Spec_Backend.md`), `Engineering_Design_Document.md`, `Implementation_Roadmap.md`.

---

## 11. DASHBOARD (`apps/web` — Next.js 15, React 19, BE1=FE1/FE2)

### 11.1 Folder Structure & Purpose (exact)
```
apps/web/
├── app/
│   ├── layout.tsx                  # root layout: ThemeProvider, QueryProvider, Toaster
│   ├── (marketing)/                # landing, pricing, docs — SSG/ISR, no auth
│   │   ├── page.tsx, pricing/page.tsx, docs/[...slug]/page.tsx
│   ├── (app)/                      # authenticated shell (route group)
│   │   ├── layout.tsx              # AppShell + SessionGuard (redirect /login if no session)
│   │   ├── marketplace/            # search: page.tsx, [model]/page.tsx, search-params query
│   │   ├── consumer/               # overview, channels, api-keys, webhooks, disputes, settings
│   │   ├── provider/               # models, analytics, payouts, health, reputation
│   │   ├── admin/                  # providers, disputes, settlements, governance
│   │   └── login/page.tsx          # wallet-sign / OAuth
├── components/
│   ├── ui/                         # shadcn/ui registry components only (button, dialog, table…)
│   ├── layout/                     # AppShell, Sidebar, Topbar, PageHeader
│   ├── marketplace/                # ModelCard, ModelGrid, SearchFilters, PricingBadge
│   ├── consumer/                   # ChannelList, SpendChart, APIKeyTable, WebhookForm
│   ├── provider/                   # ModelEditor, PayoutHistory, HealthTable
│   ├── wallet/                     # ConnectWalletModal, BalanceCard, ChannelDepositForm
│   ├── charts/                     # LineArea, Bar, Donut, Sparkline (Recharts wrappers)
│   └── shared/                     # StatCard, SkeletonCells, EmptyState, ErrorState, DataTable
├── lib/
│   ├── api/client.ts               # fetch wrapper: base URL, JWT + refresh, 401 → login redirect
│   ├── api/queries.ts              # React Query hooks (one per endpoint; useQuery keys table)
│   ├── api/keys.ts                 # query key builders {usage:[], channels:[], models:[]}
│   ├── stores/useUi.ts             # Zustand: sidebar, theme, wallet-modal, filters
│   ├── auth/session.ts             # server-only session read (cookies); client via /api/session
│   ├── utils.ts                    # cn(), formatUSDC(), formatTokens(), timeAgo()
│   └── wallet/                      # pera/defly/walletconnect adapters → Wallet interface
├── hooks/                          # useDebounce, useSSE, useWallet, useConnectedWallet
├── styles/globals.css
├── middleware.ts                   # (optional) edge session check for (app) routes
├── vercel.json / next.config.ts    # ISR revalidates, rewrites /v1 → api.omniroute.ai
└── vitest.config.ts, playwright.config.ts
```

### 11.2 State Management Rules
- **React Query owns all server state** — single source of truth for dashboards/tables.
  - `staleTime`: 30s (tables), 60s (marketplace), `refetchOnWindowFocus`: false.
  - Mutations: optimistic updates for channels/api-keys; `invalidateQueries` on success.
- **Zustand owns only client UI state** — theme, sidebar, wallet modal, active filters. Zero async logic in stores; wallet connection persists via Zustand `persist` (address only, never keys).
- **URL is state for marketplace** — filters/sort/pagination live in `searchParams` (shareable links).

### 11.3 Routing & Auth
| Route | Guard | Data |
|---|---|---|
| `/(marketing)` | public | static/ISR |
| `/login` | public | wallet-sign + OAuth (GitHub/Google) |
| `/consumer/*` | role=consumer | REST + SSE burn-rate |
| `/provider/*` | role=provider | REST |
| `/admin/*` | role=admin | REST, multi-sig UI for freeze/refund |

Session flow: wallet-signature → Control Plane `JwtService.Issue` → httpOnly cookie (`__host-omniroute.session`, Secure, SameSite=Lax, 15m) + `refresh_token` cookie (7d, rotation). API client sends cookie automatically; on 401 → silent refresh → redirect `/login` on refresh failure.

### 11.4 Table Components, Charts, Loading, Errors
- **DataTable** (shadcn Table wrapper): props `{ columns, data, loading, emptyState, error, pagination, onSort }`; server-side pagination via cursor (`X-Next-Cursor` header); skeletons = `SkeletonCells` matching column count, 8 rows.
- **Charts** (Recharts): `LineArea` (spend over time), `Bar` (per-model spend), `Donut` (cost by category), `Sparkline` (latency p50 over 24h). All wrapped in `ResponsiveContainer`; light/dark theming via CSS vars.
- **Loading:** route-level `loading.tsx` (skeletons per page); mutation buttons show spinner + disabled; buttons expose optimistic state.
- **Error handling:** `ErrorBoundary` per route segment → `ErrorState` with retry button; API errors normalized to ProblemDetails: `{ title, detail, error_type }` → toast + inline field errors.
- **Realtime:** `useSSE('/v1/consumer/usage/stream')` hook re-validates React Query keys on `settlement` events only (bounded re-renders).

## 12. TESTING MATRIX (entire repo)

| Layer | Scope | Tool | Where | Gate |
|---|---|---|---|---|
| Unit | parser, meter, crypto, settlement math, CI/UI utils | `go test`, `vitest` | both apps + packages | PR |
| Contract | TEAL all methods incl. dispute/close paths | `algokit test`, DAppFlow | packages/contracts | PR |
| Integration | gateway↔redis↔controlplane↔postgres↔LocalNet | testcontainers | apps/gateway/test | PR |
| Conformance | OpenAI golden replay (stream/non-stream/errors) | custom harness | apps/gateway/test | PR + staging deploy |
| E2E | 402 → fund → prove → stream → settle → dashboard | Playwright | apps/web/e2e | PR (smoke) + nightly (full) |
| Load | 10k streams, burst 402s, settlement stress | k6 + ghz | test/load | staging gate, nightly |
| Chaos | Redis kill (fail-closed + rehydrate), NATS drop, provider 502 | `chaos-mesh`/`pumba` | staging weekly | review |
| Security | gosec, semgrep, trivy, `npm audit`, secret scan (gitleaks) | CI | PR + nightly | PR blocker |

### 12.1 Critical Scenario Tests (must exist as named tests)
1. `double_spend_replay_rejected` — same proof twice → 409.
2. `budget_truncation_stream_reauth` — stream hits locked C_max → reauth event → truncation on timeout.
3. `provider_usage_inflation_capped` — provider sends usage 10× heuristic → billed at min(GW, 1.3×heuristic), flagged.
4. `client_disconnect_partial_billed` — TCP drop → provider RST, partial receipt.
5. `redis_flush_replay_still_blocked` — AOF loss → Postgres nonce journal blocks at settlement.
6. `settlement_batch_atomic_failure` — one leg fails → whole group fails → DLQ → replay converges.
7. `close_race_stream_dropped` — escrow_close during stream → cache invalidated, stream dropped, no bill.
8. `channel_dust_rolls_over` — < 0.05 USDC rolls over; close forfeits below-fee dust.
9. `reconciliation_delta_alert` — Postgres vs L1 mismatch raises P1 gauge.
10. `sdk_402_auto_fund_and_reauth` — Python SDK end-to-end incl. mid-stream reauth within 300s TTL.

## 13. COMPLETE MONOREPO TREE (every top-level entry + purpose)

```
omniroute/                                    # single repo; pnpm workspaces + go workspaces
├── .github/workflows/                        # ci.yml, conformance.yml, deploy-staging.yml,
│                                             # deploy-prod.yml, release-sdks.yml, nightly.yml
├── apps/
│   ├── gateway/                              # Go data plane (Part 1 §1)
│   ├── controlplane/                         # Go API + workers (Part 1 §2)
│   └── web/                                  # Next.js dashboard (Part 2 §11)
├── packages/
│   ├── sdk-python/                           # consumer x402 SDK (Part 1 §4)
│   ├── sdk-typescript/                       # consumer x402 SDK + wallet adapters
│   ├── sdk-provider/                         # provider CLI + telemetry lib (Part 1 §3)
│   ├── contracts/                            # TEAL source, algokit project, deploy scripts, tests
│   ├── proto-ts/                             # generated protobuf (web consumer of gRPC-adjacent DTOs)
│   └── config/                               # shared `.env` schemas, zod validation, defaults
├── proto/                                    # *.proto sources (Registry/Payment/Auth)
├── openapi/                                  # OpenAPI 3.1 + spectral rules + examples
├── infra/
│   ├── docker/                               # Dockerfiles (gateway, controlplane, web)
│   ├── docker-compose/dev.yml                # LocalNet dev environment
│   ├── k8s/                                  # manifests + kustomize/helm
│   │   ├── base/                             # namespaces, secrets (external), svc, hpa
│   │   ├── overlays/staging/                 # Testnet, small replicas
│   │   ├── overlays/prod/                    # Mainnet, multi-AZ
│   │   └── charts/omniroute/                 # helm chart (param = image tag, replicas, env)
│   └── terraform/                            # EKS, RDS, ElastiCache, NATS, ALB, Cloudflare
├── scripts/                                  # makefile targets impls, release tooling, hopsworks
├── test/
│   ├── load/                                 # k6 scripts + ghz protos
│   └── chaos/                                # pumba/chaos-mesh scenarios
├── docs/
│   ├── adr/                                  # ADR-001…NNN (append-only)
│   ├── runbooks/                             # P1 playbooks (redis-down, settle-fail, key-compromise)
│   └── api/                                  # generated reference from openapi/
├── .env.example                              # root: shared dev env (Part 2 §16)
├── Makefile / Taskfile.yml                   # dev, test, lint, gen, deploy, release
├── package.json / pnpm-workspace.yaml        # web + packages
├── go.work                                   # gateway + controlplane modules
├── golangci.yml, eslint.config.mjs, prettier
├── trivy.yml, gitleaks.toml, .spectral.yaml
├── README.md                                 # quickstart
└── LICENSE
```

**Why each exists:** `proto/` + `openapi/` are the contract hub (schema-first); `packages/config` guarantees env parity across apps; `infra/k8s/overlays` keeps env drift minimal; `test/chaos` operationalizes risk items from the EDD; `docs/runbooks` map 1:1 to P1 alerts (§29 EDD).

## 14. DEVELOPMENT WORKFLOW

### 14.1 Branch Strategy — Git Flow (scaled down) + Trunk-First
- `main` — always deployable (protected; requires CI + 1 approval).
- `feat/{scope}-{slug}` — e.g. `feat/gateway-sse-parser`, `feat/controlplane-settlement-batch`.
- `fix/{scope}-{slug}`, `chore/…`, `docs/…`.
- `release/vX.Y.Z` — cut from `main`; only hotfixes/spec/version bumps; merged back.
- **No long-lived feature branches** (> 3 days). Flagged/dead code merged behind feature flags.

### 14.2 PR Flow & Code Review
1. Draft PR early → link issue (#NN) + ADR if deviation → CI job block.
2. Review checklist (applies to all): financial path correctness (nonce/receipt math), no payload logging, interface ownership, OTel span coverage, tests for new behavior, migration safety.
3. Required: 1 approval from owning BE, plus **BE1 ↔ BE2 cross-review when touching proto/OpenAPI/TEAL**.
4. Merge: squash (`CONVENTIONAL SCOPE: description (#NN)`); delete branch.
5. CI gates: lint → unit → build → trivy → conformance (gateway/proto/openapi changes) → contract-e2e (contracts changes).

### 14.3 CI Flow (GitHub Actions `ci.yml`)
`on: pull_request` → matrix jobs (gateway lint+test, controlplane lint+test, web lint+test+build, sdk unit) → docker build + trivy scan → conformance (if paths) → contract-e2e (if paths) → required check `ci` passes.

### 14.4 Deployment & Rollback
- **Staging:** merge to `main` → `deploy-staging.yml` (build, tag `sha-{gitsha}`, helm upgrade staging, smoke test — buy-then-refund probe).
- **Prod:** `deploy-prod.yml` on `release/*` or `v*` tag → manual approval → Blue/Green Gateway (ALB 0%→100%), rolling Control Plane (preStop 30s drain), DB migrations as separate pre-rollout job (expand-contract).
- **Rollback:** `git revert` + redeploy previous image tag; DB rollback = forward-only compensating migration (never reverse); Gateway rollback is instant (stateless).

## 15. ENVIRONMENT VARIABLES SPEC (`env-schema`)

**Shared (`packages/config` zod schema; `.env.example` root):**
```
NODE_ENV=development
ENV=dev|staging|prod                # selects overlay
LOG_LEVEL=info
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

**Gateway:** `GATEWAY_PORT=8080`, `REDIS_ADDR=redis-cache:6379`, `NATS_URL=nats://nats:4222`, `CTRL_GRPC_ADDR=controlplane:50051`, `ROUTING_TABLE_REFRESH_MS=60000`, `RECEIPT_FLUSH_MS=100`, `RECEIPT_FLUSH_COUNT=1000`, `MAX_BODY_BYTES=10485760`, `MAX_STREAM_EVENT_BYTES=1048576`, `PAL_ENV`/`CERT_DIR`, `OTEL_SERVICE_NAME=omniroute.gateway`.

**Control Plane:** `PORT=8080`, `DATABASE_URL=postgres://omniroute:omni@postgres:5432/omniroute?sslmode=disable`, `REDIS_ADDR`, `NATS_URL`, `ALGOD_ADDR`, `ALGOD_TOKEN` (secret), `INDEXER_ADDR`, `INDEXER_TOKEN` (secret), `USDC_ASA_ID=31566704`, `ESCROW_APP_ID`, `SETTLEMENT_FEE_RATE_BASIS_POINTS=100`, `SETTLEMENT_MIN_MICRO_USDC=50000` (0.05), `JWT_SECRET` (secret), `MACAROON_ROOT_KEY_ARN`, `MARKETPLACE_FEE_ADDR`.

**Web:** `NEXT_PUBLIC_API_BASE=https://api.omniroute.ai/v1`, `SESSION_COOKIE_SECURE=true`, `SIGNIN_CALLBACK_URL`, `OTEL_EXPORTER_OTLP_ENDPOINT`.

**Secrets never in env files:** committed only as `.env.example` placeholders + `external-secrets`/KMS refs in k8s `Secret`; `gitleaks` CI scan blocks accidental commits.

## 16. DOCKER COMPOSE (dev — `infra/docker-compose/dev.yml`)

Services: `localnet` (algorand sandbox), `postgres:16` (vol + init SQL for partition trigger), `redis:7` (2 logical DBs: 0=cache, 1=queues), `nats:2.10` (jetstream enabled), `otel-collector`, `gateway` (build target dev), `controlplane`, `web` (Next dev on :3000), `algod-indexer` (writes to postgres `algod-indexer` db for dev only). Healthchecks: postgres `pg_isready`, redis `PING`, nats `nats server check`. `make dev` = compose up + migrations + seed + LocalNet deploy.

## 17. KUBERNETES MANIFESTS (summary — full YAML lives with infra)

- **Namespaces:** `omniroute`, `omniroute-obs` (otel/prom/grafana/loki/tempo).
- **Gateway Deployment:** replicas 3, HPA (CPU 70%, active-conn custom metric), `topologySpreadConstraints`, `preStop sleep 5`, PDB min 2, resources `req 500m/512Mi, lim 2/2Gi`; `GOMEMLIMIT=1GiB`.
- **Control Plane Deployment:** replicas 2, HPA CPU 70%, PDB 1, `preStop` 30s drain.
- **Workers Deployment:** one Deployment, `command` per container (settlement leader, health, reconcile…), replicas 1 for leader-type, 2 for idempotent types.
- **Services:** ClusterIP gateway:80, controlplane:80 + gRPC 50051; NLB via AWS Load Balancer Controller annotations (targetgroup stickiness off; proxy protocol v2 for client IP).
- **Secrets:** `external-secrets` operator → AWS SecretsManager/SSM refs; `cert-manager` for Gateway client mTLS + origin certs.
- **NetworkPolicies:** deny-by-default; allow: cloudflare-egress → gateway:80; gateway → redis:6379, nats:4222, controlplane:50051; controlplane → rds:5432, redis, nats, algod NLB:4180/8980; workers → rds, nats, algod NLB.

## 18. GITHUB ACTIONS (summary — full YAML lives with `.github/workflows`)

| File | Trigger | Steps |
|---|---|---|
| `ci.yml` | PR | lint → unit → build → trivy → conformance/contract-e2e (path-scoped) → blazer gitleaks |
| `conformance.yml` | PR touching gateway/proto/openapi | replay OpenAI golden matrix |
| `deploy-staging.yml` | push main | build+tag `sha-…` → helm upgrade staging → smoke probe |
| `deploy-prod.yml` | tag `v*` | gate: tests green + SBOM + approval → blue/green gateway, rolling CP, migration job, smoke, release notes |
| `release-sdks.yml` | tag `sdk-*` | build+test → publish PyPI (@npm scope `@omniroute/…`) |
| `nightly.yml` | cron | full e2e (Playwright), load (k6), chaos spot, `npm audit`, dependency updates PR |

## 19. DOCUMENTATION STRUCTURE

```
docs/
├── architecture.md (EDD)                    # high-level
├── implementation/Part1_backend.md           # this spec part 1
├── implementation/Part2_frontend_ops.md      # this spec part 2
├── adr/ADR-001.. (append-only, dated)        # decision log
├── runbooks/001-redis-down.md, 002-settle-fail.md,
│   003-key-compromise.md, 004-mainnet-drill.md  # P1 playbooks
└── api/openapi.md + examples/                # generated reference
```

Rule: docs change in the same PR as code (lint: `markdownlint`). Bump ADR on any architecture deviation.

## 20. CODING STANDARDS & CONVENTIONS

- **Naming:** Go — idiomatic (exported types, `ProviderService`, `WithTx`); TS — PascalCase components/types, camelCase functions, kebab-case files; SQL — snake_case, prefixed indexes `idx_<table>_<cols>`; K8s — kebab-case resources, `omniroute-{app}` labels; branches/commits — Conventional Commits scope from §14.
- **Error conventions:** errors typed at boundary (EDD §25); `err != nil` wrapped once with `%w`; no string errors for control flow.
- **Context propagation:** all repo methods take `context.Context` as first arg; never stored beyond request scope.
- **Concurrency:** goroutines only via controlled workers (use `errgroup`); every goroutine has a panic boundary (log stack → close channel); no `init()` side effects.
- **Linting/naming gates:** `golangci-lint` (gosec, errcheck, revive), `eslint` (react-hooks, TS strict), `prettier`, `markdownlint` — all PR-required.
- **Testing style:** table-driven Go tests; `vitest` colocated `*.test.ts(x)`; Playwright spec naming `*.spec.ts`; no `t.Skip` in prod CI except known-flaky with `// flaky:` comment + issue link.

## 21. COMMIT, BRANCH, VERSIONING, RELEASE STRATEGY

### 21.1 Commit Conventions (Conventional Commits + enforce via commitlint)
```
feat(gateway): add SSE state machine parser (#22)
fix(controlplane): settlement batch nonce ordering (#28)
chore(infra): bump nats to 2.10 (#-)
docs(adr): record ADR-12 redis failover (#-)
test(contracts): add escrow dispute path (#-)
```
- Types: `feat|fix|chore|docs|test|refactor|perf|build|ci|revert`.
- Scope: `gateway|controlplane|web|sdk-python|sdk-ts|sdk-provider|contracts|proto|openapi|infra|docs|admin|analytics|settlement`.
- Breaking changes add `!` e.g. `feat!(openapi): v2 channels` → triggers MAJOR bump.
- Reference issue #NN. Squash-merge only.

### 21.2 Versioning
- **Monorepo version:** Calendar-based MVP `2026.08.0`; releases tagged `vYYYY.MM.PATCH`.
- **SDK packages:** SemVer `0.x` pre-1.0; published from `release-sdks.yml` on `sdk-*` tags. Co-release policy: Python + TS SDKs advance in lockstep versions.
- **Contract (TEAL):** versioned by `app_id` migration table `contract_versions` (applied, block, ma) — immutable past deployments; never in-place modify a live escrow app; new app = new version.
- **API version:** URI `v1`; additive changes only; date-pinned headers `X-Marketplace-Version: 2026-06-30`; `Sunset`/`Deprecation` per RFC 8594 (≥12mo support).

### 21.3 API Evolution Strategy
1. Additive (new field/endpoint): allowed; OpenAPI `oasdiff` in CI ensures no breaking delta; SDKs forward-compatible (ignore unknown fields).
2. Minor behavior change: ship under feature flag (`X-Omniroute-Experimental`), sunset flag, then default.
3. Breaking: new `/v2/*`; SDK dual support for ≥12 months; deprecation notice + telemetry on usage before removal.
4. Contracts (`proto`) evolution: additive fields only; `buf breaking` check in CI; gRPC versioned stream names.

### 21.4 Migration Strategy (DB)
- `golang-migrate` + SQL files; applied via `atlas` diff for reviews.
- Expand-contract pattern: (1) add new column/index/tables (old code runs), (2) dual-write + backfill, (3) switch read path, (4) drop old (separate release).
- Partitions: trigger auto-creates next month before midnight (cron + on-demand function).
- Pre-flight: `schema-preview` job in CI renders Postgres EXPLAIN for partition + index sanity; PITR snapshot before prod migration.

### 21.5 Release Strategy
- **Staging:** every `main` (auto). **Beta gate:** after M2 (streaming billing + settlement on LocalNet) + M3 (OpenAI drop-in) — invite-only on Testnet.
- **Prod soft-launch:** tag `v2026.12.0` (target) → deploy-prod (manual approval + soak 24h) → wave rollouts: 5% → 25% → 100% by consumer cohort; freeze windows for settlement batches during rollouts.
- **Hotfix:** `release/vX.Y.Z-hotfix` from tag; CI + manual smoke; squash → cherry-pick to main.
- **Rollback:** revert image tag (Gateway stateless = instant); Control Plane forward-only compensating migration; SDKs: yank on registry + `npm unpublish` deprecated flag only.

## 22. OBSERVABILITY & SECURITY OPERATING RULES

- Every P1 alert maps to exactly one `docs/runbooks/` file (named by metric). On-call rotation 4 engineers, weekly, PagerDuty.
- Prometheus rules (extracts): `gateway_error_ratio > 0.1%` 5m → page; `settlement_batch_failure` → page; `reconcile_delta_micro_usdc > 0` 5m → page; `unsettled_pct_of_tvl > 50%` 15m → page; `redis_latency p99 > 50ms` 10m → warn.
- **Secrets:** never in k8s YAML; `external-secrets` → AWS SecretsManager; rotation weekly (JWT/macaroon root), quarterly (provider secrets); incident-triggered rotation for compromise (runbook 003).
- **Audit:** every admin action + settlement mutation writes `audit_log` (actor, before/after JSONB, IP); immutable (no UPDATE/DELETE grants; retention 7y).
- **Compliance note (privacy):** zero prompt/response persistence; SOC2-aligned logging policy enforced by CI grep + runtime logger guard (`internal/otel/logguard.go`).

## 23. HOW TO BOOTSTRAP (Engineer Day 1)

1. `git clone` monorepo → `make dev` (compose + LocalNet + migrations + seed).
2. `make test` (unit+integration), `make gen` (proto/openapi/TEAL codegen), `make lint`.
3. Read: `docs/implementation/Part1_backend.md` (their service), `docs/adr/`, `docs/runbooks/`.
4. Pick first issue from `Implementation_Roadmap.md` §5 for their track (`track:*` label).
5. Branch per §14.1 → PR per §14.2 → CI gates → merge squash.

— End of Implementation Specification (Parts 1 & 2).
