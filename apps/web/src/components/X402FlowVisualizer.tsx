"use client";

/**
 * X402FlowVisualizer
 *
 * An interactive, step-by-step visualization of the x402 payment protocol.
 * Collapsed by default (progressive disclosure). When expanded, shows an
 * animated flow: Wallet → Reserve → Gateway → Provider → Stream → Settle → Refund → Ledger
 *
 * Each node activates as the session progresses through stages.
 * Clicking a node expands a plain-English explanation with live data.
 *
 * Design language: Vercel dashboard — functional, technical, educational.
 */

import { useState, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ChevronDown, ChevronRight, Info } from "lucide-react";

// ── Types ──────────────────────────────────────────────────────────────────────

export type X402Stage =
  | "idle"
  | "reserved"
  | "streaming"
  | "completed"
  | "settled"
  | "refunded"
  | "closed";

export interface X402SessionData {
  sessionId: string;
  reservedPaise: number;
  actualPaise: number;
  refundedPaise: number;
  providerName: string;
  modelName: string;
  latencyMs: number;
}

interface Props {
  stage: X402Stage;
  sessionData?: X402SessionData;
}

// ── Step definitions ────────────────────────────────────────────────────────────

interface FlowStep {
  id: string;
  label: string;
  shortLabel: string;
  activeFrom: X402Stage[];
  doneFrom: X402Stage[];
  explain: (data?: X402SessionData) => string;
  detail?: (data?: X402SessionData) => string | undefined;
}

const STAGES_ORDER: X402Stage[] = [
  "idle", "reserved", "streaming", "completed", "settled", "refunded", "closed",
];

function stageIndex(s: X402Stage) {
  return STAGES_ORDER.indexOf(s);
}

function isAtOrPast(current: X402Stage, target: X402Stage) {
  return stageIndex(current) >= stageIndex(target);
}

const FLOW_STEPS: FlowStep[] = [
  {
    id: "wallet",
    label: "Wallet",
    shortLabel: "Wallet",
    activeFrom: ["reserved"],
    doneFrom: ["streaming", "completed", "settled", "refunded", "closed"],
    explain: (d) =>
      `Your INR wallet${d ? ` holds ${fmt(d.reservedPaise + d.actualPaise)}` : ""}. Before any inference starts, the estimated session cost is locked from your available balance.`,
    detail: (d) =>
      d ? `Reserved: ${fmt(d.reservedPaise)} · Actual: ${fmt(d.actualPaise)}` : undefined,
  },
  {
    id: "reserve",
    label: "Reserve",
    shortLabel: "Reserve",
    activeFrom: ["reserved"],
    doneFrom: ["streaming", "completed", "settled", "refunded", "closed"],
    explain: () =>
      "The gateway reserves (holds) the estimated cost before calling any AI provider. This ensures you can't overdraft even if many sessions run concurrently.",
    detail: (d) => d ? `Session ${d.sessionId}` : undefined,
  },
  {
    id: "gateway",
    label: "Gateway",
    shortLabel: "GW",
    activeFrom: ["streaming"],
    doneFrom: ["completed", "settled", "refunded", "closed"],
    explain: () =>
      "The x402 gateway authenticates your session, routes the request to the right provider using the catalog, and sets up the SSE stream.",
  },
  {
    id: "provider",
    label: "Provider",
    shortLabel: "Provider",
    activeFrom: ["streaming"],
    doneFrom: ["completed", "settled", "refunded", "closed"],
    explain: (d) =>
      `The ${d?.providerName ?? "AI"} provider receives the request and starts generating tokens. Model: ${d?.modelName ?? "—"}.`,
    detail: (d) => d?.latencyMs ? `P90 latency: ${d.latencyMs}ms` : undefined,
  },
  {
    id: "stream",
    label: "Stream",
    shortLabel: "SSE",
    activeFrom: ["streaming"],
    doneFrom: ["completed", "settled", "refunded", "closed"],
    explain: () =>
      "Tokens are streamed back to your browser as Server-Sent Events (SSE). You see them character-by-character while the provider generates them.",
  },
  {
    id: "settle",
    label: "Settle",
    shortLabel: "Settle",
    activeFrom: ["completed", "settled"],
    doneFrom: ["refunded", "closed"],
    explain: (d) =>
      `The actual token count is metered and the real cost ${d ? fmt(d.actualPaise) : ""} is charged. The provider receives their payout immediately.`,
  },
  {
    id: "refund",
    label: "Refund",
    shortLabel: "Refund",
    activeFrom: ["refunded", "closed"],
    doneFrom: ["closed"],
    explain: (d) =>
      `Any unused reserved funds${d && d.refundedPaise > 0 ? ` (${fmt(d.refundedPaise)})` : ""} are instantly returned to your wallet. You only ever pay for what you use.`,
    detail: (d) => d && d.refundedPaise > 0 ? `Refunded: ${fmt(d.refundedPaise)}` : "No refund needed",
  },
  {
    id: "ledger",
    label: "Ledger",
    shortLabel: "Ledger",
    activeFrom: ["settled", "refunded", "closed"],
    doneFrom: ["closed"],
    explain: () =>
      "Every reserve, charge, and refund is written to the immutable in-memory ledger. You can inspect the full transaction history in the Wallet tab.",
  },
];

