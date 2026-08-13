// Quantum Token Ledger — payment infrastructure visualization.
// A frontend-only simulation of authorize → route → meter → settle → provider,
// built with the site's existing tokens (base-*, accent-*, Card/Button primitives).
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { RotateCcw, Radio, Lock, CheckCircle2 } from "lucide-react";
import { Card, Badge, Button } from "@/components/ui";

/* ------------------------------------------------------------------ */
/* Network topology — hand-placed for an organic, non-grid feel.       */
/* ------------------------------------------------------------------ */

type NodeKind = "hub" | "generic" | "provider";

interface LedgerNode {
  x: number;
  y: number;
  label?: string;
  kind: NodeKind;
}

const NODES: LedgerNode[] = [
  { x: 68, y: 176, label: "GATEWAY", kind: "hub" },
  { x: 132, y: 84, label: "04", kind: "generic" },
  { x: 146, y: 268, label: "09", kind: "generic" },
  { x: 224, y: 150, label: "ESCROW", kind: "hub" },
  { x: 236, y: 238, label: "11", kind: "generic" },
  { x: 302, y: 64, label: "15", kind: "generic" },
  { x: 292, y: 298, label: "NODE 07", kind: "generic" },
  { x: 366, y: 176, label: "18", kind: "generic" },
  { x: 416, y: 108, label: "SETTLEMENT", kind: "hub" },
  { x: 420, y: 246, label: "22", kind: "generic" },
  { x: 494, y: 188, label: "NODE 12", kind: "generic" },
  { x: 556, y: 150, label: "PROVIDER", kind: "provider" },
  { x: 248, y: 36, kind: "generic" },
  { x: 476, y: 314, kind: "generic" },
];

const EDGES: [number, number][] = [
  [0, 1], [0, 2], [0, 3],
  [1, 3], [1, 5], [1, 12],
  [2, 3], [2, 4], [2, 6],
  [3, 4], [3, 5], [3, 8],
  [4, 6], [4, 9],
  [5, 8], [5, 12],
  [6, 7], [6, 9],
  [7, 8], [7, 10],
  [8, 9], [8, 10], [8, 13], [8, 11],
  [9, 10], [9, 13],
  [10, 11], [10, 13],
];

// The illuminated route during "routing" — every consecutive pair exists in EDGES.
const ACTIVE_PATH = [0, 3, 4, 9, 8, 10, 11];
const ACTIVE_EDGE_KEYS = new Set(
  ACTIVE_PATH.slice(0, -1).map((n, i) => edgeKey(n, ACTIVE_PATH[i + 1])),
);

function edgeKey(a: number, b: number) {
  return a < b ? `${a}-${b}` : `${b}-${a}`;
}

/* ------------------------------------------------------------------ */
/* State machine                                                       */
/* ------------------------------------------------------------------ */

type Phase = "idle" | "authorizing" | "routing" | "metering" | "settling" | "complete";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

const METER_STEPS = [
  { tokens: 1024, paise: 28 },
  { tokens: 2048, paise: 35 },
  { tokens: 3112, paise: 42 },
];

const AUTH_LABELS = ["AUTHORIZING", "402 PAYMENT REQUIRED", "PAYMENT AUTHORIZED"];

