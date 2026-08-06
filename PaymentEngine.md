# Payment Engine

## Executive Summary

The primary objective of the Payment Engine is to process machine-to-machine (M2M) micro-transactions for AI inference services at sub-10ms latency.

The scope of this document covers the internal financial architecture of the platform: escrow management, off-chain state channels, authorization flows, and on-chain batch settlement. The engine bridges the asynchronous, high-latency environment of a Layer 1 blockchain with the synchronous, extremely low-latency requirements of streaming HTTP API proxies.

Out-of-scope for this document: Gateway routing logic, model execution, and TLS termination details.

The core design philosophy is **"Zero-Trust Off-Chain State with On-Chain Finality."** The platform does not hold user balances in a centralized database ledger; all financial state is derived from cryptographic proofs (Macaroons) backed by Layer 1 smart contract escrows.

---

## 1. Payment Philosophy

**Why x402?**
We utilize HTTP 402 Payment Required headers to trigger automated payment flows.

* *Alternative considered:* API keys. Rejected because API keys require out-of-band trust, pre-funded balances on a centralized ledger, and human-in-the-loop credit card billing. x402 enables autonomous agents to discover an endpoint and negotiate payment programmatically.

**Why Escrow and State Channels?**
Directly settling every HTTP request on Layer 1 is impossible. Even with Algorand's ~2.8s finality, placing a 3-second delay before the first token of an LLM stream breaks the developer experience.

* *Alternative considered:* Direct Layer 2 rollups. Rejected because moving to a rollup still incurs centralized sequencer trust and latency overhead. Unidirectional state channels (Escrow) allow the Gateway to instantly authorize requests locally at <5ms latency.

**Why Algorand and USDC?**

* ✅ **Verified Fact:** Algorand mainnet block finality is deterministic at ~2.8 seconds. This rapid finality allows for fast escrow funding.
* ✅ **Verified Fact:** USDC exists natively on Algorand (ASA ID 31566704), providing a globally liquid, USD-pegged settlement asset without the volatility of native L1 gas tokens.
* *Alternative considered:* Stripe fiat micropayments. Rejected due to minimum transaction floors ($0.50 + 2.9%) which mathematically prohibit a $0.0001 LLM prompt.

---

## 2. Payment Lifecycle

1. **Wallet Funding:** The consumer funds their Algorand wallet with USDC (ASA 31566704) and ALGO (for gas).
2. **Escrow Creation:** The consumer calls the `create_escrow` smart contract method, depositing a block of USDC (e.g., $10.00) and authorizing a specific Gateway pubkey to submit state updates.
3. **Payment Authorization:** For each API request, the consumer generates a Macaroon off-chain. The Macaroon cryptographically commits to spending a fraction of the escrow (e.g., $0.01).
4. **Usage Accounting:** The Gateway streams the AI response and meters the bytes.
5. **Balance Decrement:** The Gateway accepts the Macaroon. The consumer's remaining usable balance in the channel decreases.
6. **Settlement:** Periodically (e.g., every 1 hour or when 80% depleted), the Gateway submits the *latest, highest-value* Macaroon to the Algorand smart contract.
7. **Withdrawal:** The smart contract validates the Gateway's submission, transfers the USDC to the Provider (minus Marketplace fees), and leaves the remaining escrow balance intact.
8. **Channel Closure:** The consumer can call `close_escrow` to withdraw remaining funds, subject to a timelock to allow the Gateway to submit any final pending receipts.

---

## 3. Escrow System

The Escrow system relies on Algorand Smart Contract **Box Storage** to manage unbounded concurrent channels without bloat.

* **Ownership:** The Smart Contract holds the USDC. Neither the Gateway nor the Consumer has unilateral custody once locked.
* **State:** Each active channel is represented by a Box. The Box key is `sha256(consumer_pubkey || gateway_pubkey)`. The Box value contains `[locked_amount, spent_amount, nonce]`.
* **Deposit Flow:** Consumer groups an Application Call (to create the Box) and an Asset Transfer (to fund the contract) into an Atomic Transfer.
* **Failure Handling:** If the Box creation fails, the Atomic Transfer ensures the USDC never leaves the consumer's wallet.
* 🟡 **Engineering Recommendation:** The Consumer must pay the Minimum Balance Requirement (MBR) in ALGO to create the Box. Upon channel closure, the contract refunds this ALGO. This prevents state-bloat attacks on the network.

---

## 4. Payment Channel Design

Streaming AI fundamentally breaks standard request-response billing. You cannot know the final price of an LLM request until the final token is generated.

**Why State Channels?**
By using unidirectional state channels, the Consumer issues a Macaroon with a *Max Spend* caveat. The Gateway streams the AI response, continuously decrementing against this maximum limit. If the AI stops early, the Gateway only settles the exact amount used.

* **Direct L1 vs. State Channels:** Direct L1 requires pre-calculating the cost. State channels allow probabilistic, streaming cost calculation.
* **Channel Expiration:** Escrows have a hard expiration (e.g., 30 days). If the Gateway goes offline and refuses to settle, the Consumer can reclaim their funds after the timelock expires.

