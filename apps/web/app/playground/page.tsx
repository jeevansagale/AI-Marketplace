"use client";

import { Suspense, useEffect, useMemo, useRef, useState, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import {
  Play, Square, Loader2, Wallet, Zap, Activity, CheckCircle2,
  Clock, ShieldCheck, RefreshCw, Image as ImageIcon, Download, MessageSquare
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { Shell } from "@/components/Shell";
import { Button, Card, Badge, Skeleton, EmptyState, Label, Field, Textarea, Select } from "@/components/ui";
import { api, streamSession, generateImageSession, type MarketModel, type Session } from "@/lib/api";
import { formatINR, estimateSessionCost, stageLabel, providerBadge } from "@/lib/format";
import { useWalletClient } from "@/lib/stores/useWallet";
import { StaggeredItem } from "@/components/PageTransition";
import { ProviderRoutingAnimation, type ProviderRouteResult } from "@/components/ProviderRoutingAnimation";
import { SessionMeter, type SessionMetrics } from "@/components/SessionMeter";
import { X402FlowVisualizer, type X402Stage } from "@/components/X402FlowVisualizer";

const EASE = [0.16, 1, 0.3, 1] as const;
const STAGE_ORDER = ["requested", "authorized", "reserved", "streaming", "generating", "completed", "settled", "refunded"];

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

/** Maps session stage to the X402Stage enum used by the visualizer */
function toX402Stage(sessionStage: string | undefined): X402Stage {
  switch (sessionStage) {
    case "reserved":    return "reserved";
    case "streaming":
    case "generating":  return "streaming";
    case "completed":   return "completed";
    case "settled":     return "settled";
    case "refunded":    return "refunded";
    case "closed":      return "closed";
    default:            return "idle";
  }
}

function PlaygroundPage() {
  const searchParams = useSearchParams();
  const models = useCatalog();
  const { balance, refresh } = useWalletClient();

  const [model, setModel] = useState<string>("");
  const [mode, setMode] = useState<"auto" | "chat" | "image">("auto");
  const [temp, setTemp] = useState(0.7);
  const [maxTokens, setMaxTokens] = useState(512);
  const [prompt, setPrompt] = useState("");
  const [output, setOutput] = useState("");
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [session, setSession] = useState<Session | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [summary, setSummary] = useState<{
    actual: number; refunded: number;
    prompt: number; completion: number; latency: number;
    image_url?: string;
  } | null>(null);
  const [activityRefreshKey, setActivityRefreshKey] = useState(0);
  const outRef = useRef<HTMLDivElement>(null);

  // ── Provider routing animation state ──────────────────────────────────────
  const [routeResult, setRouteResult] = useState<ProviderRouteResult | null>(null);
  const [routingDone, setRoutingDone] = useState(false);

  // ── Live session meter state ───────────────────────────────────────────────
  const [meterMetrics, setMeterMetrics] = useState<SessionMetrics | null>(null);

  // Hydrate model from query param (marketplace "Use" button).
  useEffect(() => {
    if (models && models.length > 0 && !model) {
      const q = searchParams.get("model");
      const matched = models.find((m) => m.slug === q || m.name === q);
      if (matched) setModel(matched.slug);
      else setModel(models[0].slug);
    }
  }, [models, model, searchParams]);

  const selectedModel = useMemo(
    () => models?.find((m) => m.slug === model || m.name === model) ?? null,
    [models, model],
  );

  const effectiveMode = useMemo(() => {
    if (mode !== "auto") return mode;
    if (selectedModel?.supports_images) return "image";
    return "chat";
  }, [mode, selectedModel]);

  // While running, refresh the wallet balance every 2 s.
  useEffect(() => {
    if (!running || !session) return;
    const t = setInterval(() => {
      void refresh();
    }, 2000);
    return () => clearInterval(t);
  }, [running, session, refresh]);

  // Update meter metrics when session/summary changes
  useEffect(() => {
    if (!session || !selectedModel) return;
    setMeterMetrics({
      providerName: session.provider_name ?? selectedModel.provider,
      modelName: session.model_name ?? selectedModel.name,
      modelSlug: session.model_slug ?? model,
      reservedPaise: session.estimated_paise ?? 0,
      actualPaise: summary?.actual ?? 0,
      promptTokens: summary?.prompt ?? 0,
      completionTokens: summary?.completion ?? 0,
      latencyMs: summary?.latency ?? 0,
      settlementProgress: summary ? 1 : running ? 0.5 : 0,
      settled: !!summary && !running,
    });
  }, [session, summary, running, selectedModel, model]);

  const start = useCallback(async () => {
    if (!model || !prompt.trim()) {
      setError("Select a model and enter a prompt.");
      return;
    }
    setError(null);
    setOutput("");
    setImageUrl(null);
    setSummary(null);
    setRoutingDone(false);
    setRouteResult(null);
    setMeterMetrics(null);
    setRunning(true);

    try {
      // Always create a BRAND NEW session — closed sessions are never reused.
      const sess = await api.createSession(model, maxTokens);
      setSession(sess);
      setActivityRefreshKey((k) => k + 1);
      await refresh();

      // Show provider routing animation using catalog data for the selected model
      if (selectedModel) {
        setRouteResult({
          selectedProvider: selectedModel.provider,
          selectedModel: selectedModel.slug,
          modelName: selectedModel.name,
          latencyMs: selectedModel.p90_latency_ms ?? 380,
          pricePaise: selectedModel.price_base_paise ?? 10,
          contextWindow: selectedModel.context_window ?? 1048576,
        });
        // routingDone will be set by the animation's onDone callback
      } else {
        setRoutingDone(true);
      }

      if (effectiveMode === "image") {
        // Image generation execution
        const res = await generateImageSession(sess.id, prompt);
        if (res.image_url) {
          setImageUrl(res.image_url);
        }
        setSummary({
          actual: res.actual_cost_paise,
          refunded: res.refunded_paise,
          prompt: res.prompt_tokens,
          completion: res.completion_tokens,
          latency: res.latency_ms,
          image_url: res.image_url,
        });
        setRunning(false);
        setActivityRefreshKey((k) => k + 1);
        void refresh();
      } else {
        // Streaming text chat execution
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
            setActivityRefreshKey((k) => k + 1);
            void refresh();
          },
          onError: (m) => {
            setError(m);
            setRunning(false);
            setActivityRefreshKey((k) => k + 1);
            void refresh();
          },
        });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Session failed");
      setRunning(false);
      setRoutingDone(true);
      setActivityRefreshKey((k) => k + 1);
      void refresh();
    }
  }, [model, prompt, maxTokens, effectiveMode, refresh, selectedModel]);

  const cancel = () => {
    setRunning(false);
    setRoutingDone(true);
    setError("Operation cancelled. Wallet reservation released.");
    setActivityRefreshKey((k) => k + 1);
    void refresh();
  };

  const reserve = selectedModel ? estimateSessionCost(selectedModel, maxTokens) : 0;

  // Determine which panel to show in the output card center
  const showRoutingAnimation = routeResult && !routingDone && running;
  const showOutput = routingDone || !routeResult;

  return (
    <Shell>
      <StaggeredItem index={0}>
        <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-base-100">Playground</h1>
            <p className="mt-1 text-sm text-base-400">
              Pay-per-session AI chat &amp; image generation. Funds are reserved, consumed, and settled per request.
            </p>
          </div>
          {selectedModel && (
            <Badge tone="blue" dot>{providerBadge(selectedModel.provider)} · {selectedModel.name}</Badge>
          )}
        </div>
      </StaggeredItem>

      <StaggeredItem index={1}>
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-[300px_minmax(0,1fr)_310px]">
          {/* ---- Left sidebar: model + config + reserve ---- */}
          <Card className="h-fit overflow-hidden lg:sticky lg:top-20">
            <div className="border-b border-base-700 px-5 py-4">
              <h2 className="text-sm font-semibold text-base-100">Session Config</h2>
            </div>
            <div className="space-y-4 px-5 py-4">
              {/* Mode selector */}
              <Field>
                <Label>Execution Mode</Label>
                <div className="grid grid-cols-3 gap-1 rounded-md border border-base-700 bg-base-900 p-1 text-xs">
                  <button
                    type="button"
                    onClick={() => setMode("auto")}
                    className={`rounded py-1 font-medium transition ${mode === "auto" ? "bg-accent-500/20 text-accent-400" : "text-base-400 hover:text-base-200"}`}
                  >
                    Auto
                  </button>
                  <button
                    type="button"
                    onClick={() => setMode("chat")}
                    className={`flex items-center justify-center gap-1 rounded py-1 font-medium transition ${mode === "chat" ? "bg-accent-500/20 text-accent-400" : "text-base-400 hover:text-base-200"}`}
                  >
                    <MessageSquare className="h-3 w-3" /> Chat
                  </button>
                  <button
                    type="button"
                    onClick={() => setMode("image")}
                    className={`flex items-center justify-center gap-1 rounded py-1 font-medium transition ${mode === "image" ? "bg-accent-500/20 text-accent-400" : "text-base-400 hover:text-base-200"}`}
                  >
                    <ImageIcon className="h-3 w-3" /> Image
                  </button>
                </div>
              </Field>

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
                    <option key={m.id} value={m.slug}>{m.name} {m.supports_images ? "🎨" : ""}</option>
                  ))}
                </Select>
              </Field>

              {/* Temperature & Tokens (for text mode) */}
              {effectiveMode === "chat" && (
                <>
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
                </>
              )}

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
                <Label>{effectiveMode === "image" ? "Image Prompt" : "Prompt"}</Label>
                <Textarea
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  disabled={running}
                  rows={4}
                  placeholder={effectiveMode === "image" ? "Describe the image to generate…" : "Ask anything…"}
                />
              </Field>

              <div className="flex gap-2">
                <Button className="flex-1" onClick={start} loading={running && !session}>
                  <Play className="h-4 w-4" /> {running ? (effectiveMode === "image" ? "Generating…" : "Streaming…") : (effectiveMode === "image" ? "Generate image" : "Start session")}
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

          {/* ---- Center: routing animation → output ---- */}
          <Card className="flex min-h-[520px] flex-col overflow-hidden">
            <div className="flex items-center justify-between border-b border-base-700 px-5 py-4">
              <div className="flex items-center gap-2">
                {effectiveMode === "image" ? <ImageIcon className="h-4 w-4 text-base-400" /> : <Zap className="h-4 w-4 text-base-400" />}
                <h2 className="text-sm font-semibold text-base-100">
                  {effectiveMode === "image" ? "Generated Image Result" : "Streaming Output"}
                </h2>
              </div>
              {running && <Badge tone="amber" dot>{effectiveMode === "image" ? "Generating" : "Live"}</Badge>}
            </div>
            <div
              ref={outRef}
              className="flex-1 overflow-y-auto bg-base-900 p-5"
              style={{ maxHeight: "560px" }}
            >
              {/* ── Provider routing animation (shown for ~1.5s on session start) ── */}
              <AnimatePresence mode="wait">
                {showRoutingAnimation && routeResult && (
                  <motion.div
                    key="routing"
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -6 }}
                    transition={{ duration: 0.25, ease: EASE }}
                    className="flex flex-col items-center justify-center min-h-[300px]"
                  >
                    <div className="w-full max-w-xs">
                      <p className="text-xs font-medium text-base-400 mb-3 text-center">
                        Routing request
                      </p>
                      <ProviderRoutingAnimation
                        result={routeResult}
                        onDone={() => setRoutingDone(true)}
                      />
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>

              {/* ── Main output content ── */}
              <AnimatePresence mode="wait">
                {showOutput && (
                  <motion.div
                    key="output"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.3, ease: EASE }}
                    className="h-full"
                  >
                    {effectiveMode === "image" ? (
                      imageUrl ? (
                        <div className="flex flex-col items-center justify-center gap-4">
                          {/* eslint-disable-next-line @next/next/no-img-element */}
                          <img
                            src={imageUrl}
                            alt="AI Generated Output"
                            className="max-h-[420px] rounded-lg border border-base-700 object-contain shadow-lg"
                          />
                          <a
                            href={imageUrl}
                            download="ai-generated-image.png"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-2 rounded-md bg-accent-500/10 px-4 py-2 text-xs font-medium text-accent-400 border border-accent-500/20 hover:bg-accent-500/20 transition"
                          >
                            <Download className="h-3.5 w-3.5" /> Download Generated Image
                          </a>
                        </div>
                      ) : running ? (
                        <div className="flex h-full flex-col items-center justify-center gap-3 text-base-400">
                          <Loader2 className="h-8 w-8 animate-spin text-accent-400" />
                          <p className="text-sm">Synthesizing image pixels…</p>
                        </div>
                      ) : (
                        <div className="flex h-full items-center justify-center">
                          <EmptyState
                            icon={<ImageIcon className="h-8 w-8" />}
                            title="Run an image session to generate AI art"
                            body="Image generation is metered per request and settled via your INR wallet."
                          />
                        </div>
                      )
                    ) : output ? (
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
                  </motion.div>
                )}
              </AnimatePresence>
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

          {/* ---- Right sidebar: session meter + timeline + x402 inspector ---- */}
          <div className="space-y-4 lg:sticky lg:top-20 lg:h-fit">

            {/* Live session meter (replaces static config display during sessions) */}
            <AnimatePresence>
              {meterMetrics && (
                <motion.div
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  transition={{ duration: 0.3, ease: EASE }}
                >
                  <SessionMeter metrics={meterMetrics} streaming={running} />
                </motion.div>
              )}
            </AnimatePresence>

            {/* Session timeline */}
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

            {/* x402 Protocol Inspector */}
            <X402FlowVisualizer
              stage={toX402Stage(session?.stage)}
              sessionData={session && summary ? {
                sessionId: session.id,
                reservedPaise: session.estimated_paise ?? 0,
                actualPaise: summary.actual,
                refundedPaise: summary.refunded,
                providerName: session.provider_name ?? "",
                modelName: session.model_name ?? "",
                latencyMs: summary.latency,
              } : undefined}
            />

            <RecentActivity refreshKey={activityRefreshKey} streaming={running} />
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

