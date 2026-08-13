// App shell: top nav with live wallet balance + top-up modal.
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
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
import { TopUpPaymentAnimation, type TopUpPhase } from "./TopUpPaymentAnimation";

const nav = [
  { href: "/", label: "Overview", icon: LayoutDashboard, exact: true },
  { href: "/marketplace", label: "Marketplace", icon: Boxes },
  { href: "/playground", label: "Playground", icon: Play },
  { href: "/wallet", label: "Wallet", icon: Wallet },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const { balance, loading, error, refresh } = useWalletClient();
  const [openTopUp, setOpenTopUp] = useState(false);
  const pathname = usePathname();
  const reduceMotion = useReducedMotion();

  // Initial fetch on mount.
  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="min-h-screen bg-base-950 text-base-100 font-sans antialiased selection:bg-accent-500/20 selection:text-accent-400">
      {/* Network banner */}
      <div className="border-b border-base-800/80 bg-base-900/60 backdrop-blur-md px-4 py-1.5 text-xs text-base-400">
        <div className="mx-auto flex max-w-7xl items-center justify-between">
          <span className="flex items-center gap-2 font-mono text-[11px] text-base-300">
            <Radio className="h-3 w-3 text-pos-400 animate-pulse" />
            <span className="text-base-400">OMNI-ROUTE</span>
            <span className="text-base-600">/</span>
            <span>SETTLEMENT CHANNEL OPEN</span>
          </span>
          <span className="hidden font-mono text-[11px] text-base-400 sm:inline">
            x402 Pay-per-session · INR Prepaid
          </span>
        </div>
      </div>

      {/* Main navigation */}
      <header className="sticky top-0 z-40 border-b border-base-800/80 bg-base-950/80 backdrop-blur-xl">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 sm:px-6 h-14">
          <div className="flex items-center gap-8">
            <Link
              href="/"
              className="flex items-center gap-2 font-mono text-sm font-semibold tracking-tight text-base-100 transition-opacity hover:opacity-90 focus-ring rounded-md"
            >
              <span className="flex h-7 w-7 items-center justify-center rounded-md bg-accent-500/10 text-accent-400 border border-accent-500/20 font-bold">
                X
              </span>
              <span>AI Marketplace</span>
            </Link>

            <nav className="flex items-center gap-1" aria-label="Main navigation">
              {nav.map((item) => {
                const active = item.exact
                  ? pathname === item.href
                  : pathname === item.href || pathname.startsWith(item.href + "/");
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`relative flex items-center gap-2 rounded-md px-3 py-1.5 text-xs font-medium transition-[color,background-color] duration-150 ease-out focus-ring ${
                      active
                        ? "text-base-100"
                        : "text-base-400 hover:bg-base-900/60 hover:text-base-200"
                    }`}
                  >
                    {active && (
                      <motion.div
                        layoutId={reduceMotion ? undefined : "nav-active"}
                        className="absolute inset-0 rounded-md bg-base-800/80 border border-base-700/50"
                        transition={{ type: "spring", stiffness: 400, damping: 30 }}
                      />
                    )}
                    <span className="relative z-10 flex items-center gap-2">
                      <Icon className={`h-3.5 w-3.5 ${active ? "text-accent-400" : "text-base-400"}`} />
                      {item.label}
                    </span>
                  </Link>
                );
              })}
            </nav>
          </div>

          {/* Right side: Wallet balance & Top Up button */}
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2 rounded-lg border border-base-800 bg-base-900/60 px-3 py-1.5 text-xs">
              <Wallet className="h-3.5 w-3.5 text-base-400" />
              {loading && !balance ? (
                <Skeleton className="h-3.5 w-16" />
              ) : error ? (
                <span className="text-neg-400 text-[11px]">Offline</span>
              ) : (
                <span className="font-mono font-medium text-base-100">
                  {balance ? formatINR(balance.available_paise) : "₹0.00"}
                </span>
              )}
            </div>

            <Button
              size="sm"
              onClick={() => setOpenTopUp(true)}
              className="gap-1.5 font-medium shadow-sm"
            >
              <Plus className="h-3.5 w-3.5" />
              Top up
            </Button>
          </div>
        </div>
      </header>

      {/* Main page content container with smooth transition */}
      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
        <PageTransition>{children}</PageTransition>
      </main>

      {/* Top up modal */}
      {openTopUp && (
        <TopUpModal
          onClose={() => setOpenTopUp(false)}
          onDone={() => void refresh()}
        />
      )}
    </div>
  );
}

/* ---------- Top-up modal (v2 payment abstraction with animation) ---------- */
export function TopUpModal({
  onClose,
  onDone,
}: {
  onClose: () => void;
  onDone: () => void;
}) {
  const [amount, setAmount] = useState(100);
  const [step, setStep] = useState<"amount" | "paying" | "done" | "error">("amount");
  const [payPhase, setPayPhase] = useState<TopUpPhase>("processing");
  const [message, setMessage] = useState("");
  const [orderId, setOrderId] = useState("");

  const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

  const start = async () => {
    setStep("paying");
    setPayPhase("processing");
    setMessage("");

    const verified = (async () => {
      const order = await api.topup(amount);
      setOrderId(order.order_id);
      await api.topupVerify(order.order_id);
    })();

    try {
      await sleep(200);
      setPayPhase("routing");
      await sleep(350);
      setPayPhase("verifying");

      await verified;

      setPayPhase("success");
      onDone();
      await sleep(650);
      setStep("done");
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
          <AnimatePresence mode="wait">
            <motion.div
              key={step}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
              className="space-y-4"
            >
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
                <TopUpPaymentAnimation
                  amountPaise={amount * 100}
                  phase={payPhase}
                  orderId={orderId}
                />
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
            </motion.div>
          </AnimatePresence>
        </div>
      </Card>
    </div>
  );
}
