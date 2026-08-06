# OmniRoute AI Marketplace — Independent Engineering Audit (Pre-Implementation)

**Auditor role:** Principal Staff Engineer (external consultancy, pre-launch)
**Reviewed artifacts (all 10):** Research.md · Research_v2.md · System_Design.md · Architecture.md · API_Specification.md · PaymentEngine.md · Engineering_Design_Document.md · Implementation_Roadmap.md · Implementation_Spec_Backend.md · Implementation_Spec_Frontend_Ops.md
**Scope:** Payments · State channels · Escrow · Algorand · Smart contracts · Database · Redis · NATS/Kafka · Gateway · Streaming/SSE · Routing · Security/Auth/Macaroons · SDKs · Dashboards · Deployment · CI/CD · Observability · Scaling · Cost · Reliability · Testing · Recovery
**Status:** Implementation has NOT started. This is a pre-launch audit.

**Audit method:** Read all 10 documents cover-to-cover. Flagged cross-document contradictions, unreviewed single-source decisions, and design flaws. Every finding below is labeled with severity, real-world failure scenario, impact, recommended fix, and priority. The system was **not** redesigned; all fixes are surgical.

---

## 0. Executive Verdict (TL;DR)

The architecture direction is fundamentally sound: Go gateway + control-plane split, x402 402-flow, Algorand escrow + off-chain state channels with batched on-chain settlement, OpenAI wire compatibility, and a 4-engineer-friendly monorepo are all the right calls.

**However, the document set is not ready to hand to engineers.** There are **major cross-document contradictions** in the most expensive-to-fix areas: *who signs settlements* (client vs. gateway operator), *what a "Macaroon" actually is* (HMAC-minted by server vs. ED25519-signed by client), *which queue* (Kafka vs. Redis Streams vs. NATS), *Gateway framework* (Fiber vs. stdlib), and *fee percentage* (1% vs. 0.2%). The custody model — gateway-operator-signed settlement — contradicts the "zero-trust / non-custodial" claims repeated throughout the docs and constitutes the single largest financial risk in the project. Several API behaviors (idempotency on chat completions, 3-second re-auth grace, `tx_id` naming) are incorrect or misleading as specified.

**Recommendation: CONDITIONAL GO** for Sprint 0 (project scaffolding, contract-first API definition, TEAL spike), **NO-GO for any production/mainnet work** until the 8 "Blocking" issues in section 10 are resolved and the specs are unified.

---

## 1. Cross-Document Contradiction Matrix (highest severity first)

