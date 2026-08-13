"use client";

/**
 * SessionMeter
 *
 * Live session metrics panel shown during and after streaming.
 * Displays provider, model, token counts, actual cost (animated), latency,
 * and settlement progress. All numeric values animate on change.
 *
 * Design language: Stripe Dashboard data table — scannable, monospaced, precise.
 */

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Zap,
  DollarSign,
  Clock,
  Hash,
  CheckCircle2,
  Loader2,
  Activity,
} from "lucide-react";
import { formatINR } from "@/lib/format";

export interface SessionMetrics {
  providerName: string;
  modelName: string;
  modelSlug: string;
  /** Paise reserved at session start */
  reservedPaise: number;
  /** Paise actually spent — updated live from SSE summary */
  actualPaise: number;
  promptTokens: number;
  completionTokens: number;
  latencyMs: number;
  /** 0–1 settlement progress */
  settlementProgress: number;
  /** Whether the session has fully closed */
  settled: boolean;
}

interface Props {
  metrics: SessionMetrics;
  /** Whether streaming is currently active */
  streaming: boolean;
}

const EASE = [0.16, 1, 0.3, 1] as const;

/** Animated number that re-renders with a quick slide-up on value change */
function AnimatedValue({ value, className = "" }: { value: string | number; className?: string }) {
  const prev = useRef(value);
  const [changed, setChanged] = useState(false);

  useEffect(() => {
    if (prev.current !== value) {
      prev.current = value;
      setChanged(true);
      const t = setTimeout(() => setChanged(false), 160);
      return () => clearTimeout(t);
    }
  }, [value]);

  return (
    <span
      className={`inline-block tabular ${className} ${changed ? "animate-count" : ""}`}
    >
      {value}
    </span>
  );
}

interface MetricRowProps {
  icon: React.FC<{ className?: string }>;
  label: string;
  value: string | number;
  sub?: string;
  accent?: boolean;
}

function MetricRow({ icon: Icon, label, value, sub, accent }: MetricRowProps) {
  return (
    <div className="flex items-start justify-between gap-2 py-1.5">
      <div className="flex items-center gap-1.5 min-w-0">
        <Icon className={`h-3 w-3 shrink-0 ${accent ? "text-accent-500" : "text-base-500"}`} />
        <span className="text-[11px] text-base-400 truncate">{label}</span>
      </div>
      <div className="text-right shrink-0">
        <AnimatedValue
          value={value}
          className={`text-[11px] font-mono font-medium ${accent ? "text-accent-500" : "text-base-200"}`}
        />
        {sub && <div className="text-[10px] text-base-500 mt-0.5">{sub}</div>}
      </div>
    </div>
  );
}

