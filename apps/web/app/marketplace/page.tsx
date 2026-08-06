"use client";

import { useQuery } from "@tanstack/react-query";
import { fetchCatalog, microToUsdc, type Catalog, type Model } from "@/lib/api";
import { QueryProvider } from "@/providers/QueryProvider";

const CATEGORY_LABELS: Record<string, string> = {
  chat: "Chat",
  vision: "Vision",
  ocr: "OCR",
  embeddings: "Embeddings",
  audio: "Audio",
};

function categoryColor(cat: string): string {
  switch (cat) {
    case "chat":
      return "bg-indigo-500/10 text-indigo-300 border-indigo-500/30";
    case "vision":
    case "ocr":
      return "bg-violet-500/10 text-violet-300 border-violet-500/30";
    case "embeddings":
      return "bg-emerald-500/10 text-emerald-300 border-emerald-500/30";
    case "audio":
      return "bg-amber-500/10 text-amber-300 border-amber-500/30";
    default:
      return "bg-gray-500/10 text-gray-300 border-gray-500/30";
  }
}

function ModelCard({ model }: { model: Model }) {
  const input = microToUsdc(model.price_prompt_micro_usdc);
  const output = microToUsdc(model.price_completion_micro_usdc);
  return (
    <div className="flex flex-col rounded-xl border border-gray-800 bg-gray-900/60 p-5 transition hover:border-indigo-500/50 hover:bg-gray-900">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="font-mono text-sm font-semibold text-indigo-200">
            {model.slug}
          </h3>
          <p className="mt-0.5 text-xs text-gray-400">{model.name}</p>
        </div>
        <span
          className={`shrink-0 rounded-full border px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide ${categoryColor(
            model.category,
          )}`}
        >
          {CATEGORY_LABELS[model.category] ?? model.category}
        </span>
      </div>

      <p className="mt-3 flex-1 text-sm leading-relaxed text-gray-400">
        {model.description}
      </p>

      <div className="mt-4 grid grid-cols-2 gap-2 text-xs">
        <div className="rounded-lg bg-gray-800/60 px-3 py-2">
          <div className="text-gray-500">Input</div>
          <div className="mt-0.5 font-mono font-semibold text-gray-200">
            ${input}/1K tok
          </div>
        </div>
        <div className="rounded-lg bg-gray-800/60 px-3 py-2">
          <div className="text-gray-500">Output</div>
          <div className="mt-0.5 font-mono font-semibold text-gray-200">
            ${output}/1K tok
          </div>
        </div>
        <div className="rounded-lg bg-gray-800/60 px-3 py-2">
          <div className="text-gray-500">p90 latency</div>
          <div className="mt-0.5 font-mono font-semibold text-gray-200">
            {model.p90_latency_ms} ms
          </div>
        </div>
        <div className="rounded-lg bg-gray-800/60 px-3 py-2">
          <div className="text-gray-500">Context</div>
          <div className="mt-0.5 font-mono font-semibold text-gray-200">
            {(model.context_window / 1024).toFixed(0)}K
          </div>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between text-xs">
        <span className="text-gray-500">by {model.provider}</span>
        <span className="inline-flex items-center gap-2 rounded-full border border-gray-700 px-2.5 py-0.5 font-medium text-emerald-300">
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
          x402
        </span>
      </div>
    </div>
  );
}

function MarketplacePage() {
  const { data, isPending, isError, refetch } = useQuery<Catalog>({
    queryKey: ["catalog"],
    queryFn: fetchCatalog,
  });

  return (
    <main className="mx-auto max-w-7xl px-6 py-12">
      <header className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-sm font-medium uppercase tracking-widest text-indigo-400">
            Marketplace
          </p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">
            Pay-per-use <span className="text-gradient">AI models</span>
          </h1>
          <p className="mt-2 max-w-2xl text-sm text-gray-400">
            Every request is settled on Algorand with native x402 payment
            proofs. Same OpenAI SDK — new base URL.
          </p>
        </div>
        {data && (
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <span className="h-2 w-2 rounded-full bg-emerald-400" />
            {data.models.filter((m) => m.is_active).length} active models ·{" "}
            {data.providers.filter((p) => p.status === "active").length}{" "}
            providers
          </div>
        )}
      </header>

      {isPending && (
        <p className="mt-16 text-center text-sm text-gray-500">
          Loading catalog…
        </p>
      )}

      {isError && (
        <div className="mt-16 rounded-xl border border-red-500/30 bg-red-500/5 p-6 text-center">
          <p className="text-sm text-red-300">
            Could not reach the marketplace catalog.
          </p>
          <button
            onClick={() => refetch()}
            className="mt-3 rounded-lg border border-gray-700 px-4 py-2 text-xs font-medium text-gray-200 transition hover:border-gray-500"
          >
            Retry
          </button>
        </div>
      )}

      {data && (
        <div className="mt-10 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {data.models
            .filter((m) => m.is_active)
            .map((m) => (
              <ModelCard key={m.id} model={m} />
            ))}
        </div>
      )}

      <footer className="mt-16 border-t border-gray-800 pt-6 text-center text-xs text-gray-600">
        Base URL{" "}
        <code className="rounded bg-gray-800 px-2 py-0.5 text-gray-400">
          http://localhost:8080/v1
        </code>{" "}
        · settle on Algorand testnet · updated {new Date().toISOString()}
      </footer>
    </main>
  );
}

export default function Page() {
  return (
    <QueryProvider>
      <MarketplacePage />
    </QueryProvider>
  );
}