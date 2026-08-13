
# 🤖 AI Marketplace

> **An AI-native marketplace where agents discover, pay for, and consume AI services on a pay-per-use basis.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge\&logo=go\&logoColor=white)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black?style=for-the-badge\&logo=next.js)](https://nextjs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-Database-336791?style=for-the-badge\&logo=postgresql\&logoColor=white)](https://www.postgresql.org/)
[![Algorand](https://img.shields.io/badge/Algorand-Blockchain-000000?style=for-the-badge\&logo=algorand)](https://www.algorand.foundation/)
[![USDC](https://img.shields.io/badge/Payments-USDC-2775CA?style=for-the-badge\&logo=circle\&logoColor=white)](https://www.circle.com/usdc)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)

---

## 🌐 Live Demo

🚀 **Try AI Marketplace:**
<https://ai-marketplace-primo21.vercel.app>

📦 **GitHub Repository:**
<https://github.com/jeevansagale/AI-Marketplace>

---

## 💡 What is AI Marketplace?

AI Marketplace is a **pay-per-use infrastructure platform for AI APIs and autonomous AI agents**.

Instead of forcing developers to maintain multiple subscriptions, API keys, or prepaid plans, the marketplace allows AI agents to:

```text
Discover AI Model
      ↓
Send API Request
      ↓
Receive HTTP 402
      ↓
Authorize Payment
      ↓
Payment Verified
      ↓
AI Provider Selected
      ↓
Stream AI Response
      ↓
Usage Metered
      ↓
Settlement
```

The core idea is simple:

> **Machines should be able to discover, pay for, and use AI services without requiring traditional human-oriented payment workflows.**

The production architecture combines an **HTTP 402 payment flow**, **Algorand-based settlement**, **USDC**, state-channel style off-chain billing, and an OpenAI-compatible API interface.

---

# 🚨 The Problem

The AI ecosystem is becoming increasingly fragmented.

Developers and autonomous agents often need to deal with:

* Multiple AI providers
* Different API formats
* API keys
* Monthly subscriptions
* Prepaid credits
* Complicated billing systems
* Provider-specific SDKs
* Difficulty comparing model prices
* Lack of machine-to-machine payment infrastructure

This becomes even more important as **AI agents start acting autonomously**.

An AI agent should be able to say:

> "I need OCR."

Then automatically:

1. Find an OCR provider.
2. Compare pricing.
3. Select a suitable service.
4. Pay for the request.
5. Consume the API.
6. Pay only for the actual usage.

**AI Marketplace is designed to enable this workflow.**

---

# ✨ Key Features

## 🤖 AI Model Marketplace

Discover AI services through a centralized marketplace.

Supported service categories can include:

* 💬 LLMs
* 👁️ Vision models
* 📝 OCR
* 🎙️ Speech / transcription
* 📊 Data processing
* 🧠 AI agents
* 🔌 Other AI APIs

Models can be searched, filtered, categorized, and ranked based on factors such as price, latency, and availability.

---

## 💳 Pay-Per-Use AI

No mandatory monthly subscription model.

Users pay based on actual usage.

```text
Request → Usage → Payment
```

This makes the platform suitable for both developers and autonomous agents that need dynamic AI service consumption.

---

## ⚡ HTTP 402 Payment Flow

The platform uses the HTTP `402 Payment Required` mechanism to create a machine-readable payment handshake.

Example:

```http
POST /v1/chat/completions

HTTP/1.1 402 Payment Required
```

The client receives a payment challenge, generates the required proof, and retries the request.

```text
Client
  │
  │ API Request
  ▼
Gateway
  │
  │ 402 Payment Required
  ▼
Client
  │
  │ Payment Proof
  ▼
Gateway
  │
  │ Verify
  ▼
AI Provider
  │
  │ Streaming Response
  ▼
Client
```

---

## 🔐 Cryptographic Payment Verification

The system uses cryptographic signatures and nonce validation to protect payment requests.

The MVP uses:

* Ed25519 signatures
* Request hashing
* Nonce validation
* Replay protection
* Channel budgets
* Payment proofs

This prevents the same payment authorization from being replayed.

---

## 🪙 Algorand + USDC

The production architecture uses **Algorand** for settlement and **USDC** as the payment asset.

Algorand is used because the architecture targets:

* Fast finality
* Low transaction costs
* Programmable escrow
* Machine-to-machine payments
* Batched settlement

The production design avoids putting every AI token transaction directly on-chain.

Instead:

```text
AI Usage
   ↓
Off-chain accounting
   ↓
State Channel
   ↓
Batch Settlement
   ↓
Algorand
```

This allows high-frequency AI usage without requiring an on-chain transaction for every token.

---

# 🌊 Real-Time AI Streaming

The gateway supports streaming AI responses through **Server-Sent Events (SSE)**.

The gateway can:

1. Receive the provider stream.
2. Forward the stream to the consumer.
3. Meter usage.
4. Update the active payment/channel balance.
5. Record the final usage.
6. Queue settlement information.

```text
AI Provider
     │
     │ SSE
     ▼
API Gateway
     │
     ├── Token Metering
     ├── Balance Tracking
     └── Settlement Queue
     │
     ▼
AI Consumer
```

Streaming is an important part of the architecture because AI requests can remain open while tokens are continuously generated.

---

# 🔌 OpenAI-Compatible API

One of the major design goals is compatibility with the OpenAI API format.

Instead of requiring developers to completely rewrite their applications, the marketplace is designed so that applications can primarily change their API `base_url`.

Example:

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://your-marketplace-url/v1",
    api_key="your-payment-enabled-client"
)

response = client.chat.completions.create(
    model="your-model",
    messages=[
        {
            "role": "user",
            "content": "Hello!"
        }
    ]
)
```

This allows existing AI applications and agent frameworks to integrate with the marketplace with minimal changes.

---

# 🏗️ Architecture

The system is designed as a modular service-oriented architecture.

```text
                         ┌─────────────────────┐
                         │   AI Developer /    │
                         │      AI Agent       │
                         └──────────┬──────────┘
                                    │
                                    │ HTTPS
                                    ▼
                         ┌─────────────────────┐
                         │     API Gateway     │
                         │      Go / Fiber     │
                         └──────────┬──────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
              ┌──────────┐   ┌────────────┐   ┌─────────────┐
              │ Macaroon │   │  Payment   │   │   Provider  │
              │  Engine  │   │   Engine   │   │   Registry  │
              └──────────┘   └────────────┘   └─────────────┘
                                    │               │
                                    │               ▼
                                    │        ┌─────────────┐
                                    │        │ AI Provider │
                                    │        └─────────────┘
                                    │
                                    ▼
                            ┌──────────────┐
                            │   Algorand   │
                            │     L1       │
                            └──────────────┘
```

The production architecture separates the platform into:

* API Gateway
* Authentication / Macaroon Engine
* Payment Engine
* Provider Registry
* PostgreSQL
* Redis
* AI Providers
* Algorand settlement layer

---

# 🧩 Technology Stack

### Frontend

| Technology        | Purpose                        |
| ----------------- | ------------------------------ |
| **Next.js 15**    | Web application                |
| **TypeScript**    | Type-safe frontend development |
| **Tailwind CSS**  | UI styling                     |
| **shadcn/ui**     | UI components                  |
| **Framer Motion** | Animations                     |
| **React Query**   | Server-state management        |
| **Zustand**       | Client-side state management   |

### Backend

| Technology           | Purpose                        |
| -------------------- | ------------------------------ |
| **Go**               | Core backend services          |
| **Fiber**            | HTTP framework                 |
| **Go Reverse Proxy** | AI provider proxying           |
| **SSE**              | Streaming AI responses         |
| **Ed25519**          | Payment signature verification |

### Data

| Technology     | Purpose                                        |
| -------------- | ---------------------------------------------- |
| **PostgreSQL** | Persistent marketplace data                    |
| **Redis**      | Runtime state, nonce protection, rate limiting |
| **Queues**     | Asynchronous settlement processing             |

### Blockchain

| Technology          | Purpose                          |
| ------------------- | -------------------------------- |
| **Algorand**        | Settlement infrastructure        |
| **USDC**            | Payment currency                 |
| **Smart Contracts** | Escrow and settlement            |
| **State Channels**  | High-frequency off-chain billing |

The MVP intentionally keeps the infrastructure lighter by using PostgreSQL and in-memory/demo implementations while preserving production-oriented interfaces.

---

# 🗂️ Project Structure

```text
AI-Marketplace/
│
├── apps/
│   └── web/
│       ├── app/
│       │   ├── marketplace/
│       │   ├── playground/
│       │   ├── consumer/
│       │   ├── provider/
│       │   ├── admin/
│       │   └── architecture/
│       │
│       ├── components/
│       └── lib/
│
├── services/
│   ├── gateway/
│   └── controlplane/
│
├── shared/
│   └── catalog/
│
├── infra/
│
├── API_Specification.md
├── Architecture.md
├── Engineering_Design_Document.md
├── HACKATHON_MVP_PLAN.md
├── PaymentEngine.md
├── System_Design.md
├── Research.md
├── Research_v2.md
├── Makefile
├── Run.sh
├── Run.bat
└── go.work
```

The repository separates the frontend, gateway, control plane, infrastructure, shared catalog, and extensive architecture/engineering documentation.

---

# 🔄 How It Works

## 1. Discover

An AI developer or agent searches the marketplace for an appropriate model.

```text
"Find me a low-cost vision model"
```

The marketplace returns available providers and models.

---

## 2. Request

The consumer sends an OpenAI-compatible request:

```http
POST /v1/chat/completions
```

---

## 3. Payment Challenge

If payment authorization is missing, the gateway returns:

```http
402 Payment Required
```

The response contains the information required for the client to authorize payment.

---

## 4. Payment Verification

The client provides payment proof.

The gateway verifies:

* Signature
* Request hash
* Nonce
* Payment/channel balance
* Authorization constraints

---

## 5. Provider Routing

The marketplace selects a suitable provider based on marketplace information such as:

* Price
* Latency
* Availability
* Model
* Provider health

---

## 6. AI Streaming

The gateway forwards the request to the provider and streams the response back to the consumer.

```text
Provider
   ↓
SSE
   ↓
Gateway
   ↓
Consumer
```

---

## 7. Usage Metering

The gateway tracks the actual usage associated with the request and updates the active payment state.

---

## 8. Settlement

Usage records are queued for settlement.

The production architecture batches settlements instead of performing a blockchain transaction for every individual token or request.

---

# 🖥️ Platform Modules

## 🏪 Marketplace

Discover AI services through:

* Model search
* Categories
* Pricing
* Latency
* Availability
* Provider information

---

## 🧪 AI Playground

The playground provides an OpenAI-style interface for testing models.

It demonstrates the complete payment flow:

```text
Prompt
  ↓
402 Payment Required
  ↓
Wallet / Payment
  ↓
Verification
  ↓
Provider Routing
  ↓
Streaming
  ↓
Settlement
```

---

## 👤 Consumer Dashboard

Consumers can view:

* Balance
* Spending
* Requests
* Transactions
* Model usage
* Channel state
* Usage analytics

---

## 🧑‍💻 Provider Dashboard

Providers can monitor:

* Revenue
* Registered models
* Requests
* Latency
* Health
* Payouts
* Usage

---

## 🛠️ Admin Dashboard

Administrators can monitor:

* Providers
* Models
* Settlement queue
* Transactions
* Gateway metrics
* System health

These dashboard flows are part of the project's MVP design.

---

# 🔐 Security

Security is a core part of the architecture.

### Replay Protection

Every signed request contains a nonce.

```text
Request
   ↓
Nonce
   ↓
Check uniqueness
   ↓
Verify signature
   ↓
Accept / Reject
```

### Zero-Trust Provider Model

The gateway does not establish direct communication between the consumer and provider.

```text
Consumer
   X
   │
   │ No direct connection
   │
Gateway
   │
   ▼
Provider
```

### Payment Isolation

Consumers deposit only the amount they intend to use through the escrow/channel mechanism.

### Privacy

AI prompts and responses are not intended to be stored in the settlement ledger or operational logs.

The production architecture explicitly keeps sensitive AI request payloads out of logs.

---

# 🧪 Demo Mode

The project includes an **offline-first MVP/demo architecture**.

The demo can operate without requiring the complete production blockchain infrastructure.

### Real in the MVP

* HTTP 402 handshake
* Ed25519 signature verification
* Nonce/replay protection
* AI streaming
* OpenAI-compatible API
* Marketplace APIs
* Dashboards
* Demo authentication

### Mocked / Demo Infrastructure

* Settlement batching
* Some provider routing infrastructure
* Redis/NATS/Kafka/Kubernetes production infrastructure
* Blockchain settlement when running without a real wallet

A bundled Demo Wallet and Mock Provider allow the core flow to be demonstrated locally.

---

# 🚀 Getting Started

## Prerequisites

Make sure you have:

* Git
* Go
* Node.js
* npm/pnpm
* Docker
* Docker Compose

---

## Clone the Repository

```bash
git clone https://github.com/jeevansagale/AI-Marketplace.git

cd AI-Marketplace
```

---

## Run the Project

The repository includes platform-specific run scripts and a `Makefile`.

### Linux / macOS

```bash
./Run.sh
```

or:

```bash
make dev
```

### Windows

```bat
Run.bat
```

The MVP architecture is designed around local PostgreSQL/Docker infrastructure and seeded demo data so that the core experience can run without relying on external blockchain services.

---

# 📡 API

The marketplace exposes an OpenAI-compatible interface along with marketplace, consumer, provider, and admin APIs.

### Core AI Endpoint

```http
POST /v1/chat/completions
```

### Model Discovery

```http
GET /v1/models
```

### Marketplace Search

```http
GET /v1/marketplace/models
```

### Authentication

```http
POST /v1/auth/wallet/connect
```

### Consumer APIs

```http
GET /v1/consumer/overview
GET /v1/consumer/usage
GET /v1/consumer/transactions
GET /v1/consumer/channels
```

### Provider APIs

```http
GET /v1/provider/overview
GET /v1/provider/models
GET /v1/provider/revenue
GET /v1/provider/requests
GET /v1/provider/payouts
```

### Admin APIs

```http
GET /v1/admin/providers
GET /v1/admin/models
GET /v1/admin/settlements
GET /v1/admin/health
GET /v1/admin/metrics
GET /v1/admin/transactions
```

The complete API contract is documented in [`API_Specification.md`](API_Specification.md).

---

# 📚 Documentation

The repository contains extensive technical documentation.

| Document                                                           | Description                               |
| ------------------------------------------------------------------ | ----------------------------------------- |
| [`Architecture.md`](Architecture.md)                               | System architecture and production design |
| [`API_Specification.md`](API_Specification.md)                     | API contracts                             |
| [`Engineering_Design_Document.md`](Engineering_Design_Document.md) | Engineering design                        |
| [`System_Design.md`](System_Design.md)                             | System-level design                       |
| [`PaymentEngine.md`](PaymentEngine.md)                             | Payment architecture                      |
| [`HACKATHON_MVP_PLAN.md`](HACKATHON_MVP_PLAN.md)                   | MVP and demo strategy                     |
| [`Research.md`](Research.md)                                       | Research and background                   |
| [`Research_v2.md`](Research_v2.md)                                 | Updated research                          |
| [`ENGINEERING_AUDIT.md`](ENGINEERING_AUDIT.md)                     | Engineering audit                         |

---

# 🎯 Design Goals

The platform is designed around four major principles:

### 1. Machine-to-Machine Commerce

AI agents should be able to purchase AI capabilities autonomously.

### 2. Usage-Based Pricing

Pay for what is actually consumed rather than relying exclusively on subscriptions.

### 3. Provider Agnosticism

Developers should be able to discover and consume AI services from multiple providers through a common interface.

### 4. Low-Latency Payments

Payment verification should not become a bottleneck for AI inference.

The production architecture targets **sub-50 ms gateway overhead** while supporting large numbers of concurrent streaming connections.

---

# 🛣️ Roadmap

## Phase 1 — MVP

* [x] AI marketplace
* [x] OpenAI-compatible API
* [x] HTTP 402 payment flow
* [x] Ed25519 verification
* [x] Replay protection
* [x] AI streaming
* [x] Demo wallet
* [x] Consumer dashboard
* [x] Provider dashboard
* [x] Admin dashboard

## Phase 2 — Blockchain Integration

* [ ] Full Algorand Testnet integration
* [ ] Production escrow contracts
* [ ] Real USDC settlement
* [ ] WalletConnect integration
* [ ] Automated settlement batching

## Phase 3 — Production Infrastructure

* [ ] Redis cluster
* [ ] Message queue infrastructure
* [ ] Kubernetes deployment
* [ ] Multi-region gateway
* [ ] Advanced observability
* [ ] Automated provider health monitoring

## Phase 4 — AI Agent Economy

* [ ] Autonomous agent wallets
* [ ] Agent-to-agent payments
* [ ] Automated model selection
* [ ] Dynamic pricing
* [ ] Provider reputation system
* [ ] Cross-chain payment support
* [ ] AI service composition

---

# 🌍 Vision

The long-term vision is to build an **open financial and service layer for AI agents**.

Imagine an autonomous agent receiving a task:

```text
"Analyze this document."
```

Instead of requiring a human to:

```text
Choose Provider
     ↓
Create Account
     ↓
Add API Key
     ↓
Buy Credits
     ↓
Configure SDK
     ↓
Send Request
```

The agent can eventually:

```text
Understand Task
      ↓
Discover Required AI Capability
      ↓
Compare Providers
      ↓
Authorize Payment
      ↓
Consume Service
      ↓
Pay Based on Usage
      ↓
Complete Task
```

This transforms AI APIs from traditional subscription-based products into **discoverable, programmable, machine-consumable services**.

---

# 🤝 Contributing

Contributions are welcome!

```bash
# Fork the repository

# Create your feature branch
git checkout -b feature/amazing-feature

# Commit your changes
git commit -m "feat: add amazing feature"

# Push the branch
git push origin feature/amazing-feature

# Open a Pull Request
```

Before submitting a PR:

* Keep API contracts backward compatible.
* Add tests for new functionality.
* Avoid committing secrets or private keys.
* Update relevant documentation.
* Keep security-sensitive payment logic carefully reviewed.

---

# ⚠️ Security Notice

This project involves payments, cryptographic signatures, wallets, and blockchain infrastructure.

**Do not use real funds or production private keys in local development.**

Use demo/testnet wallets and credentials while experimenting with the project.

Never commit:

```text
.env
private keys
mnemonics
wallet secrets
API keys
database credentials
```

---

# 📜 License

This project is intended as an open-source project. See the [`LICENSE`](LICENSE) file for the applicable license.

---

# 👥 Team / Project

**AI Marketplace**

Built around:

> **AI × Agents × Payments × Blockchain**

---

## ⭐ Support the Project

If you find the project interesting:

⭐ Star the repository
🍴 Fork it
🐛 Report issues
💡 Suggest improvements
🤝 Contribute

---

<p align="center">

### 🤖 Let AI Agents Pay for AI

**Discover → Pay → Use → Settle**

</p>
=======
# OmniRoute AI Marketplace (x402 + Algorand / INR Wallet)

Production-ready, pay-per-session AI inference marketplace backed by real AI infrastructure (**Google Gemini** & **OpenAI**).

---

## 🚀 Supported Functional Providers & Models

All models listed in the catalog are backed by real upstream API integrations with streaming SSE inference and per-session wallet metering.

| Provider | Model | Model Slug | Context Window | Description |
| --- | --- | --- | --- | --- |
| **Gemini** | Gemini 2.0 Flash | `gemini-2.0-flash` | 1,048,576 | Low-latency multimodal generation |
| **Gemini** | Gemini 1.5 Flash | `gemini-1.5-flash` | 1,048,576 | Fast and versatile performance |
| **Gemini** | Gemini 1.5 Pro | `gemini-1.5-pro` | 2,097,152 | Mid-tier reasoning for complex tasks |
| **OpenAI** | GPT-4o | `gpt-4o` | 128,000 | Flagship high-intelligence model |
| **OpenAI** | GPT-4o Mini | `gpt-4o-mini` | 128,000 | Affordable small model for lightweight tasks |
| **OpenAI** | GPT-3.5 Turbo | `gpt-3.5-turbo` | 16,385 | Legacy fast text model |

---

## 🔑 Environment Setup

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Open `.env` and paste your actual API keys:

   ```env
   OMNI_GEMINI_API_KEY=your-gemini-api-key
   OMNI_OPENAI_API_KEY=your-openai-api-key
   ```

3. Run the gateway. It will automatically load the `.env` file on local startup:

   ```bash
   cd apps/gateway
   go run ./cmd/gateway
   ```

---

## 🛠 Running Locally

### 1. Run Gateway (Go)

```bash
cd apps/gateway
go run ./cmd/gateway
```

The gateway listens on `http://localhost:8080`.

### 2. Run Web Frontend (Next.js 15)

```bash
cd apps/web
npm run dev
```

Open `http://localhost:3000` to access the Marketplace & Playground.

---

## 🧪 Testing

```bash
# Test shared catalog package
cd shared/catalog
go test ./...

# Test gateway package
cd apps/gateway
go test ./...

# Typecheck web frontend
cd apps/web
npx tsc --noEmit
```
