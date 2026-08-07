"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { Wallet, ArrowDownLeft, ArrowUpRight, Coins, Activity } from "lucide-react";
import { Shell } from "@/components/Shell";
import { Button, Card, CardHeader, Badge, Skeleton, Table, THead, Th, Td, TRow } from "@/components/ui";
import { api, type LedgerEntry, type WalletTx } from "@/lib/api";
import { formatINR, formatSigned, providerBadge } from "@/lib/format";
import { useWallet } from "@/lib/stores/useWallet";

export default function WalletPage() {
  const { balance, transactions, loading, refresh, invalidate } = useWallet();
  const [ledger, setLedger] = useState<LedgerEntry[] | null>(null);
  const [ledgerLoading, setLedgerLoading] = useState(true);

  const loadLedger = useCallback(() => {
    setLedgerLoading(true);
    api
      .ledger()
      .then((l) => setLedger(l.entries))
      .catch(() => setLedger([]))
      .finally(() => setLedgerLoading(false));
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 5000);
    return () => clearInterval(t);
  }, [refresh]);

  useEffect(() => {
    void loadLedger();
  }, [loadLedger]);

  // Aggregate spend: today / this week from transactions (debited sessions only).
  const stats = useMemo(() => {
    const now = new Date();
    const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    const startOfWeek = new Date(now.getFullYear(), now.getMonth(), now.getDate() - now.getDay()).getTime();
    const spent: WalletTx[] = transactions.filter((tx) => (tx.kind === "debit" || tx.kind === "session") && tx.amount < 0);
    const today = spent.filter((tx) => new Date(tx.created_at).getTime() >= startOfDay);
    const week = spent.filter((tx) => new Date(tx.created_at).getTime() >= startOfWeek);
    const sum = (list: WalletTx[]) => list.reduce((acc, tx) => acc + Math.abs(tx.amount), 0);
    return {
      today: sum(today),
      week: sum(week),
      sessions: spent.length,
    };
  }, [transactions]);

  // Top providers by session spend.
  const topProviders = useMemo(() => {
    const map = new Map<string, { name: string; total: number }>();
    for (const tx of transactions) {
      if (tx.kind !== "session" || tx.amount >= 0) continue;
      const name = tx.reference.split("/")[0] || "Unknown";
      const cur = map.get(name) ?? { name, total: 0 };
      cur.total += Math.abs(tx.amount);
      map.set(name, cur);
    }
    return [...map.values()].sort((a, b) => b.total - a.total).slice(0, 4);
  }, [transactions]);

  return (
    <Shell>
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight text-base-100">Wallet</h1>
            <p className="mt-1 text-sm text-base-400">
              INR prepaid wallet — reserved, metered, and settled per session.
            </p>
          </div>
          <Badge tone="green" dot>Live</Badge>
        </div>

        {/* Stat cards */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Card className="p-5">
            <div className="flex items-center gap-2 text-xs text-base-400">
              <Wallet className="h-3.5 w-3.5" /> Current balance
            </div>
            {loading && !balance ? (
              <Skeleton className="mt-2 h-8 w-28" />
            ) : (
              <p className="mt-2 font-mono text-[26px] font-semibold tabular-nums text-base-100">
                {balance ? formatINR(balance.available) : "—"}
              </p>
            )}
            <p className="mt-1 text-xs text-base-500">
              {balance && balance.reserved > 0 ? `${formatINR(balance.reserved)} held in reservations` : "No active reservations"}
            </p>
          </Card>

          <Card className="p-5">
            <div className="flex items-center gap-2 text-xs text-base-400">
              <ArrowDownLeft className="h-3.5 w-3.5" /> Today's spend
            </div>
            <p className="mt-2 font-mono text-[26px] font-semibold tabular-nums text-base-100">
              {formatINR(stats.today)}
            </p>
            <p className="mt-1 text-xs text-base-500">Across all AI providers</p>
          </Card>

          <Card className="p-5">
            <div className="flex items-center gap-2 text-xs text-base-400">
              <Coins className="h-3.5 w-3.5" /> This week
            </div>
            <p className="mt-2 font-mono text-[26px] font-semibold tabular-nums text-base-100">
              {formatINR(stats.week)}
            </p>
            <p className="mt-1 text-xs text-base-500">7-day rolling spend</p>
          </Card>

          <Card className="p-5">
            <div className="flex items-center gap-2 text-xs text-base-400">
              <Activity className="h-3.5 w-3.5" /> Sessions
            </div>
            <p className="mt-2 text-[26px] font-semibold text-base-100">{stats.sessions}</p>
            <p className="mt-1 text-xs text-base-500">Billed sessions to date</p>
          </Card>
        </div>

        <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
          {/* Recent transactions */}
          <Card className="lg:col-span-2">
            <CardHeader
              title="Recent transactions"
              subtitle="Credits appear after verified top-ups; debits after settlement."
              icon={<Wallet className="h-4 w-4" />}
            />
            <Table>
              <THead>
                <Th>Type</Th>
                <Th>Reference</Th>
                <Th>Time</Th>
                <Th className="text-right">Amount</Th>
              </THead>
              <tbody>
                {transactions.slice(0, 8).map((tx) => (
                  <TRow key={tx.id}>
                    <Td>
                      <Badge
                        tone={tx.amount >= 0 ? "green" : "gray"}
                        dot
                      >
                        {tx.kind}
                      </Badge>
                    </Td>
                    <Td className="font-mono text-[13px] text-base-200">{tx.reference}</Td>
                    <Td className="text-[13px] text-base-400">
                      {new Date(tx.created_at).toLocaleString("en-IN", { hour12: false })}
                    </Td>
                    <Td className={`text-right font-mono text-[13px] tabular-nums ${tx.amount >= 0 ? "text-pos-400" : "text-base-200"}`}>
                      {formatSigned(tx.amount)}
                    </Td>
                  </TRow>
                ))}
                {transactions.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-5 py-10 text-center text-sm text-base-400">
                      No transactions yet. Top up to get started.
                    </td>
                  </tr>
                )}
              </tbody>
            </Table>
          </Card>

          {/* Top providers */}
          <Card>
            <CardHeader title="Top providers" subtitle="By session spend" icon={<Coins className="h-4 w-4" />} />
            <div className="divide-y divide-base-200 px-5 py-2">
              {topProviders.length === 0 && (
                <p className="py-4 text-xs text-base-400">No provider spend yet.</p>
              )}
              {topProviders.map((p) => (
                <div key={p.name} className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-2.5">
                    <div className="flex h-7 w-7 items-center justify-center rounded-md bg-base-800 text-[10px] font-semibold text-base-300">
                      {providerBadge(p.name).slice(0, 2)}
                    </div>
                    <span className="text-[13px] font-medium text-base-100">{providerBadge(p.name)}</span>
                  </div>
                  <span className="font-mono text-[13px] tabular-nums text-base-200">{formatINR(p.total)}</span>
                </div>
              ))}
            </div>
          </Card>
        </div>

        {/* Immutable ledger */}
        <Card>
          <CardHeader
            title="Ledger"
            subtitle="Immutable append-only records — debits, credits, and refunds."
            icon={<Coins className="h-4 w-4" />}
          />
          {ledgerLoading && !ledger ? (
            <div className="space-y-2 px-5 py-4">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-2/3" />
            </div>
          ) : (
            <Table>
              <THead>
                <Th>ID</Th>
                <Th>Kind</Th>
                <Th>Provider</Th>
                <Th>Reservation</Th>
                <Th>Status</Th>
                <Th>Timestamp</Th>
                <Th className="text-right">Amount</Th>
              </THead>
              <tbody>
                {(ledger ?? []).slice(0, 10).map((e) => (
                  <TRow key={e.id}>
                    <Td className="font-mono text-[12px] text-base-400">#{e.id}</Td>
                    <Td>
                      <Badge tone={e.kind === "credit" ? "green" : e.kind === "refund" ? "green" : e.kind === "debit" ? "red" : "blue"}>
                        {e.kind}
                      </Badge>
                    </Td>
                    <Td className="font-mono text-[13px] text-base-200">{providerBadge(e.provider_id || "—")}</Td>
                    <Td className="font-mono text-[12px] text-base-400">{e.reservation || "—"}</Td>
                    <Td className="text-[13px] text-base-300">{e.status}</Td>
                    <Td className="font-mono text-[12px] text-base-400">
                      {new Date(e.timestamp).toLocaleString("en-IN", { hour12: false })}
                    </Td>
                    <Td className={`text-right font-mono text-[13px] tabular-nums ${e.amount > 0 ? "text-pos-400" : e.amount < 0 ? "text-neg-400" : "text-base-400"}`}>
                      {formatINR(e.amount)}
                    </Td>
                  </TRow>
                ))}
                {(!ledger || ledger.length === 0) && (
                  <tr>
                    <td colSpan={7} className="px-5 py-10 text-center text-sm text-base-400">
                      Ledger is empty.
                    </td>
                  </tr>
                )}
              </tbody>
            </Table>
          )}
        </Card>
      </div>
    </Shell>
  );
}