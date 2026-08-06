"use client";

import { useState } from "react";

// Demo wallet credentials (mirror gateway store seed + init SQL):
//   channel: 40000000-0000-0000-0000-000000000001
//   sender:  DEMOWALLET000000000000000000000000000000000
// A real consumer signs the challenge canonical bytes with their keypair and
// attaches the on-chain payment (M2 per Implementation Plan §5.1).
const DEMO_CHANNEL = "40000000-0000-0000-0000-000000000001";
const DEMO_SENDER = "DEMOWALLET000000000000000000000000000000000";

const GATEWAY_URL =
  process.env.NEXT_PUBLIC_GATEWAY_URL ?? "http://localhost:8080";

const DEMO_MODELS = [
  "market-llama-3.3-70b-instruct",
  "market-llama-3.1-8b-instruct",
  "algoforge-deepseek-r1-32b",
];

interface Step {
  label: string;
  detail: string;
  ok: boolean;
}

function base64Encode(obj: unknown): string {
  const json = JSON.stringify(obj);
  if (typeof btoa === "function") return btoa(json);
  return Buffer.from(json, "utf8").toString("base64");
}

async function callChat(
  model: string,
  proofHeader: string | null,
): Promise<Response> {
  return fetch(`${GATEWAY_URL}/v1/chat/completions`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(proofHeader ? { "X-402-Payment-Proof": proofHeader } : {}),
    },
    body: JSON.stringify({
      model,
      messages: [
        { role: "user", content: "Say hello in the style of a blockchain pirate." },
      ],
      stream: false,
    }),
  });
}

