// App shell: top nav with live wallet balance + top-up modal.
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { motion, useReducedMotion } from "framer-motion";
import {
  Plus,
  Wallet,
  Boxes,
  Play,
  LayoutDashboard,
  Radio,
  X,
} from "lucide-react";
import { Button, Card, Badge, Skeleton } from "./ui";
import { useWalletClient } from "@/lib/stores/useWallet";
import { api } from "@/lib/api";
import { formatINR } from "@/lib/format";
import { PageTransition } from "./PageTransition";

const nav = [
  { href: "/", label: "Overview", icon: LayoutDashboard, exact: true },
  { href: "/marketplace", label: "Marketplace", icon: Boxes },
  { href: "/playground", label: "Playground", icon: Play },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const { balance, loading, error, refresh } = useWalletClient();
  const [topupOpen, setTopupOpen] = useState(false);
  const pathname = usePathname();
  const reduceMotion = useReducedMotion();

  // Live balance refresh every 5s (reservations + refunds land back).
  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 5000);
    return () => clearInterval(t);
  }, [refresh]);

  return (
    <div className="min-h-screen bg-base-950">
      <header className="sticky top-0 z-30 border-b border-base-700 bg-base-950/90 backdrop-blur-md">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4 sm:gap-6 sm:px-6">
          {/* Brand */}
          <Link href="/" className="flex min-w-0 items-center gap-2.5 rounded-md focus-ring">
            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-accent-500 text-white shadow-[0_1px_1px_rgb(15_23_42_/_0.12)]">
              <Radio className="h-4 w-4" />
            </div>
            <span className="truncate text-sm font-semibold text-base-100">
              x402 Marketplace
            </span>
          </Link>

          {/* Nav */}
          <nav className="hidden items-center gap-1 md:flex">
            {nav.map(({ href, label, icon: Icon, exact }) => (
              <Link
                key={href}
                href={href}
                aria-current={(exact ? pathname === href : pathname.startsWith(href)) ? "page" : undefined}
                className={`inline-flex items-center gap-2 rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors duration-150 ease-out focus-ring ${
                  (exact ? pathname === href : pathname.startsWith(href))
                    ? "bg-base-900 text-base-100 shadow-[inset_0_0_0_1px_var(--border-subtle)]"
                    : "text-base-300 hover:bg-base-800 hover:text-base-100"
                }`}
              >
                <Icon className="h-3.5 w-3.5" />
                {label}
              </Link>
            ))}
          </nav>

          <div className="ml-auto flex items-center gap-3">
            {/* Live wallet pill */}
            <Link
              href="/wallet"
              className="group flex min-w-0 items-center gap-2.5 rounded-md border border-base-700 bg-base-900 px-3 py-1.5 transition-[border-color,box-shadow,background-color] duration-150 ease-out hover:border-base-600 hover:bg-base-850 focus-ring"
            >
              <Wallet className="h-3.5 w-3.5 shrink-0 text-base-400" />
              {loading && !balance ? (
                <Skeleton className="h-4 w-20" />
              ) : balance ? (
                <span className="truncate font-mono text-[13px] font-medium tabular-nums text-base-100">
                  {formatINR(balance.available_paise ?? 0)}
                </span>
              ) : (
                <span className="text-[13px] text-base-400">{error ?? "—"}</span>
              )}
              {balance && balance.reserved_paise > 0 && (
                <span className="font-mono text-[11px] text-warn-400">
                  ({formatINR(balance.reserved_paise)} held)
                </span>
              )}
            </Link>

            <Button size="sm" onClick={() => setTopupOpen(true)}>
              <Plus className="h-3.5 w-3.5" />
              Top up
            </Button>
          </div>
        </div>
        <nav className="mx-auto flex max-w-7xl gap-1 overflow-x-auto px-4 pb-2 md:hidden">
          {nav.map(({ href, label, icon: Icon, exact }) => {
            const active = exact ? pathname === href : pathname.startsWith(href);
            return (
              <Link
                key={href}
                href={href}
                aria-current={active ? "page" : undefined}
                className={`inline-flex h-8 shrink-0 items-center gap-2 rounded-md px-3 text-[13px] font-medium transition-colors duration-150 ease-out focus-ring ${
                  active
                    ? "bg-base-900 text-base-100 shadow-[inset_0_0_0_1px_var(--border-subtle)]"
                    : "text-base-300 hover:bg-base-800 hover:text-base-100"
                }`}
              >
                <Icon className="h-3.5 w-3.5" />
                {label}
              </Link>
            );
          })}
        </nav>
      </header>

      <PageTransition>
        <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
          {children}
        </main>
      </PageTransition>

      {topupOpen && (
        <TopUpModal
          onClose={() => setTopupOpen(false)}
          onDone={() => {
            void refresh();
          }}
        />
      )}
    </div>
  );
}