| # | Area | Document A says | Document B says | Impact | Resolution |
|---|---|---|---|---|---|
| C1 | **Settlement signer** | PaymentEngine §7: "smart contract validates the **client's signature**"; System_Design §4: same | Architecture §6: "The **Gateway signs** this batch using its Operator Key"; `settle_batch` "only callable by the Gateway Operator"; API_Spec `X-Admin-Signature` | **These are opposite custody models.** Client-signed = non-custodial (marketplace cannot steal). Operator-signed = full custody of all escrow funds; one key compromise = total drain. | **CRITICAL.** Final spec must choose. See finding F-01. |
| C2 | **Macaroons** | Research §10, Architecture §3.3, PaymentEngine ADR-001: HMAC root key minted by Gateway | PaymentEngine §5: "Client signs a Macaroon with their ED25519 private key" | Client-signed ED25519 "macaroons" are not macaroons (macaroons are HMAC-based). Security reasoning built on wrong primitive. SDK + contract verification logic ambiguous. | Final spec renamed to server-signed session ticket + client-signed state updates. Must be written into *all* docs. See F-02. |
| C3 | **Settlement queue** | Architecture §5.2: **Kafka** | System_Design §7: **Redis Streams**; PaymentEngine §7: Redis Streams | 3 competing queue designs; 4-engineer team cannot converge. | Final spec chose NATS + Redis-Streams fallback. See F-03 (still over-abstracted for MVP). |
| C4 | **Gateway framework** | Architecture ADR-2: "standard library `httputil.ReverseProxy`" | Architecture container diagram + `Implementation_Spec_Backend.md`: **Fiber** | Direct contradiction *inside Architecture.md*. Fiber adds its own middleware/fasthttp surface to a problem stdlib solves natively. | See F-04. |
| C5 | **Escrow contract API** | PaymentEngine §8: `deposit/settle/initiate_closure/finalize_closure`, box = `[locked, spent, nonce]`, "Admin can upgrade" | Architecture §6: `deposit/withdraw/settle_batch(merkle_root, allocations)`, "only Gateway Operator", contract holds balances | Not the same contract. On-chain funds at risk if wrong shape is built. Also "Admin can upgrade" contradicts spec §21.2 "immutable past deployments". | SEE F-05. |
| C6 | **Fee rate** | Architecture §6: 1% marketplace fee | API_Spec governance example: "lower from 20bps to 15bps" | Business terms ambiguous → contract `fee_percentage` param + payout math wrong. | Pick one; make it contract-configurable with governance constraint. |
| C7 | **Re-auth grace** | API_Spec: `grace_period_ms = 3000` (3 s) | Final backend spec: 300 s TTL | 3 s is impossible for wallet signing/on-chain top-up; legitimate streams die; billing truncation disputes. | Adopt 300 s; update API_Spec. |
| C8 | **Latency SLA** | Architecture NFR: gateway overhead < 50 ms; API_Spec SLA: < 2 ms P50 / < 10 ms P99 / 20 ms max | System_Design: < 5 ms budget; PaymentEngine: < 10 ms target, < 3 ms p99 macaroon verify | Unachievable P50 < 2 ms with Redis SETNX in hot path cross-AZ. Contradictory acceptance criteria. | Unify: 5 ms P50 / 15 ms P99 at gateway (excluding Redis network in same AZ). See F-13. |
| C9 | **Consumer billing basis** | PaymentEngine §6: bill by Gateway byte-heuristic; usage block "for the final financial settlement" | System_Design §16: reject trusting usage block; count on the fly | Provider-injected usage block trusted for billing = inflation attack. | Gateway-measured bytes/tokens = billing truth. See F-07. |
| C10 | **Refund model** | Architecture §17: non-streaming "locks max_cost … refunds the unspent difference" | PaymentEngine §6: "refunds are irrelevant" | Non-streaming (images/embeddings) needs pre-lock + refund semantics; streaming doesn't. Both are right *for their path*; docs don't separate them. | Spec must separate streaming vs non-streaming billing semantics explicitly. |
| C11 | **Payment Engine language** | Architecture §2.2: **Rust** | Research_v2, System_Design, final: **Go** | Confusion during sprint 0; wasted spike time. | All-Go confirmed; delete Rust reference. |
| C12 | **Registry language** | Architecture §2.2: **Node.js/Express** | Final spec: Go control plane | Same as C11. | All-Go confirmed. |
| C13 | **DB engine** | Architecture deployment: "Amazon Aurora PostgreSQL" | Final spec: RDS PostgreSQL 16 | Aurora ≠ RDS; feature/parameter mismatch (partitions, logical replication). | Standardize on RDS PG16 (final spec). |
| C14 | **Slashing** | API_Spec `/v1/admin/providers/slashing` with `slash_amount_algo` | No staking mechanism anywhere (`providers` table has no stake field) | Can't slash without a lock; governance theater. | Drop slashing for MVP; add staking later. See F-15. |
| C15 | **x402 headers** | Research/Research_v2 describe `WWW-Authenticate: x402` (interop standard) | API_Spec uses proprietary `X-402-Payment-Required` / `X-402-Payment-Proof` | Not interoperable with the emerging x402 standard; SDKs from other providers won't work. | Support standard headers, keep X-402-* as aliases. See F-16. |

---

## 2. Per-Subsystem Audit

Legend: **S = Severity** (Critical/High/Medium/Low) · **P = Priority** (P0 fix before code, P1 before beta, P2 backlog).

### 2.1 Payment Engine & Settlement

