// Marketplace API client — consumes the controlplane catalog.
// In prod this routes through BFF/Next rewrites; for the demo we call the
// controlplane directly (CORS-enabled via same-host ports).

export interface Provider {
  id: string;
  name: string;
  slug: string;
  base_url: string;
  wallet_address: string;
  status: string;
  reputation: number;
}

export interface Model {
  id: string;
  name: string;
  slug: string;
  category: string;
  description: string;
  context_window: number;
  price_prompt_micro_usdc: number;
  price_completion_micro_usdc: number;
  provider: string;
  p90_latency_ms: number;
  throughput_tps: number;
  is_active: boolean;
}

export interface Catalog {
  providers: Provider[];
  models: Model[];
}

const CONTROLPLANE_URL =
  process.env.NEXT_PUBLIC_CONTROLPLANE_URL ?? "http://localhost:8081";

export async function fetchProviders(): Promise<Provider[]> {
  const res = await fetch(`${CONTROLPLANE_URL}/api/v1/providers`, {
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`providers ${res.status}`);
  const data = (await res.json()) as { providers: Provider[] };
  return data.providers;
}

export async function fetchModels(): Promise<Model[]> {
  const res = await fetch(`${CONTROLPLANE_URL}/api/v1/models`, {
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`models ${res.status}`);
  const data = (await res.json()) as { models: Model[] };
  return data.models;
}

export async function fetchCatalog(): Promise<Catalog> {
  const [providers, models] = await Promise.all([
    fetchProviders(),
    fetchModels(),
  ]);
  return { providers, models };
}

// micro-USDC (1e-6 USDC) → USDC string, e.g. 2000 → "0.002"
export function microToUsdc(v: number): string {
  return (v / 1_000_000).toFixed(6).replace(/\.?0+$/, "");
}