export default function PlaygroundPage() {
  const [model, setModel] = useState(DEMO_MODELS[1]);
  const [steps, setSteps] = useState<Step[]>([]);
  const [challengeHdr, setChallengeHdr] = useState<string>("");
  const [balance, setBalance] = useState<string>("100.000000");
  const [spent, setSpent] = useState<string>("0");
  const [output, setOutput] = useState<string>("");
  const [busy, setBusy] = useState(false);

  const addStep = (s: Step) => setSteps((prev) => [...prev, s]);

  async function runDemo() {
    setSteps([]);
    setOutput("");
    setBusy(true);
    try {
      // Step 1 — request without proof → 402 + challenge
      const first = await callChat(model, null);
      const challengeRaw = first.headers.get("X-402-Payment-Required") ?? "";
      addStep({
        label: "POST /v1/chat/completions (no proof)",
        detail: `HTTP ${first.status}`,
        ok: first.status === 402,
      });
      if (first.status !== 402 || !challengeRaw) {
        setOutput("Expected HTTP 402 with an X-402-Payment-Required challenge.");
        return;
      }
      setChallengeHdr(challengeRaw);

      // Step 2 — "pay" with the demo wallet (simulated Algorand USDC + sig)
      const challenge = JSON.parse(atob(challengeRaw));
      const proof = {
        schema: "x402.proof/1.0",
        nonce: challenge.nonce,
        channelId: DEMO_CHANNEL,
        sender: DEMO_SENDER,
        signature: "demo-signature",
        signedAt: new Date().toISOString(),
        hint: "ed25519",
      };
      addStep({
        label: "Sign + pay (demo wallet)",
        detail: `nonce ${challenge.nonce.slice(0, 8)}… · ${challenge.amount} μUSDC`,
        ok: true,
      });

      // Step 3 — retry with the proof → billed + inference
      const second = await callChat(model, base64Encode(proof));
      const bal = second.headers.get("X-402-Balance");
      const spentHdr = second.headers.get("X-402-Spent");
      if (bal) setBalance((Number(bal) / 1_000_000).toFixed(6));
      if (spentHdr) setSpent(spentHdr);
      addStep({
        label: "Retry with X-402-Payment-Proof",
        detail: `HTTP ${second.status}`,
        ok: second.status === 200,
      });

      const json = await second.json();
      const content =
        json?.choices?.[0]?.message?.content ?? JSON.stringify(json, null, 2);
      setOutput(content);
    } catch (err) {
      addStep({ label: "Error", detail: String(err), ok: false });
      setOutput(String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-12">
      <header>
        <p className="text-sm font-medium uppercase tracking-widest text-indigo-400">
          Playground
        </p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">
          Pay-per-use <span className="text-gradient">inference</span>
        </h1>
        <p className="mt-2 max-w-2xl text-sm text-gray-400">
          Watch the HTTP 402 handshake in action: request, get the invoice,
          pay with demo USDC, retry, and receive the completion.
        </p>
      </header>

      <section className="mt-10 grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="rounded-xl border border-gray-800 bg-gray-900/60 p-6">
          <div className="flex flex-wrap items-center gap-3">
            <label className="text-xs font-medium uppercase tracking-wider text-gray-500">
              Model
            </label>
            <select
              value={model}
              onChange={(e) => setModel(e.target.value)}
              className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-indigo-500"
            >
              {DEMO_MODELS.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
            <span className="ml-auto inline-flex items-center gap-2 rounded-full border border-gray-700 px-2.5 py-0.5 text-xs text-emerald-300">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
              {busy ? "paying…" : "live"}
            </span>
          </div>

          <div className="mt-6 flex flex-col gap-3">
            {steps.length === 0 && (
              <p className="text-sm text-gray-500">
                Press{" "}
                <button
                  onClick={runDemo}
                  className="font-semibold text-indigo-400 hover:underline"
                >
                  Send request
                </button>{" "}
                to start the 402 handshake.
              </p>
            )}
            {steps.map((s, i) => (
              <div
                key={i}
                className="flex items-start gap-3 rounded-lg border border-gray-800 bg-gray-800/40 px-4 py-3"
              >
                <span
                  className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold ${
                    s.ok
                      ? "bg-emerald-500/20 text-emerald-300"
                      : "bg-red-500/20 text-red-300"
                  }`}
                >
                  {s.ok ? "✓" : "!"}
                </span>
                <div className="min-w-0">
                  <div className="text-sm font-medium text-gray-200">
                    {s.label}
                  </div>
                  <div className="mt-0.5 truncate font-mono text-xs text-gray-500">
                    {s.detail}
                  </div>
                </div>
              </div>
            ))}
          </div>

          {(challengeHdr || output) && (
            <div className="mt-6">
              {challengeHdr && (
                <div className="mb-3">
                  <div className="text-xs font-medium uppercase tracking-wider text-gray-500">
                    X-402-Payment-Required
                  </div>
                  <pre className="mt-1 max-h-32 overflow-auto rounded-lg bg-gray-950 p-3 font-mono text-[10px] leading-relaxed text-gray-400">
                    {challengeHdr}
                  </pre>
                </div>
              )}
              {output && (
                <div>
                  <div className="text-xs font-medium uppercase tracking-wider text-gray-500">
                    Completion
                  </div>
                  <pre className="mt-1 max-h-64 overflow-auto rounded-lg bg-gray-950 p-3 font-mono text-sm text-emerald-200">
                    {output}
                  </pre>
                </div>
              )}
            </div>
          )}
        </div>

        <aside className="flex flex-col gap-4">
          <div className="rounded-xl border border-gray-800 bg-gray-900/60 p-5">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-gray-500">
              Channel balance
            </h2>
            <div className="mt-2 font-mono text-2xl font-bold text-gray-100">
              ${balance}
            </div>
            <div className="mt-1 text-xs text-gray-500">USDC · testnet</div>
          </div>

          <div className="rounded-xl border border-gray-800 bg-gray-900/60 p-5">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-gray-500">
              This request
            </h2>
            <dl className="mt-3 space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-gray-500">Spent</dt>
                <dd className="font-mono text-gray-200">
                  {Number(spent) / 1_000_000} USDC
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Channel</dt>
                <dd className="font-mono text-[10px] text-gray-400">
                  {DEMO_CHANNEL.slice(0, 16)}…
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Network</dt>
                <dd className="font-mono text-gray-200">algorand-testnet</dd>
              </div>
            </dl>
          </div>

          <button
            onClick={runDemo}
            disabled={busy}
            className="rounded-lg bg-indigo-600 px-5 py-3 font-semibold text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {busy ? "Running handshake…" : "Send request"}
          </button>
        </aside>
      </section>

      <footer className="mt-16 border-t border-gray-800 pt-6 text-center text-xs text-gray-600">
        Demo settlement is simulated in-process — on-chain escrow + state
        channels land in M2.
      </footer>
    </main>
  );
}