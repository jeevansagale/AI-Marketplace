-- Pay-Per-Use AI Marketplace (x402 + Algorand) — Hackathon MVP schema + seed
-- Production-shaped subset: same column semantics as full spec.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ── Users ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE,
    display_name  TEXT,
    role          TEXT NOT NULL DEFAULT 'consumer' CHECK (role IN ('consumer','provider','admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Providers ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS providers (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    slug           TEXT UNIQUE NOT NULL,
    base_url       TEXT NOT NULL,
    wallet_address TEXT,
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','suspended')),
    reputation     NUMERIC(3,2) NOT NULL DEFAULT 0.95,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Models ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS models (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id        UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    slug               TEXT UNIQUE NOT NULL,
    category           TEXT NOT NULL CHECK (category IN ('chat','vision','ocr','embeddings','audio')),
    description        TEXT,
    context_window     INTEGER NOT NULL DEFAULT 8192,
    price_base_micro   BIGINT NOT NULL DEFAULT 0,      -- base fee per request
    price_prompt_micro BIGINT NOT NULL DEFAULT 0,      -- per 1k prompt tokens
    price_completion_micro BIGINT NOT NULL DEFAULT 0,  -- per 1k completion tokens
    p90_latency_ms     INTEGER NOT NULL DEFAULT 400,
    throughput_tps     NUMERIC(6,1) NOT NULL DEFAULT 60.0,
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_models_category ON models(category, is_active);
CREATE INDEX IF NOT EXISTS idx_models_name_trgm ON models USING gin(name gin_trgm_ops);

-- ── Wallets ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS wallets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    algorand_address TEXT UNIQUE NOT NULL,
    asa_id          BIGINT NOT NULL DEFAULT 31566704,
    nickname        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Channels (escrow state, production shape) ─────────────────────────────
CREATE TABLE IF NOT EXISTS channels (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id             UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    app_id                BIGINT NOT NULL DEFAULT 0,
    locked_micro_usdc     BIGINT NOT NULL DEFAULT 0,
    settled_micro_usdc    BIGINT NOT NULL DEFAULT 0,
    latest_nonce          BIGINT NOT NULL DEFAULT 0,
    status                TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closing','closed','disputed')),
    opened_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at             TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_channels_status ON channels(status);

-- ── Receipts (append-only usage/billing ledger) ───────────────────────────
CREATE TABLE IF NOT EXISTS receipts (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id                UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    nonce                     TEXT UNIQUE NOT NULL,
    model_id                  UUID NOT NULL REFERENCES models(id),
    provider_id               UUID NOT NULL REFERENCES providers(id),
    prompt_tokens             INTEGER NOT NULL DEFAULT 0,
    completion_tokens         INTEGER NOT NULL DEFAULT 0,
    delta_micro_usdc          BIGINT NOT NULL DEFAULT 0,
    cumulative_spend_micro_usdc BIGINT NOT NULL DEFAULT 0,
    gateway_signature         TEXT NOT NULL DEFAULT '',
    status                    TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','settled','dead')),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_receipts_channel_created ON receipts(channel_id, created_at DESC);

-- ── Settlements ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS settlements (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_ref      TEXT UNIQUE NOT NULL,
    status         TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','submitted','confirmed','failed')),
    total_micro_usdc BIGINT NOT NULL DEFAULT 0,
    fee_micro_usdc BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS settlement_items (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_id      UUID NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
    receipt_id         UUID NOT NULL REFERENCES receipts(id),
    provider_id        UUID NOT NULL REFERENCES providers(id),
    amount_micro_usdc  BIGINT NOT NULL DEFAULT 0,
    fee_micro_usdc     BIGINT NOT NULL DEFAULT 0
);

-- ── API Keys / Demo auth ──────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash    TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL DEFAULT 'Demo Key',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ══════════════════════════════════════════════════════════════════════════
-- SEED — demo must never look empty
-- ══════════════════════════════════════════════════════════════════════════

INSERT INTO users (id, email, display_name, role) VALUES
    ('00000000-0000-0000-0000-000000000001', 'demo@marketplace.dev', 'Demo Agent', 'consumer'),
    ('00000000-0000-0000-0000-000000000002', 'alice@marketplace.dev', 'Alice (Agent Ops)', 'consumer'),
    ('00000000-0000-0000-0000-000000000003', 'provider@marketplace.dev', 'Provider Ops', 'provider'),
    ('00000000-0000-0000-0000-000000000004', 'admin@marketplace.dev', 'Platform Admin', 'admin')
ON CONFLICT (id) DO NOTHING;

INSERT INTO providers (id, name, slug, base_url, wallet_address, status, reputation) VALUES
    ('10000000-0000-0000-0000-000000000001', 'Nova Compute',       'nova',       'http://gateway:8080/provider/nova/v1',       'TESTNETWALLETNOVA000000000000000000000000000000', 'active', 0.98),
    ('10000000-0000-0000-0000-000000000002', 'AlgoForge',          'algoforge',  'http://gateway:8080/provider/algoforge/v1',   'TESTNETWALLETALGO00000000000000000000000000000',  'active', 0.96),
    ('10000000-0000-0000-0000-000000000003', 'TensorGrid',         'tensorgrid', 'http://gateway:8080/provider/tensorgrid/v1',  'TESTNETWALLETTENSOR000000000000000000000000000',  'active', 0.94),
    ('10000000-0000-0000-0000-000000000004', 'VisionForge',        'visionforge','http://gateway:8080/provider/visionforge/v1', 'TESTNETWALLETVISION000000000000000000000000000',  'active', 0.92),
    ('10000000-0000-0000-0000-000000000005', 'VectorHaven',        'vectorhaven','http://gateway:8080/provider/vectorhaven/v1', 'TESTNETWALLETVECTOR000000000000000000000000000',  'active', 0.97),
    ('10000000-0000-0000-0000-000000000006', 'AudioWave',          'audiowave',  'http://gateway:8080/provider/audiowave/v1',   'TESTNETWALLETAUDIO0000000000000000000000000000',  'paused',  0.89)
ON CONFLICT (id) DO NOTHING;

INSERT INTO models (id, provider_id, name, slug, category, description, context_window, price_base_micro, price_prompt_micro, price_completion_micro, p90_latency_ms, throughput_tps, is_active) VALUES
    ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'market/llama-3.3-70b-instruct', 'market-llama-3.3-70b-instruct', 'chat',       'Flagship Llama 3.3 70B — best quality per dollar.', 131072, 100, 2, 8, 380, 85.4, true),
    ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'market/llama-3.1-8b-instruct',  'market-llama-3.1-8b-instruct',  'chat',       'Fast 8B instruct for cost-sensitive workloads.',    131072, 50,  1, 3, 140, 220.0, true),
    ('20000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000002', 'algoforge/deepseek-r1-32b',        'algoforge-deepseek-r1-32b',        'chat',       'Reasoning model with chain-of-thought.',            65536,  120, 2, 6, 520, 48.2,  true),
    ('20000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000003', 'tensorgrid/mixtral-8x7b',          'tensorgrid-mixtral-8x7b',          'chat',       'Sparse MoE — high throughput.',                     32768,  80,  1, 4, 210, 150.0, true),
    ('20000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000001', 'market/qwen-2.5-72b',           'market-qwen-2.5-72b',           'chat',       'Multilingual 72B assistant.',                       131072, 110, 2, 7, 470, 62.0, true),
    ('20000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000004', 'visionforge/gpt-4o-vision-class',  'visionforge-gpt-4o-vision-class',  'vision',     'Image understanding and captioning.',                128000, 500, 10, 15, 850, 25.0, true),
    ('20000000-0000-0000-0000-000000000007', '10000000-0000-0000-0000-000000000004', 'visionforge/yolo-v11-ocr',         'visionforge-yolo-v11-ocr',         'ocr',        'Scene-text OCR with bounding boxes.',               4096,   300, 8, 12, 620, 30.0, true),
    ('20000000-0000-0000-0000-000000000008', '10000000-0000-0000-0000-000000000005', 'vectorhaven/text-embedding-3',     'vectorhaven-text-embedding-3',     'embeddings', '768-dim dense embeddings for RAG.',                8192,   40,  1, 0, 90,  400.0, true),
    ('20000000-0000-0000-0000-000000000009', '10000000-0000-0000-0000-000000000005', 'vectorhaven/bge-m3',               'vectorhaven-bge-m3',               'embeddings', 'Multilingual embedding model.',                     8192,   40,  1, 0, 100, 380.0, true),
    ('20000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000006', 'audiowave/whisper-large-v3',       'audiowave-whisper-large-v3',       'audio',      'Speech-to-text transcription.',                     262144, 600, 20, 0, 1200, 12.0, false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO wallets (id, user_id, algorand_address, asa_id, nickname) VALUES
    ('30000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'DEMOWALLET000000000000000000000000000000000', 31566704, 'Demo Agent Wallet'),
    ('30000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'DEMOWALLETALICE00000000000000000000000000000', 31566704, 'Alice Ops Wallet')
ON CONFLICT (id) DO NOTHING;

-- Demo channel: 100 USDC locked, 4.3 USDC spent (seeded history)
INSERT INTO channels (id, wallet_id, app_id, locked_micro_usdc, settled_micro_usdc, latest_nonce, status, opened_at) VALUES
    ('40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 98765432, 100_000_000, 4_300_000, 42, 'open', now() - interval '3 days'),
    ('40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000002', 98765432, 50_000_000,  1_920_000, 18, 'open', now() - interval '5 days')
ON CONFLICT (id) DO NOTHING;

-- Seeded receipts history so dashboards never appear empty.
INSERT INTO receipts (id, channel_id, nonce, model_id, provider_id, prompt_tokens, completion_tokens, delta_micro_usdc, cumulative_spend_micro_usdc, status, created_at)
SELECT
    gen_random_uuid(),
    '40000000-0000-0000-0000-000000000001',
    'seed-' || '40000000-0000-0000-0000-000000000001' || '-' || g,
    (ARRAY['20000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000002','20000000-0000-0000-0000-000000000003','20000000-0000-0000-0000-000000000005']::uuid[])[1 + g % 4],
    (ARRAY['10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000002']::uuid[])[1 + g % 2],
    140 + (g % 320),
    120 + (g % 900),
    (900 + g * 13),
    ((900 + g * 13) * 0.6)::bigint,
    'settled',
    now() - make_interval(hours => g)
FROM generate_series(0, 239) AS seq(g)
ON CONFLICT (nonce) DO NOTHING;

-- Historical settlement batch (confirmed)
INSERT INTO settlements (id, batch_ref, status, total_micro_usdc, fee_micro_usdc, created_at, confirmed_at) VALUES
    ('50000000-0000-0000-0000-000000000001', 'sb-demo-0001', 'confirmed', 4_300_000, 43_000, now() - interval '6 hours', now() - interval '5.9 hours')
ON CONFLICT (id) DO NOTHING;

-- Demo API key (hash of "omni_demo_key")
INSERT INTO api_keys (id, user_id, key_hash, name) VALUES
    ('60000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', encode(sha256('omni_demo_key'::bytea), 'hex'), 'Demo Agent Key')
ON CONFLICT (id) DO NOTHING;

COMMIT;