"use client";

import { create } from "zustand";

interface UiState {
  sidebarOpen: boolean;
  walletModalOpen: boolean;
  theme: "dark" | "light";
  toggleSidebar: () => void;
  setWalletModal: (open: boolean) => void;
  setTheme: (theme: "dark" | "light") => void;
}

export const useUi = create<UiState>((set) => ({
  sidebarOpen: false,
  walletModalOpen: false,
  theme: "dark",
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  setWalletModal: (walletModalOpen) => set({ walletModalOpen }),
  setTheme: (theme) => set({ theme }),
}));