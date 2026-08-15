# Deployment Guide

---

## Docker (Recommended)

Build and run all services with Docker Compose:

```bash
docker compose -f infra/docker-compose/dev.yml up --build
```

Services:

- Gateway: `http://localhost:8080`
- Control Plane: `http://localhost:8081`
- PostgreSQL: `localhost:5432`

---

## Building Docker Images Manually

### Gateway

```bash
docker build -f infra/docker/gateway.Dockerfile -t marketplace-gateway .
docker run -p 8080:8080 --env-file .env marketplace-gateway
```

### Control Plane

```bash
docker build -f infra/docker/controlplane.Dockerfile -t marketplace-controlplane .
docker run -p 8081:8081 --env-file .env marketplace-controlplane
```

---

## Building Go Binaries

```bash
# Gateway
cd apps/gateway
CGO_ENABLED=0 go build -o ../../bin/gateway ./cmd/gateway

# Control Plane
cd apps/controlplane
CGO_ENABLED=0 go build -o ../../bin/controlplane ./cmd/controlplane
```

---

## Frontend (Vercel)

1. Connect the repository to Vercel
2. Set the root directory to `apps/web`
3. Add environment variable: `NEXT_PUBLIC_GATEWAY_URL=https://your-gateway-url`
4. Deploy

---

## Frontend (Self-hosted)

```bash
cd apps/web
npm install
npm run build
npm start
```

Or export static files:

```bash
npm run build
# Serve the .next/ directory with any static server
```

---

## Required Environment Variables (Production)

| Variable | Description |
| --- | --- |
| `OMNI_GEMINI_API_KEY` | Google Gemini API key |
| `OMNI_OPENAI_API_KEY` | OpenAI API key |
| `OMNI_HTTP_ADDR` | Gateway listen address (`:8080`) |
| `OMNI_MODE` | `demo` or `production` |
| `NEXT_PUBLIC_GATEWAY_URL` | Public URL of the gateway |

---

## Health Checks

Both services expose:

- `GET /livez` — liveness probe (Kubernetes-compatible)
- `GET /healthz` — readiness probe with metadata

---

## Scaling Considerations

The demo wallet is in-memory. For production:

- Replace `internal/wallet` with a PostgreSQL-backed implementation
- Add Redis for distributed session state
- Place the gateway behind a load balancer
- Use `OMNI_MODE=production` to enable real payment verification