export function SessionMeter({ metrics, streaming }: Props) {
  const {
    providerName,
    modelName,
    reservedPaise,
    actualPaise,
    promptTokens,
    completionTokens,
    latencyMs,
    settlementProgress,
    settled,
  } = metrics;

  // Live elapsed timer while streaming
  const [elapsed, setElapsed] = useState(0);
  const startTime = useRef<number | null>(null);

  useEffect(() => {
    if (streaming) {
      startTime.current = Date.now();
      const interval = setInterval(() => {
        setElapsed(Math.floor((Date.now() - (startTime.current ?? Date.now())) / 1000));
      }, 1000);
      return () => clearInterval(interval);
    } else {
      // keep final elapsed
    }
  }, [streaming]);

  const refundPaise = Math.max(0, reservedPaise - actualPaise);
  const spendPercent = reservedPaise > 0
    ? Math.min(100, Math.round((actualPaise / reservedPaise) * 100))
    : 0;

  return (
    <motion.div
      initial={{ opacity: 0, x: 8 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.35, ease: EASE }}
      className="rounded-lg border border-base-700/60 bg-base-900/80 backdrop-blur-sm overflow-hidden"
    >
      {/* Header */}
      <div className="flex items-center justify-between gap-2 px-3 py-2 border-b border-base-800/60">
        <div className="flex items-center gap-1.5">
          {streaming ? (
            <motion.div
              animate={{ opacity: [1, 0.4, 1] }}
              transition={{ duration: 1.2, repeat: Infinity }}
            >
              <Activity className="h-3 w-3 text-accent-500" />
            </motion.div>
          ) : settled ? (
            <CheckCircle2 className="h-3 w-3 text-pos-400" />
          ) : (
            <Loader2 className="h-3 w-3 text-base-500 animate-spin" />
          )}
          <span className="text-[11px] font-medium text-base-300">
            {streaming ? "Live session" : settled ? "Settled" : "Completing…"}
          </span>
        </div>
        {streaming && (
          <span className="font-mono text-[10px] text-base-500">
            {elapsed}s
          </span>
        )}
      </div>

      {/* Metrics */}
      <div className="px-3 divide-y divide-base-800/40">
        <MetricRow icon={Zap} label="Provider" value={providerName} />
        <MetricRow icon={Hash} label="Model" value={modelName} />
        <MetricRow
          icon={Hash}
          label="Tokens"
          value={`${promptTokens + completionTokens}`}
          sub={`${promptTokens}↑ ${completionTokens}↓`}
        />
        <MetricRow
          icon={Clock}
          label="Latency"
          value={latencyMs > 0 ? `${latencyMs}ms` : streaming ? `${elapsed * 1000}ms` : "—"}
        />
        <MetricRow
          icon={DollarSign}
          label="Reserved"
          value={formatINR(reservedPaise)}
        />
        <MetricRow
          icon={DollarSign}
          label="Actual cost"
          value={actualPaise > 0 ? formatINR(actualPaise) : "—"}
          accent={actualPaise > 0}
        />
        {settled && refundPaise > 0 && (
          <MetricRow
            icon={DollarSign}
            label="Refunded"
            value={`+ ${formatINR(refundPaise)}`}
          />
        )}
      </div>

      {/* Settlement progress bar */}
      <div className="px-3 pb-3 pt-2">
        <div className="flex items-center justify-between mb-1">
          <span className="text-[10px] text-base-500">Settlement</span>
          <span className="text-[10px] font-mono text-base-400">
            {settled ? "100%" : `${spendPercent}%`}
          </span>
        </div>
        <div className="h-1 w-full rounded-full bg-base-800 overflow-hidden">
          <motion.div
            className="h-full rounded-full bg-gradient-to-r from-accent-500 to-pos-400"
            initial={{ width: "0%" }}
            animate={{
              width: settled ? "100%" : streaming ? `${spendPercent}%` : `${spendPercent}%`,
            }}
            transition={{ duration: 0.6, ease: EASE }}
          />
        </div>

        {/* Stage chips */}
        <div className="mt-2 flex flex-wrap gap-1">
          {(["Reserved", "Streaming", "Settled", "Refunded"] as const).map((stage, i) => {
            const active =
              (stage === "Reserved" && !settled) ||
              (stage === "Streaming" && streaming) ||
              (stage === "Settled" && settled) ||
              (stage === "Refunded" && settled && refundPaise > 0);
            const done =
              (stage === "Reserved") ||
              (stage === "Streaming" && (settled || !streaming)) ||
              (stage === "Settled" && settled);
            return (
              <AnimatePresence key={stage}>
                <motion.span
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ delay: i * 0.05, duration: 0.2 }}
                  className={`inline-flex items-center rounded px-1.5 py-0.5 text-[9px] font-mono font-medium uppercase tracking-wide border ${
                    active
                      ? "border-accent-500/30 bg-accent-500/10 text-accent-500"
                      : done
                      ? "border-pos-400/20 bg-pos-400/5 text-pos-400/70"
                      : "border-base-700/40 bg-transparent text-base-600"
                  }`}
                >
                  {stage}
                </motion.span>
              </AnimatePresence>
            );
          })}
        </div>
      </div>
    </motion.div>
  );
}