/* ---- Recent sessions (right sidebar bottom) ---- */

interface RecentActivityProps {
  refreshKey: number;
  streaming: boolean;
}

function RecentActivity({ refreshKey, streaming }: RecentActivityProps) {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [balance, setBalance] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      const [s, b] = await Promise.all([api.sessions(), api.wallet()]);
      setSessions(s.sessions.slice(0, 5));
      setBalance(b.available_paise);
    } catch { /* ignore */ } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshKey]);

  useEffect(() => {
    const interval = streaming ? 2000 : 5000;
    const t = setInterval(() => void load(), interval);
    return () => clearInterval(t);
  }, [streaming, load]);

  const stageTone = (stage: string) => {
    if (stage === "settled" || stage === "completed" || stage === "closed") return "text-pos-400";
    if (stage === "failed") return "text-neg-400";
    if (stage === "streaming" || stage === "generating") return "text-warn-400";
    return "text-base-400";
  };

  return (
    <Card>
      <div className="flex items-center justify-between border-b border-base-700 px-5 py-4">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4 text-base-400" />
          <h2 className="text-sm font-semibold text-base-100">Recent activity</h2>
        </div>
        <div className="flex items-center gap-3">
          {balance !== null && (
            <span className="font-mono text-xs text-base-400">{formatINR(balance)} avail.</span>
          )}
          <button
            onClick={() => void load()}
            className="rounded p-1 text-base-500 transition hover:text-base-200"
            title="Refresh"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          </button>
        </div>
      </div>
      <div className="px-5 py-2">
        {sessions.length === 0 ? (
          <p className="py-4 text-xs text-base-400">No sessions yet.</p>
        ) : (
          <ul className="divide-y divide-base-800">
            {sessions.map((s) => (
              <li key={s.id} className="flex items-center justify-between py-2.5">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-base-800 text-[10px] font-semibold text-base-300">
                    {providerBadge(s.provider_name).slice(0, 2).toUpperCase()}
                  </div>
                  <div>
                    <p className="text-[13px] font-medium text-base-100">{s.model_name}</p>
                    <p className={`text-[11px] ${stageTone(s.stage)}`}>
                      {s.stage} · {new Date(s.created_at).toLocaleTimeString("en-IN", { hour12: false })}
                    </p>
                  </div>
                </div>
                <span className={`font-mono text-[13px] ${
                  s.stage === "failed" ? "text-neg-400" :
                  s.actual_cost_paise > 0 ? "text-base-200" : "text-base-400"
                }`}>
                  {s.actual_cost_paise > 0 ? `−${formatINR(s.actual_cost_paise)}` : formatINR(s.estimated_paise)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}
