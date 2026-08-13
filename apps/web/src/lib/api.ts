// Marketplace v2 API client — consumes the gateway's /api/* surface.
// All money values are integer paise (₹). Format with formatINR().

export interface Provider {
  id: string;
  name: string;
  slug: string;
  base_url: string;
  kind: string;
  status: string;
}

export interface MarketModel {
  id: string;
  provider_id: string;

  name: string;
  slug: string;
  category: string;
  description: string;

  context_window: number;

  // THESE MATCH THE BACKEND
  price_base_paise: number;
  price_prompt_paise: number;
  price_completion_paise: number;

  provider: string;

  p90_latency_ms: number;
  throughput_tps: number;

  is_active: boolean;
}

export interface Catalog {
  providers: Provider[];
  models: MarketModel[];
}

export interface WalletBalance {
  balance_paise: number; // paise
  reserved_paise: number; // paise
  available_paise: number; // paise
}

export interface WalletTx {
  id: string;
  kind: string;
  amount_paise: number; // paise (signed: negative = spend)
  reference: string;
  timestamp: string;
}

export interface SessionEvent {
  stage: string;
  label: string;
  at: string;
  note?: string;
}

export interface Session {
  id: string;
  wallet_id: string;
  model_id: string;
  model_slug: string;
  model_name: string;
  provider_id: string;
  provider_name: string;
  reservation_id: string;
  stage: string;
  estimated_paise: number;
  actual_cost_paise: number;
  refunded_paise: number;
  timeline: SessionEvent[];
  usage: {
    prompt_tokens: number;
    completion_tokens: number;
    latency_ms: number;
    request_count: number;
  };
  created_at: string;
  updated_at: string;
}

export interface StreamSummary {
  session_id: string;
  reserved_paise: number;
  actual_cost_paise: number;
  refunded_paise: number;
  prompt_tokens: number;
  completion_tokens: number;
  latency_ms: number;
}

export interface LedgerEntry {
  id: number;
  timestamp: string;
  kind: string;
  wallet_id: string;
  provider_id: string;
  model_id: string;
  reservation: string;
  amount: number;
  reference: string;
  status: string;
}

export interface TopupOrder {
  order_id: string;
  amount_paise: number;
  status: string;
  checkout_url: string;
}

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${GATEWAY_URL}${path}`, { cache: "no-store" });
  if (!res.ok) {
    let msg = `${path} → ${res.status}`;
    try {
      const body = await res.json();
      msg = body?.error?.message ?? msg;
    } catch {
      /* ignore */
    }
    throw new ApiError(msg, res.status);
  }
  return (await res.json()) as T;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${GATEWAY_URL}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    let msg = `${path} → ${res.status}`;
    try {
      const data = await res.json();
      msg = data?.error?.message ?? msg;
    } catch {
      /* ignore */
    }
    throw new ApiError(msg, res.status);
  }
  return (await res.json()) as T;
}

export const api = {
  catalog: () => getJSON<Catalog>("/api/catalog"),
  wallet: () => getJSON<WalletBalance>("/api/wallet"),
  transactions: () =>
    getJSON<{ transactions: WalletTx[] }>("/api/wallet/transactions"),
  ledger: () => getJSON<{ entries: LedgerEntry[]; len: number }>("/api/ledger"),
  topup: (amountINR: number) =>
    postJSON<TopupOrder>("/api/topup", { amount_inr: amountINR }),
  topupVerify: (orderId: string) =>
    postJSON<{ status: string; credited_paise: number }>("/api/topup/verify", {
      order_id: orderId,
    }),
  createSession: (model: string, maxTokens: number) =>
    postJSON<Session>("/api/sessions", { model, max_tokens: maxTokens }),
  sessions: () => getJSON<{ sessions: Session[] }>("/api/sessions"),
};

export interface StreamCallbacks {
  onChunk: (text: string) => void;
  onDone: (summary: StreamSummary) => void;
  onError: (message: string) => void;
}

// streamSession opens the SSE endpoint and parses OpenAI-compatible
// `data: {...}` frames, invoking callbacks as tokens arrive.
export async function streamSession(
  sessionId: string,
  messages: { role: string; content: string }[],
  maxTokens: number,
  cb: StreamCallbacks,
): Promise<void> {
  const res = await fetch(`${GATEWAY_URL}/api/sessions/${sessionId}/stream`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ messages, max_tokens: maxTokens }),
  });
  if (!res.ok) {
    let msg = `stream → ${res.status}`;
    try {
      const data = await res.json();
      msg = data?.error?.message ?? msg;
    } catch {
      /* ignore */
    }
    cb.onError(msg);
    return;
  }
  if (!res.body) {
    cb.onError("stream: empty response body");
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const handleFrame = (frame: string) => {
    const line = frame.trim();
    if (!line.startsWith("data:")) return;
    const payload = line.slice(5).trim();
    if (payload === "[DONE]") return;
    let data: unknown;
    try {
      data = JSON.parse(payload);
    } catch {
      cb.onChunk(payload);
      return;
    }
    const obj = data as Record<string, unknown>;
    if (obj.error) {
      cb.onError(String(obj.error));
      return;
    }
    if (obj.summary && obj.usage) {
      cb.onDone(obj.summary as StreamSummary);
      return;
    }
    const choices = obj.choices as
      | { delta?: { content?: string } }[]
      | undefined;
    const delta = choices?.[0]?.delta?.content;
    if (delta) cb.onChunk(delta);
  };

  try {
    for (; ;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buffer.indexOf("\n\n")) !== -1) {
        const frame = buffer.slice(0, idx);
        buffer = buffer.slice(idx + 2);
        handleFrame(frame);
      }
    }
    // Tail: flush any trailing frame without a blank line terminator.
    if (buffer.trim().length > 0) handleFrame(buffer);
  } catch (e) {
    cb.onError(e instanceof Error ? e.message : "stream interrupted");
  }
}