export function QuantumTokenLedger() {
  const reduceMotion = useReducedMotion();
  const mounted = useRef(true);
  const runIdRef = useRef(0);

  const [phase, setPhase] = useState<Phase>("idle");
  const [authLabel, setAuthLabel] = useState<string>("READY");
  const [meterIdx, setMeterIdx] = useState(0);
  const [running, setRunning] = useState(false);
  const [pathLit, setPathLit] = useState(0); // how many nodes on the active path are lit

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const stale = useCallback((id: number) => !mounted.current || id !== runIdRef.current, []);

  const play = useCallback(async () => {
    const id = ++runIdRef.current;
    setRunning(true);
    setPhase("authorizing");
    setAuthLabel("AUTHORIZING");

    const authGap = reduceMotion ? 60 : 130;
    for (const label of AUTH_LABELS.slice(1)) {
      await sleep(authGap);
      if (stale(id)) return;
      setAuthLabel(label);
    }
    await sleep(reduceMotion ? 40 : 90);
    if (stale(id)) return;

    setPhase("routing");
    const routeDur = reduceMotion ? 160 : 420;
    const perNode = routeDur / ACTIVE_PATH.length;
    for (let i = 1; i <= ACTIVE_PATH.length; i++) {
      await sleep(perNode);
      if (stale(id)) return;
      setPathLit(i);
    }

    setPhase("metering");
    const meterGap = reduceMotion ? 70 : 140;
    for (let i = 0; i < METER_STEPS.length; i++) {
      setMeterIdx(i);
      await sleep(meterGap);
      if (stale(id)) return;
    }

    setPhase("settling");
    await sleep(reduceMotion ? 120 : 280);
    if (stale(id)) return;

    setPhase("complete");
    setRunning(false);
  }, [reduceMotion, stale]);

  const reset = useCallback(() => {
    runIdRef.current++; // invalidate any in-flight sequence
    setPhase("idle");
    setAuthLabel("READY");
    setMeterIdx(0);
    setPathLit(0);
    setRunning(false);
  }, []);

  const handleReplay = useCallback(() => {
    reset();
    requestAnimationFrame(() => void play());
  }, [reset, play]);

  const cost = METER_STEPS[meterIdx].paise;
  const tokens = METER_STEPS[meterIdx].tokens;

  const statusTone: "gray" | "blue" | "amber" | "green" =
    phase === "idle"
      ? "gray"
      : phase === "complete"
        ? "green"
        : phase === "settling"
          ? "amber"
          : "blue";

  const statusText: Record<Phase, string> = {
    idle: "READY",
    authorizing: authLabel,
    routing: "ROUTING",
    metering: "METERING",
    settling: "SETTLEMENT VERIFIED",
    complete: "PAYMENT SETTLED",
  };

  return (
    <section aria-labelledby="qtl-heading" className="pt-2">
      <Badge tone="blue" className="mb-3 inline-flex">
        <span className="flex items-center gap-1.5">
          <Radio className="h-3 w-3" />
          PAYMENT INFRASTRUCTURE
        </span>
      </Badge>
      <h2 id="qtl-heading" className="max-w-2xl text-2xl font-semibold leading-tight text-base-100 sm:text-3xl text-balance">
        Every AI request, settled with precision.
      </h2>
      <p className="mt-2 max-w-xl text-[14px] leading-relaxed text-base-400">
        Each session is authorized against wallet balance, routed through the
        settlement network, metered against live token usage, and settled to
        the provider — reconciled automatically, down to the paisa.
      </p>

      <Card className="mt-6 overflow-hidden p-0">
        <div className="grid grid-cols-1 lg:grid-cols-[300px_1fr]">
          {/* Transaction / state panel */}
          <div className="flex flex-col justify-between gap-6 border-b border-base-700 p-5 lg:border-b-0 lg:border-r">
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="shrink-0 text-xs font-medium text-base-400">AI INFERENCE</span>
                <Badge tone={statusTone} dot className="shrink-0 whitespace-nowrap">
                  <span aria-live="polite">{statusText[phase]}</span>
                </Badge>
              </div>

              <p className="mt-3 font-mono text-2xl font-semibold tabular-nums text-base-100">
                ₹{(cost / 100).toFixed(2)}
              </p>
              <p className="mt-0.5 font-mono text-[11px] text-base-500">
                {tokens.toLocaleString("en-IN")} tokens · MODEL Gemini
              </p>

              <dl className="mt-5 space-y-2.5 border-t border-base-700 pt-4">
                <Row label="ENDPOINT" value="POST /v1/chat" />
                <Row label="NETWORK" value="Quantum Ledger" />
                <Row label="LATENCY" value={phase === "complete" ? "4.8ms" : "—"} />
              </dl>
            </div>

            <div className="flex items-center gap-2">
              {phase !== "complete" ? (
                <Button className="w-full" onClick={() => void play()} disabled={running}>
                  Simulate payment
                </Button>
              ) : (
                <Button variant="secondary" className="w-full" onClick={handleReplay}>
                  <RotateCcw className="h-3.5 w-3.5" />
                  Replay
                </Button>
              )}
            </div>
          </div>

          {/* Network visualization */}
          <div className="relative min-h-[280px] bg-[#0a0e1a] sm:min-h-[340px]">
            <div
              className="pointer-events-none absolute inset-0 opacity-[0.06]"
              style={{
                backgroundImage:
                  "linear-gradient(to right, #7c9bff 1px, transparent 1px), linear-gradient(to bottom, #7c9bff 1px, transparent 1px)",
                backgroundSize: "28px 28px",
              }}
              aria-hidden
            />
            <NetworkGraph phase={phase} pathLit={pathLit} reduceMotion={!!reduceMotion} />
          </div>
        </div>
      </Card>
    </section>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <dt className="text-[11px] text-base-500">{label}</dt>
      <dd className="font-mono text-[11px] text-base-300">{value}</dd>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* SVG network graph                                                    */
/* ------------------------------------------------------------------ */

function NetworkGraph({
  phase,
  pathLit,
  reduceMotion,
}: {
  phase: Phase;
  pathLit: number;
  reduceMotion: boolean;
}) {
  const litIndices = new Set(ACTIVE_PATH.slice(0, pathLit));
  const consensus = phase === "settling" || phase === "complete";
  const settled = phase === "complete";

  return (
    <svg
      viewBox="0 0 624 360"
      className="h-full w-full"
      role="img"
      aria-label={`Payment network — ${phase === "idle" ? "idle" : phase}`}
    >
      <defs>
        <radialGradient id="qtl-glow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="#4f6ef7" stopOpacity="0.35" />
          <stop offset="100%" stopColor="#4f6ef7" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="qtl-edge-active" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#22d3ee" />
          <stop offset="100%" stopColor="#8b5cf6" />
        </linearGradient>
      </defs>

      {/* ambient glow */}
      <circle cx="312" cy="180" r="220" fill="url(#qtl-glow)" />

      {/* dim mesh */}
      <g>
        {EDGES.map(([a, b]) => {
          const key = edgeKey(a, b);
          const active = ACTIVE_EDGE_KEYS.has(key);
          const na = NODES[a];
          const nb = NODES[b];
          const isLit = active && litIndices.has(a) && litIndices.has(b);
          return (
            <line
              key={key}
              x1={na.x}
              y1={na.y}
              x2={nb.x}
              y2={nb.y}
              stroke={isLit ? "url(#qtl-edge-active)" : "#26314f"}
              strokeWidth={isLit ? 1.6 : 1}
              opacity={isLit ? 0.9 : active ? 0.35 : reduceMotion ? 0.5 : undefined}
              className={!reduceMotion && !active ? "qtl-breathe" : undefined}
              style={
                !reduceMotion && !active
                  ? { animationDelay: `${((a * 7 + b * 3) % 20) / 10}s` }
                  : undefined
              }
            />
          );
        })}
      </g>

      {/* consensus ring */}
      {consensus && (
        <motion.circle
          cx="312"
          cy="180"
          r="40"
          fill="none"
          stroke="#22d3ee"
          strokeWidth="1.5"
          initial={{ opacity: 0.9, r: 30 }}
          animate={reduceMotion ? { opacity: 0.5, r: 150 } : { opacity: 0, r: 190 }}
          transition={{ duration: reduceMotion ? 0.3 : 0.7, ease: [0.16, 1, 0.3, 1] }}
        />
      )}

      {/* nodes */}
      <g>
        {NODES.map((n, i) => {
          const lit = litIndices.has(i);
          const isProvider = n.kind === "provider";
          const isHub = n.kind === "hub";
          const radius = isProvider ? 6 : isHub ? 4.5 : 3;
          const providerGlow = isProvider && (settled || lit);

          return (
            <g key={i} className={!reduceMotion ? "qtl-drift" : undefined} style={!reduceMotion ? { animationDelay: `${(i % 7) * 0.35}s` } : undefined}>
              {isProvider ? (
                <rect
                  x={n.x - radius}
                  y={n.y - radius}
                  width={radius * 2}
                  height={radius * 2}
                  transform={`rotate(45 ${n.x} ${n.y})`}
                  fill={providerGlow ? "#22d3ee" : "#161f36"}
                  stroke={providerGlow ? "#22d3ee" : "#3a4568"}
                  strokeWidth="1"
                />
              ) : (
                <circle
                  cx={n.x}
                  cy={n.y}
                  r={radius}
                  fill={lit ? "#22d3ee" : isHub ? "#1c2744" : "#141b30"}
                  stroke={lit ? "#67e8f9" : isHub ? "#3a4568" : "#26314f"}
                  strokeWidth="1"
                />
              )}
              {lit && !reduceMotion && (
                <circle cx={n.x} cy={n.y} r={radius + 4} fill="none" stroke="#22d3ee" opacity="0.5">
                  <animate attributeName="r" from={radius} to={radius + 10} dur="0.6s" begin="0s" fill="freeze" />
                  <animate attributeName="opacity" from="0.6" to="0" dur="0.6s" fill="freeze" />
                </circle>
              )}
              {n.label && (isHub || isProvider || lit) && (
                <text
                  x={n.x}
                  y={n.y - radius - 6}
                  textAnchor="middle"
                  fontSize="8"
                  fontFamily="var(--font-mono)"
                  letterSpacing="0.5"
                  fill={lit || isHub || isProvider ? "#8fa3d1" : "#3a4568"}
                  opacity={lit || isHub ? 0.85 : 0.4}
                >
                  {n.label}
                </text>
              )}
            </g>
          );
        })}
      </g>

      <style>{`
        .qtl-drift { animation: qtl-drift 6s ease-in-out infinite; }
        @keyframes qtl-drift {
          0%, 100% { transform: translate(0px, 0px); }
          50% { transform: translate(0px, -3px); }
        }
        .qtl-breathe { animation: qtl-breathe 4.5s ease-in-out infinite; }
        @keyframes qtl-breathe {
          0%, 100% { opacity: 0.22; }
          50% { opacity: 0.4; }
        }
      `}</style>
    </svg>
  );
}
