export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col items-center justify-center px-6">
      <h1 className="text-4xl font-bold tracking-tight sm:text-6xl">
        Pay-per-use <span className="text-gradient">AI Marketplace</span>
      </h1>
      <p className="mt-4 max-w-2xl text-center text-lg text-gray-400">
        OpenAI-compatible AI inference with native x402 machine-to-machine
        payments on Algorand. Change your base URL — keep your SDK.
      </p>
      <div className="mt-8 flex flex-col gap-4 sm:flex-row">
        <a
          href="/marketplace"
          className="rounded-lg bg-indigo-600 px-6 py-3 font-semibold text-white transition hover:bg-indigo-500"
        >
          Browse Marketplace
        </a>
        <a
          href="/playground"
          className="rounded-lg border border-gray-700 px-6 py-3 font-semibold text-gray-200 transition hover:border-gray-500"
        >
          Open Playground
        </a>
      </div>
      <p className="mt-12 text-sm text-gray-500">
        Base URL: <code className="rounded bg-gray-800 px-2 py-0.5">http://localhost:8080/v1</code>
      </p>
    </main>
  );
}