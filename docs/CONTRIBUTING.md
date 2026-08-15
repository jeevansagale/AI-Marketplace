# Contributing to AI Marketplace

Thank you for your interest in contributing. This document explains how to get started.

---

## Code of Conduct

Be respectful. Be constructive. Keep discussion focused on the code and product.

---

## Getting Started

### 1. Fork and clone

```bash
git clone https://github.com/jeevansagale/AI-Marketplace.git
cd AI-Marketplace
```

### 2. Set up your environment

Copy the example environment file and fill in your API keys:

```bash
cp .env.example .env
# Edit .env with your OMNI_GEMINI_API_KEY and OMNI_OPENAI_API_KEY
```

### 3. Run the project locally

```bash
# Gateway (Go)
cd apps/gateway && go run ./cmd/gateway

# Frontend (Next.js)
cd apps/web && npm install && npm run dev
```

---

## Development Workflow

### Branch naming

```
feature/short-description
fix/short-description
docs/short-description
refactor/short-description
chore/short-description
```

### Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add provider health monitoring endpoint
fix: correct session cost calculation for image models
docs: update API reference for /api/sessions
refactor: extract SSE writer into shared helper
chore: update Go dependencies
test: add catalog model slug normalization tests
```

### Pull Request checklist

Before opening a PR, verify:

- [ ] All existing tests pass (`make test`)
- [ ] New functionality has tests
- [ ] API contracts are backward compatible
- [ ] No secrets or API keys committed
- [ ] Documentation updated if behavior changed
- [ ] Code formatted (`make fmt`)
- [ ] Linted (`make lint`)

---

## Project Layout

See [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) for the full annotated directory tree.

---

## Backend (Go)

- Packages live under `apps/gateway/internal/` and `apps/controlplane/internal/`
- Shared types live in `shared/catalog/`
- Run `gofmt` before committing: `make fmt`
- Run `go vet` before committing: `make vet`
- Tests: `make test`

### Package naming

Follow Go conventions: short, lowercase, no underscores.

### Error handling

- Return errors, never panic in library code
- Wrap errors with `fmt.Errorf("context: %w", err)`
- Provider errors use the `ProviderError` type from `internal/provider`

---

## Frontend (TypeScript / Next.js)

- Pages live in `apps/web/app/`
- Shared components in `apps/web/src/components/`
- API client in `apps/web/src/lib/api.ts`
- State store in `apps/web/src/lib/stores/`

### TypeScript standards

- No `any` unless absolutely unavoidable and documented
- Prefer explicit return types on public functions
- Use interfaces for API response shapes

---

## Adding a New AI Provider

1. Add the provider entry to `shared/catalog/catalog.go` → `defaultProviders()`
2. Add the provider's models to `defaultModels()`
3. Implement the `provider.Provider` interface in a new file under `apps/gateway/internal/provider/`
4. Register the adapter in `provider.NewRegistry()`
5. Add tests

---

## Security

This project handles payments, API keys, and cryptographic proofs.

- **Never commit** `.env`, private keys, mnemonics, or API keys
- Payment verification logic in `internal/x402/` must be reviewed carefully for any changes
- Wallet and ledger operations in `internal/wallet/` and `internal/ledger/` should be treated as financial logic
- All provider error messages are sanitized before being sent to clients — do not log raw provider responses at `INFO` level

---

## Reporting Issues

Open an issue describing:

1. What you expected to happen
2. What actually happened
3. Steps to reproduce
4. Environment (OS, Go version, Node version)

---

## Questions

Open a GitHub Discussion or an issue with the `question` label.
