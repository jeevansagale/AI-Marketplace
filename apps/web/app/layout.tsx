import type { Metadata } from "next";
import "../styles/globals.css";

export const metadata: Metadata = {
  title: "AI Marketplace — Pay-per-use inference on Algorand",
  description:
    "OpenAI-compatible AI APIs with native x402 machine-to-machine payments on Algorand.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="antialiased">{children}</body>
    </html>
  );
}