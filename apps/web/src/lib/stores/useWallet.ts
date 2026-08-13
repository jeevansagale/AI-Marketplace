// Global wallet store. Polls the gateway /api/wallet every few seconds so the
// sidebar, playground, and dashboard all reflect live reservations/refunds.
"use client";

import { create } from "zustand";
import React from "react";
import { api, type WalletBalance, type WalletTx } from "../api";

interface WalletState {
  balance: WalletBalance | null;
  transactions: WalletTx[];
  loading: boolean;
  error: string | null;
  lastUpdated: number | null;
  cache: () => void;
  invalidate: () => void;
  refresh: () => Promise<void>;
}

export const useWallet = create<WalletState>((set, get) => ({
  balance: null,
  transactions: [],
  loading: true,
  error: null,
  lastUpdated: null,
  cache: () => {
    // No-op placeholder for future optimistic update caching.
    void get().refresh();
  },
  invalidate: () => {
    void get().refresh();
  },
  refresh: async () => {
    try {
      const [bal, txs] = await Promise.all([
        api.wallet(),
        api.transactions(),
      ]);
      set({
        balance: bal,
        transactions: txs.transactions,
        loading: false,
        error: null,
        lastUpdated: Date.now(),
      });
    } catch (e) {
      set({
        loading: false,
        error: e instanceof Error ? e.message : "wallet load failed",
      });
    }
  },
}));

// Client-side only hook to avoid hydration mismatch
export function useWalletClient() {
  const store = useWallet();
  const [mounted, setMounted] = React.useState(false);
  
  React.useEffect(() => {
    setMounted(true);
  }, []);
  
  if (!mounted) {
    return {
      ...store,
      balance: null,
      loading: true,
    };
  }
  
  return store;
}
