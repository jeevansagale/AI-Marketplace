"use client";

/**
 * ProviderRoutingAnimation
 *
 * A cinematic provider-selection card stack that plays for ~1.4s when a
 * session starts. Shows which provider "won" the routing decision along with
 * their latency, price, and context window. The losing provider(s) gracefully
 * fade and compress.
 *
 * Design language: Stripe / Linear — no decoration for its own sake.
 */

import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { CheckCircle2, Zap, DollarSign, Layers } from "lucide-react";

export interface ProviderRouteResult {
  selectedProvider: string; // e.g. "Gemini"
  selectedModel: string;    // e.g. "gemini-2.5-flash"
  modelName: string;        // e.g. "Gemini 2.5 Flash"
  latencyMs: number;
  pricePaise: number;       // price_base_paise
  contextWindow: number;
}

interface Props {
  result: ProviderRouteResult;
  /** Called when the animation sequence finishes and the caller can show content */
  onDone: () => void;
}

interface ProviderCardData {
  name: string;
  kind: "gemini" | "openai";
  latencyMs: number;
  pricePaise: number;
  contextK: number;
  isWinner: boolean;
}

const EASE_OUT = [0.16, 1, 0.3, 1] as const;

function formatPaise(p: number) {
  return "₹" + (p / 100).toFixed(2);
}
function formatContext(n: number) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(0) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(0) + "K";
  return n.toString();
}

/** Tiny chip inside a provider card */
function Chip({ icon: Icon, value, highlight }: { icon: React.FC<{ className?: string }>; value: string; highlight?: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-mono font-medium ${
        highlight
          ? "bg-accent-500/10 text-accent-500 border border-accent-500/20"
          : "bg-base-800/50 text-base-400 border border-base-700/50"
      }`}
    >
      <Icon className="h-2.5 w-2.5" />
      {value}
    </span>
  );
}

function ProviderCard({ card, delay, phase }: {
  card: ProviderCardData;
  delay: number;
  phase: "searching" | "routing" | "selected";
}) {
  const isSelected = card.isWinner && phase === "selected";
  const isDimmed = !card.isWinner && phase === "selected";

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{
        opacity: isDimmed ? 0.3 : 1,
        y: 0,
        scale: isSelected ? 1.01 : 1,
      }}
      transition={{ duration: 0.4, ease: EASE_OUT, delay }}
      className={`relative overflow-hidden rounded-lg border p-3 transition-cinematic ${
        isSelected
          ? "border-accent-500/40 bg-accent-500/5 shadow-sm"
          : isDimmed
          ? "border-base-700/40 bg-base-900/40"
          : "border-base-700/60 bg-base-900/60"
      }`}
    >
      {/* Selection glow */}
      {isSelected && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.3, delay: 0.1 }}
          className="absolute inset-0 pointer-events-none rounded-lg"
          style={{
            background: "radial-gradient(ellipse at top left, rgb(79 110 247 / 0.06), transparent 70%)",
          }}
        />
      )}

      <div className="relative flex items-start justify-between gap-2">
        <div>
          {/* Provider name */}
          <div className="flex items-center gap-1.5">
            <span className="text-xs font-semibold text-base-200">{card.name}</span>
            {isSelected && (
              <motion.span
                initial={{ opacity: 0, scale: 0.7 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.25, delay: 0.15, ease: EASE_OUT }}
              >
                <CheckCircle2 className="h-3 w-3 text-pos-400" />
              </motion.span>
            )}
          </div>

          {/* Metric chips */}
          <div className="mt-1.5 flex flex-wrap gap-1">
            <Chip icon={Zap} value={`${card.latencyMs}ms`} highlight={isSelected} />
            <Chip icon={DollarSign} value={formatPaise(card.pricePaise)} />
            <Chip icon={Layers} value={formatContext(card.contextK * 1000) + " ctx"} />
          </div>
        </div>

        {/* Phase indicator */}
        <div className="shrink-0 pt-0.5">
          {phase === "searching" && (
            <motion.div
              className="h-2 w-2 rounded-full bg-warn-400"
              animate={{ opacity: [1, 0.3, 1] }}
              transition={{ duration: 1, repeat: Infinity }}
            />
          )}
          {phase === "routing" && (
            <motion.div
              className="h-2 w-2 rounded-full bg-accent-400"
              animate={{ scale: [1, 1.3, 1], opacity: [0.7, 1, 0.7] }}
              transition={{ duration: 0.8, repeat: Infinity }}
            />
          )}
          {isSelected && (
            <div className="h-2 w-2 rounded-full bg-pos-400 animate-pulse-glow" />
          )}
          {isDimmed && (
            <div className="h-2 w-2 rounded-full bg-base-600" />
          )}
        </div>
      </div>
    </motion.div>
  );
}

export function ProviderRoutingAnimation({ result, onDone }: Props) {
  const [phase, setPhase] = useState<"searching" | "routing" | "selected">("searching");

  // Build representative card data for both providers.
  const cards: ProviderCardData[] = [
    {
      name: result.selectedProvider,
      kind: result.selectedProvider.toLowerCase() === "gemini" ? "gemini" : "openai",
      latencyMs: result.latencyMs,
      pricePaise: result.pricePaise,
      contextK: Math.round(result.contextWindow / 1000),
      isWinner: true,
    },
    {
      // The losing provider — show realistic alternative metrics
      name: result.selectedProvider === "Gemini" ? "OpenAI" : "Gemini",
      kind: result.selectedProvider === "Gemini" ? "openai" : "gemini",
      latencyMs: result.latencyMs + 80 + Math.floor(Math.random() * 60),
      pricePaise: Math.round(result.pricePaise * 1.6),
      contextK: result.selectedProvider === "Gemini" ? 128 : 1048,
      isWinner: false,
    },
  ];

  // Stagger the phase transitions.
  useEffect(() => {
    const t1 = setTimeout(() => setPhase("routing"), 380);
    const t2 = setTimeout(() => setPhase("selected"), 820);
    const t3 = setTimeout(() => onDone(), 1500);
    return () => {
      clearTimeout(t1);
      clearTimeout(t2);
      clearTimeout(t3);
    };
  }, [onDone]);

  const label =
    phase === "searching"
      ? "Searching marketplace…"
      : phase === "routing"
      ? "Evaluating providers…"
      : `Routed to ${result.selectedProvider}`;

  return (
    <div className="space-y-3">
      {/* Status line */}
      <div className="flex items-center gap-2">
        <AnimatePresence mode="wait">
          <motion.div
            key={phase}
            initial={{ opacity: 0, x: -4 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 4 }}
            transition={{ duration: 0.2 }}
            className="flex items-center gap-2"
          >
            {phase !== "selected" && (
              <motion.div
                className="h-1.5 w-1.5 rounded-full bg-accent-500"
                animate={{ scale: [1, 1.5, 1] }}
                transition={{ duration: 0.7, repeat: Infinity }}
              />
            )}
            <span className="text-[11px] font-mono text-base-400">{label}</span>
          </motion.div>
        </AnimatePresence>
      </div>

      {/* Provider cards */}
      <div className="space-y-2">
        {cards.map((card, i) => (
          <ProviderCard
            key={card.name}
            card={card}
            delay={i * 0.08}
            phase={phase}
          />
        ))}
      </div>

      {/* Model name line — appears on selection */}
      <AnimatePresence>
        {phase === "selected" && (
          <motion.div
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: EASE_OUT }}
            className="flex items-center gap-1.5 pt-0.5"
          >
            <span className="text-[11px] text-base-500">Model</span>
            <span className="font-mono text-[11px] text-accent-500">{result.modelName}</span>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
