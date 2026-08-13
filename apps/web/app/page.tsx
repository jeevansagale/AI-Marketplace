"use client";

import Link from "next/link";
import { ArrowRight, Wallet, Boxes, Play, ShieldCheck, Radio } from "lucide-react";
import { Shell } from "@/components/Shell";
import { Card, Badge, Button, Skeleton } from "@/components/ui";
import { useWalletClient } from "@/lib/stores/useWallet";
import { formatINR } from "@/lib/format";
import { useEffect } from "react";
import { StaggeredItem } from "@/components/PageTransition";

const STEPS = [
  { n: "01", title: "Pick a model", body: "Browse providers by price, latency, and context window." },
  { n: "02", title: "Fund your wallet", body: "Top up in INR via a verified payment provider." },
  { n: "03", title: "Run a session", body: "Estimated cost reserved, metered while streaming, remainder refunded." },
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
                <Radio className="h-3 w-3" />
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

        {/* How it works */}
        <StaggeredItem index={0}>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-base-400">
            How it works
          </h2>
        </StaggeredItem>
        <StaggeredItem index={1}>
          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
            {STEPS.map((s, idx) => (
              <StaggeredItem key={s.n} index={idx}>
                <Card className="p-5">
                  <div className="font-mono text-xs text-accent-400">{s.n}</div>
                  <h3 className="mt-2 text-sm font-semibold text-base-100">{s.title}</h3>
                  <p className="mt-1 text-[13px] leading-relaxed text-base-400">{s.body}</p>
                </Card>
              </StaggeredItem>
            ))}
          </div>
        </StaggeredItem>
      </div>
    </Shell>
  );
}
