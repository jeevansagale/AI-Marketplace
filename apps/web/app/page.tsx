"use client";

import Link from "next/link";
import { ArrowRight, Wallet, Boxes, Play, ShieldCheck, Radio, Zap, ArrowRightLeft, RefreshCw, CheckCircle2 } from "lucide-react";
import { motion } from "framer-motion";
import { Shell } from "@/components/Shell";
import { Card, Badge, Button, Skeleton } from "@/components/ui";
import { useWalletClient } from "@/lib/stores/useWallet";
import { formatINR } from "@/lib/format";
import { useEffect } from "react";
import { StaggeredItem } from "@/components/PageTransition";

const EASE = [0.16, 1, 0.3, 1] as const;

const PROTOCOL_NODES = [
  { id: "01", title: "INR Wallet", desc: "Prepaid balance" },
  { id: "02", title: "Cost Reserve", desc: "Estimated hold" },
  { id: "03", title: "AI Router", desc: "Catalog dispatch" },
  { id: "04", title: "SSE Stream", desc: "Metered tokens" },
  { id: "05", title: "Settlement", desc: "Provider payout" },
  { id: "06", title: "Auto Refund", desc: "Unused balance" },
];

export default function Home() {
  const { balance, loading, refresh } = useWalletClient();

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <Shell>
      <div className="flex flex-col gap-10 lg:gap-12">
        {/* Hero — restrained, editorial */}
        <div className="pt-4 sm:pt-10">
          <StaggeredItem index={0}>
            <Badge tone="blue" className="mb-4 inline-flex">
              <span className="flex items-center gap-1.5">
                <Radio className="h-3 w-3 text-accent-500 animate-pulse" />
                HTTP 402 · Wallet payments · Settlement-ready
              </span>
            </Badge>
          </StaggeredItem>
          <StaggeredItem index={1}>
            <h1 className="max-w-2xl text-3xl font-semibold leading-tight text-base-100 sm:text-4xl text-balance">
              Premium AI inference,
              <br />
              paid like infrastructure.
            </h1>
          </StaggeredItem>
          <StaggeredItem index={2}>
            <p className="mt-3 max-w-xl text-[15px] leading-relaxed text-base-300">
              Discover AI providers, top up an INR wallet, and consume streaming AI
              services with per-session reservations. No API keys, no surprise bills —
              unused funds are refunded automatically.
            </p>
          </StaggeredItem>
          <StaggeredItem index={3}>
            <div className="mt-6 flex flex-wrap gap-3">
              <Link href="/marketplace">
                <Button size="md">
                  Browse marketplace
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </Link>
              <Link href="/playground">
                <Button variant="secondary" size="md">
                  <Play className="h-4 w-4" /> Open playground
                </Button>
              </Link>
            </div>
          </StaggeredItem>
        </div>

        {/* Wallet stat strip */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <StaggeredItem index={0}>
            <Card className="p-5">
              <div className="flex items-center gap-2 text-xs text-base-400">
                <Wallet className="h-3.5 w-3.5" />
                Wallet balance
              </div>
              {loading && !balance ? (
                <Skeleton className="mt-2 h-7 w-32" />
              ) : (
                <p className="mt-2 font-mono text-2xl font-semibold tabular-nums text-base-100">
                  {balance ? formatINR(balance.available_paise) : "—"}
                </p>
              )}
              <p className="mt-1 text-xs text-base-500">
                {balance && balance.reserved_paise > 0
                  ? `${formatINR(balance.reserved_paise)} reserved in live sessions`
                  : "Available in your INR wallet"}
              </p>
            </Card>
          </StaggeredItem>

          <StaggeredItem index={1}>
            <Card className="p-5">
              <div className="flex items-center gap-2 text-xs text-base-400">
                <ShieldCheck className="h-3.5 w-3.5" />
                Session reservations
              </div>
              <p className="mt-2 text-2xl font-semibold text-base-100">INR</p>
              <p className="mt-1 text-xs text-base-500">
                Estimate → reserve → meter → settle → refund
              </p>
            </Card>
          </StaggeredItem>

          <StaggeredItem index={2}>
            <Card className="p-5">
              <div className="flex items-center gap-2 text-xs text-base-400">
                <Boxes className="h-3.5 w-3.5" />
                Providers
              </div>
              <p className="mt-2 text-2xl font-semibold text-base-100">Gemini + OpenAI</p>
              <p className="mt-1 text-xs text-base-500">
                One interface. Swap providers without touching business logic.
              </p>
            </Card>
          </StaggeredItem>
        </div>

        {/* Animated x402 Protocol Flow */}
        <StaggeredItem index={0}>
          <div className="space-y-4">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-base-400">
              x402 Protocol Lifecycle
            </h2>
            <Card className="p-6 overflow-hidden">
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-6">
                {PROTOCOL_NODES.map((node, i) => (
                  <motion.div
                    key={node.id}
                    initial={{ opacity: 0, y: 10 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.4, delay: i * 0.08, ease: EASE }}
                    className="relative flex flex-col justify-between rounded-lg border border-base-700/60 bg-base-850 p-4"
                  >
                    <div>
                      <span className="font-mono text-[10px] text-accent-500 font-semibold">{node.id}</span>
                      <h3 className="mt-1 text-xs font-semibold text-base-100">{node.title}</h3>
                    </div>
                    <p className="mt-2 text-[11px] text-base-400">{node.desc}</p>
                  </motion.div>
                ))}
              </div>
            </Card>
          </div>
        </StaggeredItem>
      </div>
    </Shell>
  );
}