function fmt(paise: number) {
  return "₹" + (paise / 100).toFixed(2);
}

const EASE = [0.16, 1, 0.3, 1] as const;

// ── Components ─────────────────────────────────────────────────────────────────

function StepNode({
  step,
  isActive,
  isDone,
  isSelected,
  onClick,
}: {
  step: FlowStep;
  isActive: boolean;
  isDone: boolean;
  isSelected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`relative flex flex-col items-center gap-1 group focus:outline-none`}
      aria-label={`${step.label} stage`}
      aria-pressed={isSelected}
    >
      {/* Circle node */}
      <motion.div
        animate={{
          borderColor: isSelected
            ? "rgb(79 110 247 / 0.7)"
            : isActive
            ? "rgb(79 110 247 / 0.4)"
            : isDone
            ? "rgb(22 163 74 / 0.4)"
            : "rgb(203 213 225 / 0.3)",
          backgroundColor: isSelected
            ? "rgb(79 110 247 / 0.12)"
            : isActive
            ? "rgb(79 110 247 / 0.06)"
            : isDone
            ? "rgb(22 163 74 / 0.05)"
            : "rgb(248 250 252 / 0.5)",
          scale: isActive ? 1.08 : 1,
        }}
        transition={{ duration: 0.3, ease: EASE }}
        className={`h-7 w-7 sm:h-8 sm:w-8 rounded-full border-2 flex items-center justify-center transition-all`}
      >
        <motion.span
          animate={{
            color: isSelected
              ? "rgb(79 110 247)"
              : isActive
              ? "rgb(79 110 247 / 0.8)"
              : isDone
              ? "rgb(22 163 74 / 0.7)"
              : "rgb(148 163 184)",
          }}
          transition={{ duration: 0.25 }}
          className="text-[9px] font-mono font-semibold leading-none"
        >
          {step.shortLabel.slice(0, 3).toUpperCase()}
        </motion.span>
        {/* Active pulse ring */}
        {isActive && (
          <motion.div
            className="absolute inset-0 rounded-full border border-accent-500/40"
            animate={{ scale: [1, 1.35, 1], opacity: [0.6, 0, 0.6] }}
            transition={{ duration: 1.5, repeat: Infinity }}
          />
        )}
      </motion.div>
      {/* Label */}
      <span
        className={`text-[9px] font-mono leading-tight text-center ${
          isActive || isSelected
            ? "text-accent-500"
            : isDone
            ? "text-pos-400/70"
            : "text-base-500"
        }`}
      >
        {step.shortLabel}
      </span>
    </button>
  );
}

function ConnectorLine({ done, active }: { done: boolean; active: boolean }) {
  return (
    <div className="flex-1 flex items-center px-0.5 mb-5">
      <div className="h-px w-full relative overflow-hidden rounded-full bg-base-700/40">
        <motion.div
          className="absolute inset-y-0 left-0 bg-gradient-to-r from-accent-500/60 to-accent-400/40"
          animate={{ width: done ? "100%" : active ? "60%" : "0%" }}
          transition={{ duration: 0.5, ease: EASE }}
        />
      </div>
    </div>
  );
}

// ── Main Export ─────────────────────────────────────────────────────────────────

