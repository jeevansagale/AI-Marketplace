// INR (₹) formatting + cost helpers.
// The backend stores money as integer paise; all display values derive from paise.

/** "₹100.00" — Indian number grouping (lakh/crore) via Intl. */
export function formatINR(paise: number): string {
  const rupees = paise / 100;
  return new Intl.NumberFormat("en-IN", {
    style: "currency",
    currency: "INR",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(rupees);
}

/** Compact "₹5.61" styling used in dense tables/sidebars. */
export function formatINRPaisa(paise: number): string {
  return formatINR(paise);
}

/** Signed display: "-₹2.10" / "+₹0.80" for transaction rows. */
export function formatSigned(paise: number): string {
  const abs = formatINR(Math.abs(paise));
  return paise < 0 ? `−${abs}` : `+${abs}`;
}

/** Estimated session cost for a model: base + prompt estimate (64 tokens) + completion estimate (maxTokens). */
export function estimateSessionCost(
  m: { price_base: number; price_prompt: number; price_completion: number },
  maxTokens: number,
): number {
  return (
    m.price_base +
    Math.round((64 * m.price_prompt) / 1000) +
    Math.round((maxTokens * m.price_completion) / 1000)
  );
}

/** Session reserve shown at checkout — mirrors the backend reservation. */
export function sessionReserve(m: {
  price_base: number;
  price_prompt: number;
  price_completion: number;
}, maxTokens: number): number {
  return estimateSessionCost(m, maxTokens);
}

/** Human label for a session timeline stage. */
export function stageLabel(stage: string): string {
  switch (stage) {
    case "requested":
      return "Request";
    case "authorized":
      return "Wallet Authorization";
    case "reserved":
      return "Reservation Created";
    case "streaming":
      return "Streaming";
    case "completed":
      return "Completed";
    case "settled":
      return "Settlement";
    case "refunded":
      return "Refund";
    case "failed":
      return "Failed";
    default:
      return stage;
  }
}

/** Provider display name (short). */
export function providerBadge(providerName: string): string {
  switch (providerName.toLowerCase()) {
    case "openai":
      return "OpenAI";
    case "google":
    case "gemini":
      return "Gemini";
    default:
      return providerName;
  }
}