**F-01 — Settlement signed by Gateway Operator = full custody risk. (S: Critical, P: P0)**
- **Problem:** Architecture §6 and final backend spec authorize the gateway operator key to submit `settle_batch`/escrow updates; the contract then pays providers. Multiple documents claim "zero-trust" and "the platform does not hold user balances" (PaymentEngine §Executive summary). Both cannot be true: an operator-signed settlement makes the marketplace custodian of *every escrowed USDC*.
- **Real-world failure:** Compromise of one HSM/KMS key (or rogue insider, or supply-chain compromise of the signer service) drains all open escrows in one settlement batch. No consumer signature required. Multi-sig admin freeze (added in final spec) helps on governance actions but not on the settlement key itself.
- **Impact:** Total funds loss; catastrophic reputational and regulatory exposure (see F-21).
- **Fix:** Settlement must require the **consumer-signed cumulative state proof** (nonce + total spend + provider allocation) as the authority to move funds. Gateway role = verify, aggregate, and submit within the consumer's signed budget. Gateway operator key signs *only* the batch envelope (anti-tamper), never authorizes spend. This matches PaymentEngine §7 and System_Design §4 — those documents were right; Architecture §6 was wrong.
- **Tradeoff acknowledged:** consumer must sign state updates per request/stream (SDK already has the key wired for this — API_Spec's Ed25519 proof of challenge); settlement cannot happen without consumer co-operation, which is exactly why escrow timelock + force-close exists (`finalize_closure` after 24 h timelock).

**F-02 — "Macaroon" is used to mean two different primitives. (S: High, P: P0)**
- **Problem:** Research/Architecture describe HMAC-root-key macaroons minted by the gateway (true macaroons, caveat attenuation). PaymentEngine §5 describes client-signed ED25519 "macaroons" (these are just signed payloads — no attenuation property, and standard macaroon libraries do not apply).
- **Real-world failure:** Engineer implements `macaroon-go` with server HMAC, SDK implements client Ed25519; verification fails at integration; two weeks lost; worse, someone "fixes" it by trusting one side's signature without the other's.
- **Impact:** Crypto confusion → misspent security, integration delays.
- **Fix:** Final spec already renamed these: **gateway-issued session ticket** (server-signed, contains channel_id, consumer_pubkey, expiry, max C_max; serves as the "402→authorized" handoff) + **consumer-signed state update** (per-request proof of budget/`C_max` authorization). Propagate this vocabulary into SDK docs, contract method docs, and API error messages. Remove the word "macaroon" from the public API; call the proof header what it is.

**F-03 — Three queue designs (Kafka vs Redis Streams vs NATS). (S: Medium, P: P1)**
- **Problem:** Architecture says Kafka; System_Design/PaymentEngine say Redis Streams; final spec introduces NATS JetStream with a `Queue` abstraction and Redis Streams fallback.
- **Real-world failure:** A 4-engineer team maintains two queue implementations and an abstraction layer for a queue they can't yet load-test. Every bug gets double-fixed. Ops burden (NATS cluster + Redis) on day 1.
- **Fix:** Pick **NATS JetStream** only (durable, replay, ordered, cheap ops, works for receipts + events + webhooks fan-out). Keep Redis **out** of the message-bus role in prod; Redis Streams remain the *dev-only* fallback behind the `Queue` interface, deleted once NATS is proven. Kafka never ships in MVP. Delete "Kafka (optional)" from stack list.

**F-04 — Fiber as the gateway HTTP framework. (S: Medium, P: P1)**
- **Problem:** Architecture ADR-2 explicitly chose stdlib `httputil.ReverseProxy`; the final backend spec mandates Fiber. Fiber (fasthttp) has its own request lifecycle, different `http.Handler` behaviors, and a history of CVEs in fasthttp's parsing surface; it gives nothing for this workload that stdlib `net/http` + `httputil.ReverseProxy` doesn't (goroutine-per-conn, streaming, hijacking, `ResponseController`).
- **Real-world failure:** Proxy edge cases (100-continue, trailers, SSE flush semantics, upgrade) behave differently on fasthttp; conformance replay tests burn a sprint aligning them.
- **Fix:** Gateway uses stdlib `net/http` + `httputil.ReverseProxy` with a hand-rolled `ModifyResponse`/`ResponseController` for SSE. Fiber was acceptable for the control-plane REST API; keep it there if desired. Update ADR-2 and the backend spec to match.

### 2.2 State Channels & Escrow

**F-05 — Escrow contract shape is not singular in the docs. (S: Critical, P: P0)**
- **Problem:** PaymentEngine (box `[locked, spent, nonce]`, 4 methods), Architecture (`deposit/withdraw/settle_batch(merkle_root, allocations)`), final spec (box `[locked, settled, latest_nonce, root_hash]`) are three different contracts. A naive reader cannot tell what to build. Merkle-root batch settlement, if kept, requires on-chain per-claim Merkle proofs — expensive and complex in TEAL, and it re-introduces operator-controlled allocation (see F-01).
- **Real-world failure:** Team builds the wrong contract; USDC funds become unrecoverable in mainnet; or contract ships with operator-only settle → theft vector.
- **Fix:** Nominate **one** contract in the backend spec: per-channel box, `escrow_open/deposit/reauth/finalize/disclose_settlement/initiate_closure/finalize_closure`, settlement authorized by **consumer-signed latest state** (see F-01). Drop Merkle `settle_batch` for MVP. Freeze this in an ADR and require the smart-contract engineer to update every referencing doc before writing code.

**F-06 — Contract "Admin can upgrade" vs "immutable past deployments". (S: High, P: P0)**
- **Problem:** PaymentEngine §8 lists Admin "can upgrade contract"; final spec §21.2 correctly says no in-place upgrade, new app = new version.
- **Real-world failure:** An "upgradeable" contract is developed with delegate-style indirection; audit cost doubles; upgrade key becomes a prize target; or a migration path is skipped because "upgradeable" was assumed.
- **Fix:** Immutable-for-MVP. Publish upgrade path = new app id + migration worker + consumer migration flow (open new escrow, force-close old). Record in ADR.

**F-07 — Consumer billed by provider's `usage` block (up to 1.3× heuristic) is an over-charge risk. (S: High, P: P1)**
- **Problem:** The "provider usage inflation capped at min(GW, 1.3×heuristic)" rule means: gateway measured 100 tokens, provider claims 10×, consumer is billed 130 tokens. Billing consumer on *any* function of provider-reported numbers is wrong.
- **Real-world failure:** Anecdote: provider bug doubles `completion_tokens`; every consumer overbilled 30% on those streams; support tsunami; trust destroyed; the "1.3×" looks arbitrary and is indefensible in a dispute.
- **Fix:** Consumer bill = **gateway-measured** bytes→tokens ONLY. Provider-reported usage is used solely for (a) anomaly detection and (b) provider-side payout reconciliation. Never for consumer billing. Update the named test `provider_usage_inflation_capped` to assert consumer billing is unaffected.

**F-08 — Byte→token heuristic (1 token ≈ 4 bytes) under/over-charges systematically. (S: Medium, P: P1)**
- **Problem:** Real ratio varies 3.2–6+ bytes/token (code, CJK, emoji). A flat 4.0 is wrong by ±25–50% per stream. For $/1k-token pricing, that's material drift.
- **Fix options (pick one, document it):** (a) run a real tokenizer (tiktoken-compatible) off the hot path in the gateway for final billing (CPU cost absorbed by existing goroutine; ~µs-ms per stream) — best accuracy; (b) price per byte and stop claiming "per-token" pricing; (c) keep 4:1 but publish it as the billing unit ("tokens ≈ bytes/4") and make disputes resolvable by L1-verifiable byte counts. Recommend (a) at P1.

**F-09 — Off-chain balance vs on-chain locked balance UX trap. (S: Medium, P: P1)**
- **Problem:** After each stream, consumers see `channel_remaining_balance` decremented off-chain, while the on-chain escrow still shows the original deposit until settlement.
- **Real-world failure:** Consumer sees "10 USDC balance" on-chain explorer and attempts a withdrawal of the full amount; settlement/maths conflict; dispute; refund churn.
- **Fix:** Dashboards/API must expose three distinct numbers: `on_chain_locked`, `off_chain_spent`, `withdrawable_now`, and explain the settlement lag. API_Spec's `x402_settlement` field should carry all three.

### 2.3 Algorand & Smart Contract Integration

**F-10 — TEAL v8 pin is outdated. (S: Low, P: P1)**
- **Problem:** AVM is at v10/v11; v8 lacks newer opcodes (e.g., `ec_add`/pairing — not needed), but more importantly hand-writing raw TEAL is risky.
- **Fix:** Use **TealScript** or **AlgoKit Python templates** targeting latest stable AVM; generated TEAL reviewed. Pin the AVM version in CI.

**F-11 — External security audit missing from roadmap. (S: High, P: P0)**
- **Problem:** The roadmap has contract tests but no independent audit gate before mainnet.
- **Real-world failure:** A single exploitable TEAL bug with escrow funds = unrecoverable loss (immutable). This is precisely the class of bug audits exist for.
- **Fix:** Add a hard gate: **external smart-contract audit (e.g., Trail of Bits/Quantstamp/Solidified) before any mainnet lock**. Budget + schedule it in the roadmap; publish the audit report.

**F-12 — Mainnet rollout in Q4 2026 is aggressive. (S: Medium, P: P1)**
- **Problem:** Roadmap targets mainnet ~Q4 2026 with 4 engineers through: contract audit, settlement engine, dashboards, SDKs, conformance, chaos, K8s multi-region.
- **Fix:** Slip mainnet to earliest Q1–Q2 2027; keep Testnet beta in Q4 2026 with invited providers/consumers. Freeze scope aggressively (drop governance, slashing, multi-region — single region first).

### 2.4 Database

**F-13 — Single global RDS primary across "multi-region active-active" is a scalability bottleneck and a contradiction. (S: High, P: P1)**
- **Problem:** System_Design §13 says regions are independent silos sharing only Postgres + Algorand; but every region's settlement receipts/journal rows land in one RDS primary. That's neither active-active nor scalable: cross-region write latency, hot partition on one writer, and a global single point of failure.
- **Real-world failure:** API volume grows; settlement journal writes from 3 regions contend; replication lag to reporters/analytics grows; PITR/backups get slow; one DB incident takes down all dashboards and settlement.
- **Fix (MVP):** Single-region deployment (us-east-1) with multi-AZ RDS. Multi-region later = regional RDS with L1 as cross-region source of truth + async consolidation of analytics. Document that "active-active" refers to the stateless gateway layer only.

**F-14 — Missing/under-specified DB indexes and retention. (S: Medium, P: P1)**
- **Problem:** Some hot query paths lack explicit indexes in the docs (e.g., `settlement_logs(provider_id, timestamp)`, `settlement_logs(channel_id, nonce)` UNIQUE proxy for replay defense, `webhook_deliveries(status, next_attempt_at)`).
- **Fix:** Add to the spec: unique index on `(channel_id, nonce)` for the receipt journal; composite indexes for provider/consumer dashboards; `pg_trgm` GIN on models name/description for marketplace search (Architecture §10 assumes it); retention/partition policy already present — add per-table notes for `audit_log` (7y), `settlement_logs` (36 mo hot, archive after).

### 2.5 Redis

**F-15 — `volatile-lru` eviction on the state instance is catastrophic if implemented as written in System_Design. (S: Critical, P: P0)**
- **Problem:** System_Design §5 says Redis owns nonces, *channel balances*, and the settlement queue with `volatile-lru`. Under memory pressure, **channel balance/lock keys and undispatched receipts get evicted** → overspend enforcement collapses mid-stream, receipts vanish.
- **Fix:** The final spec already separates two logical Redis instances. Mandate: **instance B (state/queues) uses `maxmemory-policy noeviction` (or `volatile-ttl` with ALL financial keys explicitly TTL'd and pinned)**, while instance A (cache) stays LRU. Add a load test that fills memory and asserts noeviction on B. Also: never store authorizing state in an evictable KV; the Postgres journal is the backstop (already specified) — keep it.

**F-16 — Redis in the auth hot path breaks the 2 ms P50 SLA and creates a regional SPOF. (S: Medium, P: P1)**
- **Problem:** Every request does `SETNX` + channel-lock read in Redis synchronously. Cross-AZ Redis adds ~1–3 ms; Redis outage (even with fail-closed) = marketplace outage.
- **Fix:** (a) Same-AZ deployment of Redis for the gateway, with replica failover (documented RTO < 5 s); (b) accept gateway overhead SLA 5 ms P50 / 15 ms P99; (c) optional local positive-cache of recently-validated nonces (`TTL` few seconds) to shave the p99 tail, never bypassing the SETNX for *accepts*.

### 2.6 Gateway, Streaming & SSE

**F-17 — SSE slow-consumer kill at 5 s is too aggressive. (S: Medium, P: P1)**
- **Problem:** System_Design §2 kills the stream if the socket buffer stays full 5 s. Mobile agents on lossy networks routinely stall > 5 s; every such kill bills the consumer for partial output they may not have received (client-side) and generates disputes.
- **Fix:** Make the read-idle timeout configurable (default 60 s for streams), apply throttling of the *upstream* instead of killing (already possible: pause provider reads), and only terminate at the hard cap. Bill only up to the last byte actually transmitted to the client socket driver (add "bytes_acknowledged" semantics).

**F-18 — SSE heartbeat missing → proxies/LB kill long generations. (S: Medium, P: P1)**
- **Problem:** API_Spec sample stream has no keep-alive comments. Cloudflare/ALB idle timeout (60–100 s) will sever streams for long generations.
- **Fix:** Emit `: ping` keep-alive every 15 s on the gateway→client SSE path (and forwarding idle heartbeats from provider if present). Include in conformance golden tests.

**F-19 — SSE parser edge cases under-specified. (S: Medium, P: P1)**
- **Problem:** Multi-line `data:` fields, `event:` types, `id:`/`retry:` fields, `data: [DONE]` with trailing whitespace, and chunked UTF-8 split across TCP segments are not all covered.
- **Fix:** Add fuzz + property tests for the SSE state machine (Go `fuzz` target on the parser) and include an event-type whitelist + size cap on a single event (spec already has 1 MiB cap — good).

**F-20 — Provider token-rate heuristic (e.g., >200 tok/s) is model-blind. (S: Low, P: P2)**
- **Problem:** A 70B on 8×H100 can legitimately exceed fixed ceilings; a small quantized model on 1 GPU can't be attacked at 200 tok/s anyway. False positives hurt legit providers.
- **Fix:** Per-model ceilings from registry metadata; anomaly flagging (non-blocking) rather than hard kill; telemetry-reviewed weekly.

### 2.7 API & x402

**F-21 — Idempotency-Key on `/v1/chat/completions` with 24 h response cache is wrong for LLMs. (S: High, P: P0)**
- **Problem:** API_Spec mandates idempotency caching; replaying an identical *cached LLM response* violates the semantics of generation (each call should produce a (possibly) new completion) and OpenAI compatibility. OpenAI does not support idempotency on completions.
- **Real-world failure:** A client retries with the same key after a network blip and gets a stale/identical answer; or different clients share keys and receive cached private-ish generations.
- **Fix:** Idempotency applies ONLY to financial/state mutations (escrow open, deposits, reauth, admin ops). Chat/embeddings/image calls: idempotent at the *payment-nonce* level (the same proof cannot be re-spent — that already prevents double billing), never response-cached.

**F-22 — `x402_settlement.tx_id = "STATE_CH_UPD_..."` misnames an off-chain reference. (S: Low, P: P2)**
- **Fix:** Rename to `state_update_id` to avoid confusion with on-chain transaction IDs; document in OpenAPI.

**F-23 — x402 header names diverge from the standard. (S: Medium, P: P1)**
- **Fix:** Support `WWW-Authenticate: x402 <base64>` on the 402 and `Authorization: x402 <proof>` on the retry (interop with ecosystem SDKs); keep `X-402-Payment-Required/Proof/Response` as aliases during transition. Update examples/conformance tests to both forms.

**F-24 — `X-Admin-Signature` on batch settlement endpoint is an ambiguous trust anchor. (S: High, P: P0)**
- **Problem:** API_Spec lets a single admin ed25519 signature authorize `channel_closures[].final_provider_payouts`. Combined with operator-signed settlement this creates two keys that can move funds.
- **Fix:** Admin API can *create* a settlement proposal; the **signer service** (holding the KMS key) verifies: consumer state-proof signatures (F-01), channel openness, fee math — then signs. Admin signature alone moves nothing. 2-of-3 admin multi-sig for cancelling/refunding (already in final spec) stays.

### 2.8 Security & Trust

**F-25 — "Zero-trust/non-custodial" claims vs. actual custody model. (S: Critical, P: P0)** — see F-01/F-24. Consolidates: the narrative must change to "bounded-custody with consumer-signed authority and timelocks." Regulatory exposure follows (see F-21 below → renumber): custody of third-party funds likely triggers **money-transmission/ VASP licensing** in several jurisdictions. Engage counsel in Sprint 0; the escrow design (consumer-signed settlement + timelock close) materially reduces this exposure vs. operator-signed settlement.

**F-26 — Hardware attestation field is unverifiable theater. (S: Medium, P: P1)**
- **Problem:** `hardware_attestation: tee_type: SEV-SNP` is self-reported; no remote attestation flow exists. Displaying "attested/verified" badges would be false advertising.
- **Fix:** Remove from MVP UI; keep the field but label it `self_reported`; implement remote attestation (Nitro/SEV-SNP) only as a later trust feature.

**F-27 — Slashing without staking. (S: High, P: P1)** — see C14/F-15: remove slashing endpoint + governance for MVP; replace with reputation + dispute + admin freeze/refund (already in final spec). Governance/slashing is a protocol-level feature, not an MVP feature.

**F-28 — Nonce replay TTL should exceed the dispute window. (S: Medium, P: P1)**
- **Problem:** Short TTL (60 s per source docs) on nonce keys means an old proof can be replayed after expiry. The Postgres journal catches it at settlement (good backstop), but the *gateway accept path* won't.
- **Fix:** Redis nonce TTL = 7 days (or until channel close), plus the DB unique index (F-14) as final authority. Cost is trivial.

### 2.9 NATS / Reliability / Ops

**F-29 — Single-leader settlement with Redis lock has a failover gap. (S: Medium, P: P1)**
- **Problem:** If the leader dies after acquiring the lock, settlement stalls until lock TTL expiry (must be short: 30 s with heartbeat renewal). Also two workers must never build the same channel's batch concurrently — document ordered lock acquisition (hash of channel_id) to avoid deadlock between reconciliation and settlement (lock-ordering rule).
- **Fix:** Lock TTL 30 s + renewal; per-channel serialized processing via NATS consumer with manual ack (not just a global mutex).

**F-30 — Provider payout requires USDC opt-in and ALGO gas for signer — not enforced in docs. (S: Medium, P: P1)**
- **Problem:** A provider that hasn't opted into USDC breaks every batch that includes them (group txn fails atomically → whole batch to DLQ).
- **Fix:** Enforce USDC opt-in at registration (address balance check via indexer); batch builder excludes non-opted providers with a loud alert; signer holds gas ALGO with a low-balance alert (runbook).

**F-31 — Gateway crash mid-stream: consumer-not-billed policy is fine, but throughput of receipts must be explicit. (S: Low, P: P2)**
- **Fix:** Keep "provider eats cost" as documented policy; mitigate by flushing receipts on a 100 ms/1000-count cadence (already spec'd) and by sending provider RST on *all* exit paths. Add chaos test: kill gateway mid-stream; assert provider gets RST and receipt flush ≤ 200 ms.

### 2.10 SDKs & Dashboards

**F-32 — SDK `autoFundChannels: true` requires the agent to own ALGO (gas) + USDC and to have opted in. (S: Medium, P: P1)**
- **Fix:** SDK must expose an explicit `ensureWalletReady()` flow (gas check + opt-in + fund) with clear error messages; document fiat on-ramp as out-of-scope-but-planned (Stripe/Coinbase Pay) — already flagged in Research. Do not silently auto-sign on-chain funding in a headless agent without an explicit approve-list + spend caps (`maxAutoApproveMicroUsdc` is good; add per-day cap).

**F-33 — Dashboard "burn rate" SSE hook + React Query invalidation bounded (spec'd) — no issue.** *(No action.)*

**F-34 — Provider analytics queries must hit rollup tables, not live `settlement_logs` aggregates. (S: Medium, P: P1)**
- **Fix:** Add materialized rollups (hourly/day) refreshed by worker; enforce in code review that dashboards never query raw logs.

### 2.11 Deployment / CI/CD / Observability

**F-35 — EKS + Terraform + Helm + NATS + KMS + Cloudflare for 4 engineers. (S: Medium, P: P1)**
- **Fix:** MVP on EKS (managed) + managed RDS/ElastiCache is fine, but drop: KEDA (use HPA CPU + simple custom metric via `external.metrics` only if trivial), multi-region, and the separate "workers" deployment complexity — one deployment with sidecar process flags (already suggested) is fine. Architecture.md's `ALB → RA` (direct Registry exposure) must be closed: registry/control-plane is private, only the gateway should be public.

**F-36 — Conformance gating is genuinely good; add SSE fuzz + Redis-memory-pressure chaos to the matrix. (S: Low, P: P1)** (see F-15/F-19.)

**F-37 — Alert mapping to runbooks is good; missing alert: `settlement_signer_balance_algo < threshold`. (S: Low, P: P2)** — add.

**F-38 — `GOGC=200` + `GOMEMLIMIT` — fine. `ulimit -n = 1M` — fine on Linux; note EKS AMI defaults must be raised. (S: Low, P: P2)**

**F-39 — Cost estimate (prod soft-launch, single region):** EKS ~$80/mo ctrl + 3× m6i.large gateway ~$180 + 2× m6i.large CP ~$120 + RDS PG16 (db.r6g.large multi-AZ) ~$500 + ElastiCache (r6g.large×3) ~$400 + NATS (small nodes) ~$120 + Algorand archival/indexer node (2× m6i.2xlarge + 500 GB gp3) ~$900 + Cloudflare Pro ~$20 + misc ≈ **$2.5–3.5k/mo**. Acceptable for a funded startup; ballpark < 5% of a production AI-marketplace budget. Add an explicit infra-cost section to the EDD with right-sizing notes (archival node storage is the fastest-growing line).

---

## 3. Scores

| Metric | Score /10 | Rationale |
|---|---|---|
| **Engineering** | **7.0** | Strong decomposition (data/control plane, service boundaries, monorepo, schema-first). Penalized by cross-doc contradictions (C1–C15) and over-abstraction (dual queue, macaroon ambiguity). |
| **Production Readiness** | **5.0** | Not ready: custody/signing model unresolved, no contract audit gate, SLA contradictions, multi-region claims unimplementable as written. Testnet-beta ready after P0 fixes. |
| **Maintainability** | **7.5** | Clean monorepo, ADR discipline, typed errors, logging guard, applyable spec. Penalized by 3 queue options + fiber/stdlib contradiction + self-reported attestation theater. |
| **Security** | **5.5** | Strong threat-model thinking (double-spend, replay, provider milking, slow-client). Penalized by operator-signed custody model, admin-signature fund-movement, unverifiable attestation, nonce TTL gaps. |
| **Scalability** | **7.0** | Gateway scales cleanly (stateless, 10k streams/node). Penalized by: global RDS primary vs "active-active", Redis hot-path SPOF, and per-request SETNX tail latency. Single-region MVP fine. |
| **Developer Experience** | **8.0** | Best-in-class: OpenAI wire compat, 402-interceptor SDKs, self-healing retries, conformance golden tests, clear docs. Penalized by wallet/gas/opt-in friction that is unavoidable pre-on-ramp. |
| **Cost Efficiency** | **7.0** | Right-sizeable to $2.5–3.5k/mo. Penalized by NATS+Redis dual stack, archival node overhead, and EKS complexity for a 4-person team (managed alternatives cheaper to run, not to rent). |

**Overall weighted (Engineering 25 / Security 25 / Prod 20 / Scal 10 / DX 10 / Cost 10): 6.6/10 — "Qualified Advance".**

---

## 4. Risk Matrix

| # | Risk | Likelihood | Impact | Mitigation (in docs/fixes) | Residual |
|---|---|---|---|---|---|
| R1 | Escrow theft via signing/custody flaw (F-01/F-24) | Med | **Catastrophic** | Consumer-signed settlement; HSM/KMS; multi-sig admin; external audit gate | Low after fix |
| R2 | Contract exploit on immutable mainnet (F-11) | Med | Catastrophic | External audit + Testnet soak + invariant tests | Med |
| R3 | Settlement stall / batch DLQ pile-up (F-29/F-30) | Med | High (liquidity freeze) | Leader lock+renewal, per-channel serial, opt-in enforcement, alerts | Low |
| R4 | Redis eviction of financial state (F-15) | Low-Med | High | noeviction instance + Postgres journal | Low |
| R5 | Cross-region scaling claim vs single-primary DB (F-13) | High (if pursued as designed) | High | Single-region MVP; regional DB plan later | Med |
| R6 | Regulatory/custody classification (F-25) | Med | High | Bounded-custody design + counsel in S0 | Med |
| R7 | Billing disputes from heuristic token counting (F-07/F-08) | High | Med | Gateway-measured billing + real tokenizer | Low |
| R8 | Slow-consumer kills → CX churn (F-17) | Med | Med | Configurable idle timeout + throttle upstream | Low |
| R9 | SDK wallet friction blocks M2M adoption (F-32) | High | Med | Autofund guardrails + ready-check + on-ramp roadmap | Med |
| R10 | Mainnet date slippage (F-12) | High | Med (schedule) | Phase plan w/ buffer; freeze scope | Med |

---

## 5. Top 25 Issues to Fix Before Implementation (ordered)

| # | ID | Severity | Priority | Issue | Action |
|---|---|---|---|---|---|
| 1 | F-01 / C1 | Critical | P0 | Settlement signer ambiguity (client-signed vs operator) | Decide & ADR: consumer-signed cumulative state; operator signs envelope only |
| 2 | F-05 / C5 | Critical | P0 | Three different escrow contract shapes in docs | Single contract spec + ADR + freeze |
| 3 | F-25 | Critical | P0 | "Non-custodial" claim false under operator-signed model | Rewrite custody narrative; bounded-custody design |
| 4 | F-24 | High | P0 | Single admin signature can authorize fund movement | Admin=proposal; KMS signer=execution; 2-of-3 multi-sig |
| 5 | F-02 / C2 | High | P0 | "Macaroon" = 2 different primitives | Rename: gateway session ticket + consumer state update |
| 6 | F-11 | High | P0 | No external contract audit gate before mainnet | Add audit milestone + budget |
| 7 | F-06 / C5 | High | P0 | "Admin can upgrade" vs immutable contract | Immutable MVP; new-app migration path |
| 8 | F-21 | High | P0 | Idempotency cache on chat completions returns stale responses | Restrict idempotency to financial mutations only |
| 9 | F-15 | Critical | P0 | `volatile-lru` on financial Redis state | noeviction state instance + journal backstop + load test |
| 10 | F-07 / C9 | High | P1 | Consumer billed using 1.3× provider-usage function | Billing = gateway-measured only |
| 11 | F-26 / C14 / F-27 | High | P1 | Slashing without staking; attestation theater | Remove slashing + attestation badges; reputation+disputes only |
| 12 | F-28 | Medium | P1 | Nonce TTL shorter than dispute window | 7-day TTL + DB unique index `(channel_id, nonce)` |
| 13 | F-13 | High | P1 | Global RDS primary ≠ active-active multi-region | Single-region MVP; ADR for regional DB evolution |
| 14 | F-14 | Medium | P1 | Missing indexes / retention gaps | Add index list (see §2.4) to backend spec |
| 15 | F-03 / C3 | Medium | P1 | Kafka + Redis Streams + NATS triple design | NATS-only; delete Kafka from MVP |
| 16 | F-04 / C4 | Medium | P1 | Fiber vs stdlib contradiction for gateway | Stdlib `net/http` + `httputil.ReverseProxy`; update ADR-2 |
| 17 | F-23 / C15 | Medium | P1 | Proprietary x402 headers vs standard | Standard `WWW-Authenticate`/`Authorization` + aliases |
| 18 | F-16 / C8 | Medium | P1 | Unachievable 2 ms P50 SLA with Redis in path | 5 ms P50 / 15 ms P99 SLA; same-AZ Redis |
| 19 | F-17 | Medium | P1 | 5 s slow-consumer kill | 60 s default idle; throttle upstream; bill to last acked byte |
| 20 | F-18 | Medium | P1 | No SSE heartbeats | 15 s `: ping`; conformance test |
| 21 | F-19 | Medium | P1 | SSE parser edge case coverage | Fuzz + property tests; event whitelist |
| 22 | F-08 | Medium | P1 | 4 bytes/token heuristic drift | Real tokenizer off hot path for billing (preferred) |
| 23 | F-30 | Medium | P1 | Provider USDC opt-in not enforced → batch fail | Enforce opt-in at registration; signer gas alerts |
| 24 | F-29 | Medium | P1 | Settlement leader lock failover gap | 30 s TTL + renewal; per-channel serial consumers |
| 25 | F-35 / F-39 | Medium | P1 | Ops/complexity & cost for 4 engineers | Trim KEDA/multi-region; single-region; budget infra in EDD |

*Also see C6 (fee rate), C7/C10 (3 s grace, refund model), C11–C13 (language/DB consistency), C15 — all must be reconciled in the docs during Sprint 0 as part of the "Spec Unification" epic.*

---

## 6. Go / No-Go Recommendation

**SPRINT 0 (Project Setup & Architecture Hardening): GO** — zero production code.
1. Resolve the 8 P0 items above into an ADR set (signing model, contract shape, macaroon vocabulary, idempotency scoping, Redis noeviction, SLA, single-region, audit gate).
2. Reconcile all 10 documents (delete Kafka, lock Go-only, reconcile fees/grace/SLA).
3. Contract-first: publish OpenAPI + proto + TEAL method stubs; run spectral/oasdiff/buf lint.
4. Spin up LocalNet; spike: consumer-signed cumulative state settlement (F-01) — prove the smart-contract math with a 5-page spike report BEFORE committing to the contract ABI.

**PRODUCTION / MAINNET: NO-GO until all of the following are true:**
- [ ] P0 items 1–9 closed and cross-referenced in every doc
- [ ] External smart-contract audit passed, published, with no findings > Medium
- [ ] Bounded-custody model reviewed by counsel (licensing posture documented)
- [ ] Testnet beta (invite-only) with real USDC on Testnet for ≥ 6 weeks incl. chaos drills
- [ ] Conformance suite green against OpenAI golden set (stream/non-stream/errors)
- [ ] Named critical tests from the backend spec all passing in CI (esp. `redis_flush_replay_still_blocked`, `settlement_batch_atomic_failure`, `double_spend_replay_rejected`)
- [ ] Reconciliation worker matched L1 vs Postgres for ≥ 4 weeks with zero unexplained delta

**Estimated cost of the audit fixes:** ~1 sprint (2 weeks) of spec/ADR work for 2 senior engineers + 1 external audit engagement (3–6 weeks, runs in parallel with implementation). This is the cheapest insurance available in this project.

---

*End of audit. The architecture is strong, the team is right-sized, the unit economics work. Fix the signing/custody model, unify the specs, gate mainnet behind an audit, and this is a genuinely shippable production system.*