// Compact payment animation for the Top Up Wallet modal.
// Visual language only — no separate section, no new page. Lives entirely
// inside TopUpModal (see Shell.tsx) and is driven by the real payment
// promise (api.topup -> api.topupVerify), never a fake timer alone.
"use client";

import { useEffect, useMemo, useState } from "react";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { Wallet as WalletIcon, ShieldCheck } from "lucide-react";
import { formatINR } from "@/lib/format";

export type TopUpPhase = "processing" | "routing" | "verifying" | "success";

const EASE = [0.16, 1, 0.3, 1] as const;

/* Compact network — 7 nodes, thin connecting lines. Sized for a max-w-sm modal. */
type NetNode = { id: string; label?: string; x: number; y: number; main: boolean };

const NODES: NetNode[] = [
  { id: "wallet", label: "WALLET", x: 18, y: 62, main: true },
  { id: "n1", x: 62, y: 30, main: false },
  { id: "payment", label: "PAYMENT", x: 108, y: 62, main: true },
  { id: "n2", x: 154, y: 92, main: false },
  { id: "verify", label: "VERIFY", x: 200, y: 34, main: true },
  { id: "n3", x: 246, y: 62, main: false },
  { id: "provider", label: "PROVIDER", x: 292, y: 40, main: true },
];

const EDGES: [string, string][] = [
  ["wallet", "n1"],
  ["n1", "payment"],
  ["payment", "n2"],
  ["n2", "verify"],
  ["verify", "n3"],
  ["n3", "provider"],
  ["wallet", "payment"],
  ["verify", "provider"],
];

const TOKEN_PATH = ["wallet", "n1", "payment", "n2", "verify", "n3", "provider"];

const METADATA = ["SECURE CHANNEL", "VERIFYING PROVIDER", "SETTLEMENT"];

function nodeById(id: string) {
  return NODES.find((n) => n.id === id)!;
}

