import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "../styles/globals.css";

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono-jb" });

export const metadata: Metadata = {
  title: "x402 Marketplace — Pay-per-use AI inference",
  description:
    "Discover AI providers, top up an INR wallet, and consume streaming AI services with HTTP 402-inspired payment authorization.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={`${inter.variable} ${mono.variable}`}>
      <body className={`${inter.variable} antialiased`}>{children}</body>
    </html>
  );
}