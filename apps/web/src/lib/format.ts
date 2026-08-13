// INR (₹) formatting + cost helpers.
// The backend stores money as integer paise; all display values derive from paise.

/** "₹100.00" — Indian number grouping (lakh/crore) via Intl. */
export function formatINR(paise: number): string {
  const validPaise = Number.isNaN(Number(paise)) ? 0 : Number(paise);
  const rupees = validPaise / 100;
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
  m: { price_base_paise: number; price_prompt_paise: number; price_completion_paise: number },
  maxTokens: number,
): number {
  const base = Number(m.price_base_paise) || 0;
  const prompt = Number(m.price_prompt_paise) || 0;
  const completion = Number(m.price_completion_paise) || 0;
  const tokens = Number(maxTokens) || 0;

  return (
    base +
    Math.round((64 * prompt) / 1000) +
    Math.round((tokens * completion) / 1000)
  );
}

/** Session reserve shown at checkout — mirrors the backend reservation. */
export function sessionReserve(
  m: { price_base_paise: number; price_prompt_paise: number; price_completion_paise: number },
  maxTokens: number,
): number {
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
export function providerBadge(providerName?: string): string {
  if (!providerName) {
    return "Unknown";
  }

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