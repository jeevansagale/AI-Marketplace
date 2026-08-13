"use client";

import { Suspense, useEffect, useMemo, useRef, useState, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import {
  Play, Square, Loader2, Wallet, Zap, Activity, CheckCircle2,
  Clock, ShieldCheck,
} from "lucide-react";
import { Shell } from "@/components/Shell";
import { Button, Card, Badge, Skeleton, EmptyState, Label, Field, Textarea, Select } from "@/components/ui";
import { api, streamSession, type MarketModel, type Session } from "@/lib/api";
import { formatINR, estimateSessionCost, stageLabel, providerBadge } from "@/lib/format";
import { useWalletClient } from "@/lib/stores/useWallet";
import { StaggeredItem } from "@/components/PageTransition";

const STAGE_ORDER = ["requested", "authorized", "reserved", "streaming", "completed", "settled", "refunded"];

function useCatalog() {
  const [models, setModels] = useState<MarketModel[] | null>(null);
  useEffect(() => {
    api.catalog()
      .then((c) => setModels(c.models.filter((m) => m.is_active)))
      .catch(() => setModels([]));
  }, []);
  return models;
}

function SessionTimeline({ session }: { session: Session }) {
  const reached = new Set(session.timeline.map((e) => e.stage));

  return (
    <ol className="relative space-y-0">
      {STAGE_ORDER.map((stage) => {
        const event = session.timeline.find((e) => e.stage === stage);
        const done = reached.has(stage);
        const active = session.stage === stage;
        return (
          <li key={stage} className="relative flex gap-3 pb-5 last:pb-0">
            {stage !== "refunded" && (
              <span className="absolute left-[11px] top-6 h-[calc(100%-20px)] w-px bg-base-700" />
            )}
            <span
              className={`relative z-10 mt-0.5 flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full border ${
                done
                  ? "border-pos-400/40 bg-pos-400/10 text-pos-400"
                  : active
                    ? "border-warn-400/50 bg-warn-400/10 text-warn-400"
                    : "border-base-700 bg-base-900 text-base-500"
              }`}
            >
              {done ? <CheckCircle2 className="h-3.5 w-3.5" /> :
                active ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> :
                <Clock className="h-3.5 w-3.5" />}
            </span>
            <div className="min-w-0 pt-0.5">
              <p className={`text-[13px] font-medium ${done ? "text-base-100" : active ? "text-warn-400" : "text-base-400"}`}>
                {stageLabel(stage)}
              </p>
              {event?.note && (
                <p className="mt-0.5 truncate font-mono text-[11px] text-base-400">{event.note}</p>
              )}
              {event && (
                <p className="mt-0.5 text-[11px] text-base-500">
                  {new Date(event.at).toLocaleTimeString("en-IN", { hour12: false })}
                </p>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function PlaygroundPage() {
  const searchParams = useSearchParams();
  const models = useCatalog();
  const { balance, refresh } = useWalletClient();

  const [model, setModel] = useState<string>("");
  const [temp, setTemp] = useState(0.7);
  const [maxTokens, setMaxTokens] = useState(512);
  const [prompt, setPrompt] = useState("");
  const [output, setOutput] = useState("");
  const [session, setSession] = useState<Session | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [summary, setSummary] = useState<{
    actual: number; refunded: number;
    prompt: number; completion: number; latency: number;
  } | null>(null);
  const outRef = useRef<HTMLDivElement>(null);

  // Hydrate model from query param (marketplace "Use" button).
  useEffect(() => {
    if (models && models.length > 0 && !model) {
      const q = searchParams.get("model");
      if (q && models.some((m) => m.name === q)) setModel(q);
      else setModel(models[0].name);
    }
  }, [models, model, searchParams]);

  const selectedModel = useMemo(
    () => models?.find((m) => m.name === model) ?? null,
    [models, model],
  );

  // Live session refresh (poll) while running — timeline updates from stages.
  useEffect(() => {
    if (!running || !session) return;
    const t = setInterval(() => {
      // The gateway updates stages during streaming; polls keep us in sync
      // with settled/refund states after completion.
      void refresh();
    }, 2000);
    return () => clearInterval(t);
  }, [running, session, refresh]);

  const start = useCallback(async () => {
    if (!model || !prompt.trim()) {
      setError("Select a model and enter a prompt.");
      return;
    }
    setError(null);
    setOutput("");
    setSummary(null);
    setRunning(true);

    try {
      // 1. Reserve estimated cost (402-inspired wallet authorization).
      const sess = await api.createSession(model, maxTokens);
      setSession(sess);
      await refresh();

      // 2. Stream with metering/settlement/refund.
      const messages = [{ role: "user", content: prompt }];
      await streamSession(sess.id, messages, maxTokens, {
        onChunk: (text) => {
          setOutput((prev) => prev + text);
          requestAnimationFrame(() => outRef.current?.scrollTo({ top: outRef.current.scrollHeight }));
        },
        onDone: (s) => {
          setSummary({
            actual: s.actual_cost_paise,
            refunded: s.refunded_paise,
            prompt: s.prompt_tokens,
            completion: s.completion_tokens,
            latency: s.latency_ms,
          });
          setRunning(false);
          void refresh();
        },
        onError: (m) => {
          setError(m);
          setRunning(false);
          void refresh();
        },
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Session failed");
      setRunning(false);
      void refresh();
    }
  }, [model, prompt, maxTokens, refresh]);

  const cancel = () => {
    setRunning(false);
    setError("Stream cancelled. The reservation is automatically released.");
    void refresh();
  };

  const reserve = selectedModel ? estimateSessionCost(selectedModel, maxTokens) : 0;

  return (
    <Shell>
      <StaggeredItem index={0}>
        <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-base-100">Playground</h1>
            <p className="mt-1 text-sm text-base-400">
              Pay-per-session AI inference. Funds are reserved, consumed, and refunded automatically.
            </p>
          </div>
          {selectedModel && (
            <Badge tone="blue" dot>{providerBadge(selectedModel.provider_name)} · {selectedModel.name}</Badge>
          )}
        </div>
      </StaggeredItem>

      <StaggeredItem index={1}>
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-[300px_minmax(0,1fr)_310px]">
          {/* ---- Left sidebar: model + config + reserve ---- */}
          <Card className="h-fit overflow-hidden lg:sticky lg:top-20">
            <div className="border-b border-base-700 px-5 py-4">
              <h2 className="text-sm font-semibold text-base-100">Session</h2>
            </div>
            <div className="space-y-4 px-5 py-4">
              {/* Model select */}
              <Field>
                <Label>Model</Label>
                <Select
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  disabled={running}
                >
                  {models === null && <option>Loading…</option>}
                  {models?.map((m) => (
                    <option key={m.id} value={m.name}>{m.name}</option>
                  ))}
                </Select>
              </Field>

              {/* Temperature */}
              <Field>
                <Label>
                  <span className="flex items-center justify-between">
                    <span>Temperature</span>
                    <span className="font-mono text-base-200">{temp.toFixed(1)}</span>
                  </span>
                </Label>
                <input
                  type="range" min={0} max={1} step={0.1} value={temp}
                  onChange={(e) => setTemp(Number(e.target.value))}
                  disabled={running}
                  className="mt-2 h-1.5 w-full accent-accent-500 disabled:opacity-50"
                />
              </Field>

              {/* Max tokens */}
              <Field>
                <Label>
                  <span className="flex items-center justify-between">
                    <span>Max tokens</span>
                    <span className="font-mono text-base-200">{maxTokens}</span>
                  </span>
                </Label>
                <input
                  type="range" min={64} max={2048} step={64} value={maxTokens}
                  onChange={(e) => setMaxTokens(Number(e.target.value))}
                  disabled={running}
                  className="mt-2 h-1.5 w-full accent-accent-500 disabled:opacity-50"
                />
              </Field>

              {/* Cost estimate + reserve */}
              {selectedModel && (
                <div className="space-y-2 rounded-md border border-base-700 bg-base-850 p-3">
                  <div className="flex items-center justify-between text-[13px]">
                    <span className="text-base-400">Est. session cost</span>
                    <span className="font-mono font-medium text-base-100">{formatINR(reserve)}</span>
                  </div>
                  <div className="flex items-center justify-between text-[13px]">
                    <span className="text-base-400">Session reserve</span>
                    <span className="font-mono font-medium text-warn-400">{formatINR(reserve)}</span>
                  </div>
                   <div className="flex items-center justify-between border-t border-base-700 pt-2 text-[13px]">
                     <span className="flex items-center gap-1.5 text-base-400">
                       <Wallet className="h-3.5 w-3.5" /> Available
                     </span>
                     <span className="font-mono font-medium text-pos-400">
                       {balance ? formatINR(balance.available_paise) : "—"}
                     </span>
                   </div>
                </div>
              )}

              {/* Prompt */}
              <Field>
                <Label>Prompt</Label>
                <Textarea
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  disabled={running}
                  rows={5}
                  placeholder="Ask anything…"
                />
              </Field>

              <div className="flex gap-2">
                <Button className="flex-1" onClick={start} loading={running && !session}>
                  <Play className="h-4 w-4" /> {running ? "Streaming…" : "Start session"}
                </Button>
                {running && (
                  <Button variant="secondary" onClick={cancel} aria-label="Stop">
                    <Square className="h-4 w-4" />
                  </Button>
                )}
              </div>

              {error && (
                <p className="rounded-lg border border-neg-400/20 bg-neg-400/10 px-3 py-2 text-xs text-neg-400">
                  {error}
                </p>
              )}
            </div>
          </Card>

          {/* ---- Center: output ---- */}
          <Card className="flex min-h-[520px] flex-col overflow-hidden">
            <div className="flex items-center justify-between border-b border-base-700 px-5 py-4">
              <div className="flex items-center gap-2">
                <Zap className="h-4 w-4 text-base-400" />
                <h2 className="text-sm font-semibold text-base-100">Streaming output</h2>
              </div>
              {running && <Badge tone="amber" dot>Live</Badge>}
            </div>
            <div
              ref={outRef}
              className="flex-1 overflow-y-auto bg-base-900 p-5"
              style={{ maxHeight: "560px" }}
            >
              {output ? (
                <div className="whitespace-pre-wrap font-mono text-[13px] leading-6 text-base-100">
                  {output}
                </div>
              ) : running ? (
                <div className="space-y-3">
                  <div className="flex items-center gap-2 text-sm text-base-400">
                    <Loader2 className="h-4 w-4 animate-spin" /> Waiting for tokens…
                  </div>
                  <Skeleton className="h-3 w-11/12" />
                  <Skeleton className="h-3 w-8/12" />
                </div>
              ) : (
                <div className="flex h-full items-center justify-center">
                  <EmptyState
                    icon={<Zap className="h-8 w-8" />}
                    title="Run a session to see streaming output"
                    body="Your prompt is billed at the metered rate and refunded automatically."
                  />
                </div>
              )}
            </div>
            {summary && (
              <div className="flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-base-700 bg-base-850 px-5 py-3 text-xs text-base-400">
                <span>Charged <b className="font-mono text-base-200">{formatINR(summary.actual)}</b></span>
                {summary.refunded > 0 && (
                  <span>Refunded <b className="font-mono text-pos-400">+{formatINR(summary.refunded)}</b></span>
                )}
                <span>{summary.prompt} prompt tokens</span>
                <span>{summary.completion} completion</span>
                <span className="font-mono">{summary.latency} ms</span>
              </div>
            )}
          </Card>

          {/* ---- Right sidebar: timeline + recent activity ---- */}
          <div className="space-y-5 lg:sticky lg:top-20 lg:h-fit">
            <Card className="overflow-hidden">
              <div className="flex items-center justify-between border-b border-base-700 px-5 py-4">
                <div className="flex items-center gap-2">
                  <Activity className="h-4 w-4 text-base-400" />
                  <h2 className="text-sm font-semibold text-base-100">Session timeline</h2>
                </div>
                {session && <Badge tone={session.stage === "failed" ? "red" : "blue"}>{session.stage}</Badge>}
              </div>
              <div className="px-5 py-4">
                {session ? (
                  <SessionTimeline session={session} />
                ) : (
                  <p className="text-xs leading-relaxed text-base-400">Start a session to see the lifecycle: request → 402 → reserve → stream → settle → refund.</p>
                )}
              </div>
            </Card>

            <RecentActivity />
          </div>
        </div>
      </StaggeredItem>
    </Shell>
  );
}

/* Main export: wrapped in Suspense for useSearchParams CSR bailout. */
export default function Playground() {
  return (
    <Suspense
      fallback={
        <Shell>
          <div className="flex h-96 items-center justify-center">
            <Loader2 className="h-6 w-6 animate-spin text-base-400" />
          </div>
        </Shell>
      }
    >
      <PlaygroundPage />
    </Suspense>
  );
}

/* ---- Recent sessions (right sidebar bottom: transaction timeline) ---- */
function RecentActivity() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [balance, setBalance] = useState<number | null>(null);

  useEffect(() => {
    let mounted = true;
    const load = async () => {
      try {
        const [s, b] = await Promise.all([api.sessions(), api.wallet()]);
        if (!mounted) return;
        setSessions(s.sessions.slice(0, 5));
        setBalance(b.available_paise);
      } catch { /* ignore */ }
    };
    void load();
    const t = setInterval(load, 5000);
    return () => { mounted = false; clearInterval(t); };
  }, []);

  return (
    <Card>
      <div className="flex items-center justify-between border-b border-base-700 px-5 py-4">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4 text-base-400" />
          <h2 className="text-sm font-semibold text-base-100">Recent activity</h2>
        </div>
        {balance !== null && (
          <span className="font-mono text-xs text-base-400">{formatINR(balance)} available</span>
        )}
      </div>
      <div className="px-5 py-2">
        {sessions.length === 0 ? (
          <p className="py-4 text-xs text-base-400">No sessions yet.</p>
        ) : (
          <ul className="divide-y divide-base-200">
            {sessions.map((s) => (
              <li key={s.id} className="flex items-center justify-between py-2.5">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-base-800 text-[10px] font-semibold text-base-300">
                    {providerBadge(s.provider_name).slice(0, 2)}
                  </div>
                  <div>
                    <p className="text-[13px] font-medium text-base-100">{s.model_name}</p>
                    <p className="text-[11px] text-base-500">{s.stage} · {new Date(s.created_at).toLocaleTimeString("en-IN", { hour12: false })}</p>
                  </div>
                </div>
                <span className={`font-mono text-[13px] ${
                  s.stage === "failed" ? "text-neg-400" :
                  s.actual_cost_paise > 0 ? "text-base-200" : "text-base-400"
                }`}>
                  −{formatINR(s.actual_cost_paise || 0)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}