---

## 5. Authorization Flow

The hot path for payment authorization must complete in <5ms.

1. **402 Challenge:** Client connects to Gateway without credentials. Gateway responds `402 Payment Required` + `Macaroon Challenge` (Gateway Pubkey, Escrow ID).
2. **Macaroon Generation:** Client signs a Macaroon with their ED25519 private key. Caveats include: `escrow_id=<ID>`, `max_spend=50000`, `nonce=<UUID>`.
3. **Gateway Verification:**
* Cryptographically verifies the ED25519 signature against the Consumer's public key (retrieved from L1 Escrow state cached in Redis).
* Validates the `nonce` against Redis (`SETNX`) to prevent replay attacks.


4. **Provider Authorization:** If valid, the Gateway opens the TCP socket to the upstream AI Provider.

🟠 **Design Assumption:** The Gateway relies on an eventually consistent read-replica of L1 state to know if an escrow is valid. If a user funds an escrow, there may be a 2-3 second delay before the Gateway recognizes it.

---

## 6. Usage Metering

* **Who measures?** The Gateway acts as the trusted, neutral metering proxy between Consumer and Provider.
* **Token Counting:** The Gateway intercepts the Server-Sent Events (SSE) stream. Instead of running a heavy BPE tokenizer on the proxy path, it measures byte length of the chunks and uses a pre-calculated multiplier (e.g., 1 token ≈ 4 bytes) for real-time cutoff enforcement.
* **Post-Processing:** Once the stream closes, the upstream Provider usually injects a final `usage: { prompt_tokens, completion_tokens }` block. The Gateway uses this definitive block for the final financial settlement receipt.
* **Edge Cases:** If the Provider crashes mid-stream without sending the final `usage` block, the Gateway falls back to its internal byte-heuristic to bill the Consumer for partial work.

---

## 7. Settlement Engine

The Settlement Engine is an asynchronous worker pool that reads completed request receipts from Redis Streams.

* **Batching Strategy:** Settling every single receipt is computationally expensive. The Engine groups receipts by `Escrow ID`. It only needs to submit the *latest* cumulative receipt per channel to the smart contract.
* **Atomic Transfers:** A settlement transaction groups:
1. `AppCall` to update Escrow State.
2. `AssetTransfer` to the Provider.
3. `AssetTransfer` to the Marketplace (fee extraction).


* **Settlement Failure:** If a settlement transaction fails (e.g., L1 congestion, Consumer emptied escrow via race condition), it moves to a Dead Letter Queue (DLQ) for manual SRE review.
* 🟡 **Engineering Recommendation:** Settlement should run on a cron schedule (e.g., every 5 minutes) rather than event-driven, to maximize batch density and minimize L1 transaction fees.

---

## 8. Smart Contract Design

* **State Variables:**
* Global: `marketplace_fee_percentage`, `fee_collector_address`.
* Local (Box per channel): `consumer_address`, `provider_address`, `locked_balance`, `settled_balance`, `nonce`.


* **Roles:** `Admin` (Marketplace operator, can upgrade contract), `Gateway` (Authorized to submit settlements), `Consumer` (Can open/close channels).
* **Methods:**
* `deposit(provider_address)`: Locks USDC, creates Box.
* `settle(macaroon_proof, amount)`: Gateway submits cryptographic proof, contract transfers USDC.
* `initiate_closure()`: Consumer requests closure, starts timelock (e.g., 24 hours).
* `finalize_closure()`: Releases remaining funds to Consumer after timelock.


* **Emergency Controls:** An `Admin` can freeze new deposits in the event of a zero-day vulnerability discovery.

---

## 9. Ledger Design

* **Internal Ledger (Postgres):** Serves as a high-speed read replica for web dashboards and API usage tracking. Not strictly trusted for financial accuracy.
* **External Ledger (Algorand):** The immutable, definitive source of truth. If Postgres is corrupted, the system state can be completely reconstructed by replaying Algorand blocks.
* **Reconciliation:** A background worker continuously diffs the Postgres `settled_amount` against the L1 Smart Contract `settled_balance`. Any delta triggers a `P1` PagerDuty alert.

---

## 10. Fraud Prevention

* **Replay Attacks:** A malicious Consumer captures a valid Macaroon and resends it.
* *Mitigation:* Gateway checks the Macaroon `nonce` against a Redis cluster using `SETNX`. The Redis TTL matches the Macaroon expiration caveat.


* **Escrow Race Conditions:** A Consumer initiates a channel closure on L1 right before sending a massive AI request to the Gateway.
* *Mitigation:* The Gateway subscribes to L1 events. If a `initiate_closure` event is detected, the Gateway instantly invalidates the channel in Redis and drops active TCP connections.


* **Provider Milking:** A malicious Provider generates infinite garbage text to drain a Consumer's escrow.
* *Mitigation:* The Gateway enforces the `max_spend` caveat dynamically. Once the bytes streamed cross the threshold, the Gateway sends a TCP RST to the Provider.



---

## 11. Failure Scenarios