export function TopUpPaymentAnimation({
  amountPaise,
  phase,
  orderId,
}: {
  amountPaise: number;
  phase: TopUpPhase;
  orderId?: string;
}) {
  const reduceMotion = useReducedMotion();
  const [metaIndex, setMetaIndex] = useState(0);

  // Rotate tiny technical metadata while routing/verifying (purely decorative).
  useEffect(() => {
    if (phase === "processing" || phase === "success") return;
    const t = setInterval(() => setMetaIndex((i) => (i + 1) % METADATA.length), 700);
    return () => clearInterval(t);
  }, [phase]);

  const shortOrder = useMemo(
    () => (orderId ? orderId.slice(0, 4).toUpperCase() : "8F2A"),
    [orderId],
  );

  const statusLabel =
    phase === "processing"
      ? "Securing payment…"
      : phase === "routing"
        ? "Routing payment…"
        : phase === "verifying"
          ? "Verifying payment…"
          : "Payment verified";

  // --- Reduced-motion fallback: simple, understandable, no path/network motion.
  if (reduceMotion) {
    return (
      <div className="flex flex-col items-center gap-3 py-6">
        <AnimatePresence mode="wait">
          {phase !== "success" ? (
            <motion.div
              key="rm-progress"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="flex flex-col items-center gap-2"
            >
              <p className="font-mono text-lg font-semibold text-base-100">{formatINR(amountPaise)}</p>
              <p className="text-xs text-base-400" aria-live="polite">
                {statusLabel}
              </p>
            </motion.div>
          ) : (
            <motion.div
              key="rm-success"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="flex flex-col items-center gap-2"
            >
              <p className="font-mono text-lg font-semibold text-pos-400">+{formatINR(amountPaise)}</p>
              <p className="text-xs text-base-400" aria-live="polite">
                Wallet credited
              </p>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-3 py-2">
      {/* Amount / token focus */}
      <div className="relative flex h-16 items-center justify-center">
        <AnimatePresence mode="wait">
          {phase !== "success" ? (
            <motion.div
              key="amount"
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.85, filter: "blur(2px)" }}
              transition={{ duration: 0.25, ease: EASE }}
              className="relative flex items-center justify-center"
            >
              <span className="font-mono text-2xl font-semibold tabular-nums text-base-100">
                {formatINR(amountPaise)}
              </span>
              {/* subtle luminous ring around the amount as it becomes a token */}
              <motion.span
                className="pointer-events-none absolute -inset-3 rounded-full"
                style={{
                  boxShadow: "0 0 0 1px rgba(79,110,247,0.18), 0 0 24px 4px rgba(79,110,247,0.14)",
                }}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.35, ease: EASE }}
              />
            </motion.div>
          ) : (
            <motion.div
              key="credited"
              initial={{ opacity: 0, y: 6, scale: 0.92 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              transition={{ duration: 0.32, ease: EASE }}
              className="relative flex flex-col items-center"
            >
              <span className="font-mono text-2xl font-semibold tabular-nums text-pos-400">
                +{formatINR(amountPaise)}
              </span>
              <motion.span
                className="pointer-events-none absolute -inset-4 rounded-full"
                initial={{ opacity: 0.6, scale: 0.7 }}
                animate={{ opacity: 0, scale: 1.5 }}
                transition={{ duration: 0.6, ease: "easeOut" }}
                style={{ boxShadow: "0 0 0 1px rgba(22,163,74,0.35)" }}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Network visualization */}
      <div className="relative w-full">
        <svg
          viewBox="0 0 310 120"
          className="h-[92px] w-full sm:h-[104px]"
          role="img"
          aria-hidden="true"
        >
          {EDGES.map(([a, b], i) => {
            const na = nodeById(a);
            const nb = nodeById(b);
            // Hide the two decorative cross-links on very small screens.
            const secondary = i >= 6;
            return (
              <line
                key={`${a}-${b}`}
                x1={na.x}
                y1={na.y}
                x2={nb.x}
                y2={nb.y}
                stroke="rgb(203 213 225)"
                strokeWidth={1}
                className={secondary ? "hidden sm:block" : undefined}
                opacity={0.7}
              />
            );
          })}

          {/* Flow highlight along the main token path, sequenced with phase */}
          {phase !== "processing" &&
            TOKEN_PATH.slice(0, -1).map((id, i) => {
              const a = nodeById(id);
              const b = nodeById(TOKEN_PATH[i + 1]);
              return (
                <motion.line
                  key={`flow-${id}`}
                  x1={a.x}
                  y1={a.y}
                  x2={b.x}
                  y2={b.y}
                  stroke="url(#tuGrad)"
                  strokeWidth={1.5}
                  strokeLinecap="round"
                  initial={{ pathLength: 0, opacity: 0 }}
                  animate={{ pathLength: 1, opacity: 1 }}
                  transition={{ duration: 0.32, delay: i * 0.11, ease: EASE }}
                />
              );
            })}

          <defs>
            <linearGradient id="tuGrad" x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stopColor="#4f6ef7" />
              <stop offset="100%" stopColor="#7c9bff" />
            </linearGradient>
          </defs>

          {NODES.map((n) => {
            const secondary = !n.main;
            const isVerify = n.id === "verify";
            const lit = phase !== "processing";
            return (
              <g key={n.id} className={secondary ? "hidden sm:block" : undefined}>
                <circle
                  cx={n.x}
                  cy={n.y}
                  r={secondary ? 2.5 : 4.5}
                  fill={lit ? "#4f6ef7" : "#cbd5e1"}
                  opacity={lit ? 0.9 : 0.6}
                />
                {lit && (
                  <motion.circle
                    cx={n.x}
                    cy={n.y}
                    r={secondary ? 2.5 : 4.5}
                    fill="none"
                    stroke={isVerify && phase === "verifying" ? "#7c9bff" : "#4f6ef7"}
                    strokeWidth={1}
                    initial={{ opacity: 0.5, scale: 1 }}
                    animate={
                      isVerify && phase === "verifying"
                        ? { opacity: [0.5, 0, 0.5], scale: [1, 2.4, 1] }
                        : { opacity: 0, scale: 1.8 }
                    }
                    transition={
                      isVerify && phase === "verifying"
                        ? { duration: 1.1, repeat: Infinity, ease: "easeOut" }
                        : { duration: 0.5, ease: EASE }
                    }
                  />
                )}
                {n.main && (
                  <text
                    x={n.x}
                    y={n.y + 15}
                    textAnchor="middle"
                    className="fill-base-400"
                    fontSize={7}
                    fontFamily="var(--font-mono)"
                    letterSpacing={0.4}
                  >
                    {n.label}
                  </text>
                )}
              </g>
            );
          })}

          {/* Traveling payment token */}
          {phase !== "processing" && (
            <motion.circle
              r={3.5}
              fill="#ffffff"
              stroke="#4f6ef7"
              strokeWidth={1.5}
              initial={{ cx: NODES[0].x, cy: NODES[0].y, opacity: 0 }}
              animate={{
                cx: TOKEN_PATH.map((id) => nodeById(id).x),
                cy: TOKEN_PATH.map((id) => nodeById(id).y),
                opacity: 1,
              }}
              transition={{
                cx: { duration: 0.85, ease: EASE, times: [0, 0.16, 0.33, 0.5, 0.66, 0.83, 1] },
                cy: { duration: 0.85, ease: EASE, times: [0, 0.16, 0.33, 0.5, 0.66, 0.83, 1] },
                opacity: { duration: 0.15 },
              }}
              style={{ filter: "drop-shadow(0 0 4px rgba(79,110,247,0.6))" }}
            />
          )}
        </svg>
      </div>

      {/* Status line + micro metadata */}
      <div className="flex flex-col items-center gap-1">
        <div className="flex items-center gap-1.5">
          {phase === "success" ? (
            <ShieldCheck className="h-3.5 w-3.5 text-pos-400" />
          ) : (
            <WalletIcon className="h-3.5 w-3.5 text-accent-500" />
          )}
          <AnimatePresence mode="wait">
            <motion.span
              key={statusLabel}
              initial={{ opacity: 0, y: 2 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -2 }}
              transition={{ duration: 0.18 }}
              className="text-xs font-medium text-base-200"
              aria-live="polite"
            >
              {statusLabel}
            </motion.span>
          </AnimatePresence>
        </div>
        {phase !== "success" && (
          <AnimatePresence mode="wait">
            <motion.span
              key={METADATA[metaIndex]}
              initial={{ opacity: 0 }}
              animate={{ opacity: 0.7 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.25 }}
              className="font-mono text-[10px] tracking-wide text-base-400"
            >
              {METADATA[metaIndex]} · ID {shortOrder}…
            </motion.span>
          </AnimatePresence>
        )}
      </div>
    </div>
  );
}