export function X402FlowVisualizer({ stage, sessionData }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [selectedStep, setSelectedStep] = useState<string | null>(null);

  const handleStepClick = useCallback((id: string) => {
    setSelectedStep((prev) => (prev === id ? null : id));
  }, []);

  const selectedStepData = FLOW_STEPS.find((s) => s.id === selectedStep);

  const activeLabel =
    stage === "idle"
      ? "No active session"
      : stage === "reserved"
      ? "Session reserved"
      : stage === "streaming"
      ? "Streaming…"
      : stage === "settled"
      ? "Settled"
      : stage === "closed"
      ? "Session closed"
      : stage;

  return (
    <div className="rounded-lg border border-base-700/50 bg-base-900/60 overflow-hidden">
      {/* Collapsible header */}
      <button
        onClick={() => setExpanded((e) => !e)}
        className="w-full flex items-center justify-between px-3 py-2.5 hover:bg-base-800/40 transition-fast focus-ring text-left"
        aria-expanded={expanded}
        aria-controls="x402-flow-panel"
      >
        <div className="flex items-center gap-2">
          <Info className="h-3.5 w-3.5 text-base-500" />
          <span className="text-xs font-medium text-base-300">Protocol Inspector</span>
          <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded border ${
            stage === "streaming"
              ? "border-accent-500/30 bg-accent-500/10 text-accent-500"
              : stage === "closed"
              ? "border-pos-400/30 bg-pos-400/8 text-pos-400"
              : "border-base-700/40 text-base-500 bg-transparent"
          }`}>
            {activeLabel}
          </span>
        </div>
        <motion.div
          animate={{ rotate: expanded ? 180 : 0 }}
          transition={{ duration: 0.2 }}
        >
          <ChevronDown className="h-3.5 w-3.5 text-base-500" />
        </motion.div>
      </button>

      {/* Flow panel */}
      <AnimatePresence>
        {expanded && (
          <motion.div
            id="x402-flow-panel"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.3, ease: EASE }}
            className="overflow-hidden"
          >
            <div className="px-3 pb-3 pt-2 border-t border-base-800/50">
              {/* Sub-label */}
              <p className="text-[10px] text-base-500 mb-3 leading-relaxed">
                x402 session lifecycle · Click any node to inspect
              </p>

              {/* Flow nodes */}
              <div className="flex items-start gap-0 overflow-x-auto pb-1 scrollbar-thin">
                {FLOW_STEPS.map((step, i) => {
                  const isActive = step.activeFrom.some((s) => isAtOrPast(stage, s)) &&
                    !step.doneFrom.some((s) => isAtOrPast(stage, s));
                  const isDone = step.doneFrom.some((s) => isAtOrPast(stage, s));
                  const showConnector = i < FLOW_STEPS.length - 1;
                  const nextDone = i < FLOW_STEPS.length - 1 &&
                    FLOW_STEPS[i + 1].doneFrom.some((s) => isAtOrPast(stage, s));

                  return (
                    <div key={step.id} className="flex items-start shrink-0">
                      <StepNode
                        step={step}
                        isActive={isActive}
                        isDone={isDone}
                        isSelected={selectedStep === step.id}
                        onClick={() => handleStepClick(step.id)}
                      />
                      {showConnector && (
                        <ConnectorLine done={nextDone} active={isActive} />
                      )}
                    </div>
                  );
                })}
              </div>

              {/* Step explanation panel */}
              <AnimatePresence mode="wait">
                {selectedStepData && (
                  <motion.div
                    key={selectedStep}
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -4 }}
                    transition={{ duration: 0.22, ease: EASE }}
                    className="mt-3 rounded-md border border-accent-500/15 bg-accent-500/5 px-3 py-2.5"
                  >
                    <div className="flex items-center justify-between gap-2 mb-1">
                      <span className="text-[11px] font-semibold text-accent-500">
                        {selectedStepData.label}
                      </span>
                      <button
                        onClick={() => setSelectedStep(null)}
                        className="text-base-500 hover:text-base-300 text-[10px] focus-ring rounded"
                      >
                        ✕
                      </button>
                    </div>
                    <p className="text-[11px] leading-relaxed text-base-300">
                      {selectedStepData.explain(sessionData)}
                    </p>
                    {selectedStepData.detail?.(sessionData) && (
                      <p className="mt-1.5 font-mono text-[10px] text-base-500">
                        {selectedStepData.detail(sessionData)}
                      </p>
                    )}
                  </motion.div>
                )}
              </AnimatePresence>

              {!selectedStep && (
                <p className="mt-2 text-[10px] text-base-600 italic">
                  ↑ Click a node above to see what happens at each stage
                </p>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