* **Redis Failure:**
* *Impact:* Nonce caching drops. Replay attacks become possible.
* *Recovery:* Gateway MUST fail-closed. If Redis is unreachable, return `503 Service Unavailable`. Do not process financial transactions without replay protection.


* **Gateway Crashes Mid-Stream:**
* *Impact:* The connection drops. The Consumer is not charged for the request because the Gateway died before writing the receipt to the Settlement Engine queue.
* *Recovery:* Consumer retries against a healthy Gateway node. Provider eats the compute cost of the dropped request. (Acceptable tradeoff for stateless Gateways).


* **Partial Settlement:** If an Algorand Atomic Transfer fails midway, the entire group reverts. There is no such thing as a "partial settlement" on Algorand.

---

## 12. Performance Engineering

* **Latency Targets:** Macaroon verification and Redis Nonce check must execute in `< 3ms` at the 99th percentile.
* **Connection Pooling:** The Gateway maintains persistent HTTP/2 connection pools to upstream AI Providers to avoid TLS handshake overhead on every request.
* **Expected TPS:** The system is designed to handle 10,000 concurrent streaming connections per region.
* 🔴 **Risk:** Redis network I/O limits. 10,000 concurrent requests mean 10,000 `SETNX` calls per second. Redis cluster must be properly sharded by `Escrow ID` to distribute the load.

---

## 13. Security Architecture

* **Key Management:** The Gateway holds a hot signing key used only to sign internal receipts sent to the Settlement Engine. It does NOT hold the keys to the Algorand Smart Contract.
* **Operator Keys:** The Settlement Engine holds the L1 private keys necessary to submit batch transactions. These keys are injected at runtime via AWS KMS / HashiCorp Vault.
* **Threat Model:** We assume the Consumer is malicious (attempting to get free compute) and the Provider is malicious (attempting to overcharge). The Gateway and Smart Contract must securely mediate this adversarial relationship.

---

## 14. Observability

* **Metrics:** Prometheus scrapes `gateway_auth_latency_ms`, `redis_nonce_cache_hits`, `settlement_batch_size`.
* **Financial Monitoring:** Datadog dashboards track `total_value_locked` in the Escrow contract vs. `unsettled_receipt_value` in the Redis queue. If unsettled value exceeds 50% of TVL, SREs are alerted to investigate settlement pipeline blockages.

---

## 15. Compliance Considerations

* **Auditability:** Because every settlement is batched onto a public Layer 1, anyone can run an independent node and verify the historical payouts to Providers.
* **Privacy:** The Gateway NEVER logs the prompt text or the generated response. It only logs the byte count, token count, and cryptographic signatures. This ensures compliance with SOC2 and GDPR requirements for AI data privacy.

---

## 16. Architecture Decision Records (ADR)

* **ADR 001: Macaroons over JWTs**
* *Decision:* Use Macaroons for off-chain authorization.
* *Reasoning:* JWTs cannot be safely attenuated by the client without contacting a central server. Macaroons allow a client to take a $10 root token and locally derive a $1 token for a specific prompt without incurring network latency.


* **ADR 002: L1 Settlement over L2 App-Chains**
* *Decision:* Settle directly on Algorand Mainnet via Atomic Transfers.
* *Reasoning:* Algorand's base layer provides sufficient throughput (10k TPS) and fast finality (~2.8s) to handle batched state channel closures. Operating an L2 App-Chain introduces unnecessary sequencing infrastructure and bridging risks.



---

## 17. Evolution Roadmap

1. **Hackathon / MVP:** Single-node Gateway, SQLite ledger, manual L1 Escrow funding.
2. **Beta:** Multi-region Gateways, Redis cluster for nonce tracking, automated background settlement workers.
3. **Production:** K8s Gateway deployments, HSM-backed settlement signers, automated L1 reconciliation.
4. **Protocol:** Gateways are open-sourced. Any AI Provider can run their own Gateway sidecar, completely decentralizing the network and reducing the core platform to purely the Smart Contract layer.

---

## 18. Red Team Review

* **SRE Concern:** *What if a malicious actor opens 1,000,000 escrows of $0.01 to exhaust Algorand Box storage?*
* **Mitigation:** The smart contract forces the Consumer to pay the ALGO Minimum Balance Requirement (MBR) out-of-pocket to create the Box. Opening 1,000,000 boxes would cost the attacker thousands of dollars in ALGO, which acts as an economic deterrent.


* **Security Concern:** *What if a user bypasses the Gateway and hits the AI Provider directly?*
* **Mitigation:** AI Providers configure their network firewalls (e.g., via AWS Security Groups) to strictly drop all traffic that does not originate from the Gateway's static NAT IPs.


* **Payments Concern:** *What happens to dust balances? If a channel has $0.000001 left, settling it costs more in L1 transaction fees than the value itself.*
* **Mitigation:** The Settlement Engine implements a Minimum Settlement Threshold (e.g., $0.05). Dust is rolled over. If the channel is closed, dust below the transaction fee threshold is burned/forfeited to cover the network cost.



*Review Status: Approved for Implementation.*