/* ---------- Top-up modal (v2 payment abstraction) ---------- */
export function TopUpModal({
  onClose,
  onDone,
}: {
  onClose: () => void;
  onDone: () => void;
}) {
  const [amount, setAmount] = useState(100);
  const [step, setStep] = useState<"amount" | "paying" | "done" | "error">("amount");
  const [message, setMessage] = useState("");
  const [orderId, setOrderId] = useState("");

  const start = async () => {
    setStep("paying");
    setMessage("");
    try {
      const order = await api.topup(amount);
      setOrderId(order.order_id);
      // Simulate the client payment sheet completing (UPI/Razorpay).
      setStep("paying");
      await new Promise((r) => setTimeout(r, 800));
      await api.topupVerify(order.order_id);
      setStep("done");
      onDone();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "top-up failed");
      setStep("error");
    }
  };

  const preset = [100, 200, 500, 1000];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-[#0f172a]/45 p-4 backdrop-blur-[2px]"
      onClick={() => step !== "paying" && onClose()}
      role="presentation"
    >
      <Card
        className="surface-raised w-full max-w-sm overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="topup-title"
      >
        <div className="px-5 py-4">
          <div className="flex items-center justify-between">
            <h3 id="topup-title" className="text-sm font-semibold text-base-100">Top up wallet</h3>
            <button
              onClick={onClose}
              className="rounded-md p-1.5 text-base-400 transition-colors duration-150 ease-out hover:bg-base-800 hover:text-base-100 focus-ring"
              aria-label="Close"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <p className="mt-1 text-xs leading-relaxed text-base-400">
            Real payments via the provider abstraction — verify callback before credit.
          </p>
        </div>

        <div className="space-y-4 px-5 pb-5">
          {step === "amount" && (
            <>
              <div className="grid grid-cols-4 gap-2">
                {preset.map((p) => (
                  <button
                    key={p}
                    onClick={() => setAmount(p)}
                    className={`rounded-md border px-2 py-2 font-mono text-[13px] font-medium transition-[border-color,background-color,color,transform] duration-150 ease-out active:scale-[0.98] focus-ring ${
                      amount === p
                        ? "border-accent-500/50 bg-accent-500/10 text-accent-400"
                        : "border-base-700 bg-base-900 text-base-200 hover:border-base-600 hover:bg-base-850"
                    }`}
                  >
                    ₹{p}
                  </button>
                ))}
              </div>
              <label className="block">
                <span className="text-xs text-base-400">Amount</span>
                <div className="field mt-1 flex items-center gap-2 rounded-md px-3">
                  <span className="text-sm text-base-400">₹</span>
                  <input
                    type="number"
                    min={10}
                    value={amount}
                    onChange={(e) => setAmount(Number(e.target.value))}
                    className="h-9 w-full bg-transparent font-mono text-sm outline-none"
                  />
                </div>
              </label>
              <Button className="w-full" onClick={start} loading={false}>
                Continue to payment
              </Button>
            </>
          )}

          {step === "paying" && (
            <div className="flex flex-col items-center py-6">
              <Badge tone="blue" dot>
                Payment in progress
              </Badge>
              <p className="mt-3 font-mono text-sm text-base-200">
                {formatINR(amount * 100)}
              </p>
              <p className="mt-1 text-xs text-base-400">
                Order {orderId || "…"} — waiting for provider verification
              </p>
            </div>
          )}

          {step === "done" && (
            <div className="flex flex-col items-center py-4">
              <Badge tone="green" dot>
                Wallet credited
              </Badge>
              <p className="mt-3 text-sm text-base-200">
                {formatINR(amount * 100)} added to balance
              </p>
              <Button className="mt-4 w-full" onClick={onClose}>
                Done
              </Button>
            </div>
          )}

          {step === "error" && (
            <div className="flex flex-col items-center py-4">
              <Badge tone="red">Top-up failed</Badge>
              <p className="mt-3 text-sm text-base-200">{message}</p>
              <Button variant="secondary" className="mt-4 w-full" onClick={() => setStep("amount")}>
                Try again
              </Button>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
}
