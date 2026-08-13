"use client";

import { useEffect, useState, useMemo } from "react";
import { ArrowUpRight, Boxes, Search, Activity, Zap } from "lucide-react";
import { Shell } from "@/components/Shell";
import {
  Card,
  Table,
  THead,
  Th,
  Td,
  Badge,
  Button,
  Skeleton,
  EmptyState,
} from "@/components/ui";
import { api, type MarketModel } from "@/lib/api";
import { formatINR, estimateSessionCost, providerBadge } from "@/lib/format";
import Link from "next/link";
import { StaggeredItem } from "@/components/PageTransition";

const CATEGORY_LABELS: Record<string, { label: string; tone: "blue" | "violet" }> = {
  chat: { label: "Chat", tone: "blue" },
  vision: { label: "Vision", tone: "violet" },
};

function useCatalog() {
  const [models, setModels] = useState<MarketModel[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = () => {
    setLoading(true);
    setError(null);
    api
      .catalog()
      .then((c) => setModels(c.models.filter((m) => m.is_active)))
      .catch((e) => setError(e instanceof Error ? e.message : "catalog failed"))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);
  return { models, loading, error, reload: load };
}

export default function MarketplacePage() {
  const { models, loading, error, reload } = useCatalog();
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    if (!models) return [];
    const q = query.trim().toLowerCase();
    if (!q) return models;
    return models.filter(
      (m) =>
        m.name.toLowerCase().includes(q) ||
        (m.provider && m.provider.toLowerCase().includes(q)) ||
        m.category.toLowerCase().includes(q),
    );
  }, [models, query]);

  return (
    <Shell>
      <div className="flex flex-col gap-6">
        {/* Page header */}
        <StaggeredItem index={0}>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h1 className="text-2xl font-semibold tracking-tight text-base-100">
                Marketplace
              </h1>
              <p className="mt-1 text-sm text-base-400">
                Production-ready AI providers. Pay per session from your INR wallet — no API keys required.
              </p>
            </div>
            <div className="flex items-center gap-2 text-xs text-base-400">
              <Badge tone="green" dot>
                {models?.length ?? "—"} active models
              </Badge>
              <Badge tone="blue">x402</Badge>
            </div>
          </div>
        </StaggeredItem>

        {/* Search + dense table */}
        <StaggeredItem index={1}>
          <Card className="overflow-hidden">
            <div className="border-b border-base-700 px-5 py-3">
              <div className="field flex items-center gap-3 rounded-md px-3">
                <Search className="h-3.5 w-3.5 text-base-400" />
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Search models, providers, categories…"
                  className="h-9 w-full bg-transparent text-sm text-base-100 outline-none placeholder:text-base-500"
                />
              </div>
            </div>

            {loading && !models && (
              <div className="space-y-0">
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-6 border-b border-base-700/70 px-5 py-4 last:border-0">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-4 w-20" />
                    <Skeleton className="h-4 w-16" />
                    <Skeleton className="h-4 w-20" />
                    <Skeleton className="h-4 w-14" />
                    <Skeleton className="ml-auto h-8 w-16" />
                  </div>
                ))}
              </div>
            )}

            {error && (
              <EmptyState
                icon={<Boxes className="h-8 w-8" />}
                title="Could not reach the catalog"
                body={error}
                action={
                  <Button variant="secondary" onClick={reload}>
                    Retry
                  </Button>
                }
              />
            )}

            {models && !error && (
              <Table>
                <THead>
                  <Th>Provider</Th>
                  <Th>Model</Th>
                  <Th>Price / Session</Th>
                  <Th>Est. Cost</Th>
                  <Th>Latency</Th>
                  <Th>Context</Th>
                  <Th>Availability</Th>
                  <Th>Status</Th>
                  <Th className="text-right">Use</Th>
                </THead>
                <tbody>
                  {filtered.map((m, idx) => {
                    const cat = CATEGORY_LABELS[m.category] ?? { label: m.category, tone: "gray" as const };
                    const est = estimateSessionCost(m, 512);
                    return (
                      <StaggeredItem key={m.id} index={idx}>
                        <tr className="border-b border-base-200 transition-colors duration-150 last:border-0 hover:bg-base-800/60">
                          <Td>
                            <div className="flex items-center gap-2.5">
                              <div className="relative flex h-8 w-8 items-center justify-center rounded-md border border-base-700 bg-base-850 text-[11px] font-semibold text-base-300">
                                {providerBadge(m.provider).slice(0, 2).toUpperCase()}
                                <span className="absolute -top-0.5 -right-0.5 h-2 w-2 rounded-full bg-pos-400 animate-pulse" />
                              </div>
                              <div className="min-w-0">
                                <div className="text-[13px] font-medium text-base-100">
                                  {providerBadge(m.provider)}
                                </div>
                                <div className="text-[11px] text-base-400">{m.provider_id}</div>
                              </div>
                            </div>
                          </Td>
                          <Td>
                            <div className="font-mono text-[13px] text-base-100">{m.name}</div>
                            {m.description && (
                              <div className="mt-0.5 max-w-[260px] truncate text-[11px] text-base-500">
                                {m.description}
                              </div>
                            )}
                            <Badge tone={cat.tone} className="mt-1">
                              {cat.label}
                            </Badge>
                          </Td>
                          <Td className="font-mono text-[13px] tabular-nums text-base-100">
                            {formatINR(m.price_base_paise)}/session
                          </Td>
                          <Td className="font-mono text-[13px] tabular-nums text-base-200">
                            {formatINR(est)}
                          </Td>
                          <Td className="font-mono text-[13px] tabular-nums text-base-200">
                            <span className="flex items-center gap-1">
                              <Zap className="h-3 w-3 text-accent-500" />
                              {m.p90_latency_ms} ms
                            </span>
                          </Td>
                          <Td className="font-mono text-[13px] tabular-nums text-base-200">
                            {(m.context_window / 1024).toFixed(0)}K
                          </Td>
                          <Td>
                            <Badge tone="green" dot>
                              In stock
                            </Badge>
                          </Td>
                          <Td>
                            <Badge tone={m.is_active ? "green" : "gray"} dot>
                              {m.is_active ? "Active" : "Inactive"}
                            </Badge>
                          </Td>
                          <Td className="text-right">
                            <Link href={`/playground?model=${encodeURIComponent(m.name)}`}>
                              <Button variant="secondary" size="sm">
                                Use
                                <ArrowUpRight className="h-3.5 w-3.5" />
                              </Button>
                            </Link>
                          </Td>
                        </tr>
                      </StaggeredItem>
                    );
                  })}
                  {filtered.length === 0 && (
                    <tr>
                      <td colSpan={9} className="px-5 py-12 text-center text-sm text-base-400">
                        No models match “{query}”.
                      </td>
                    </tr>
                  )}
                </tbody>
              </Table>
            )}
          </Card>
        </StaggeredItem>
      </div>
    </Shell>
  );
}
