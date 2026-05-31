export type CopyStatus =
  | { ok: true }
  | {
      ok: false;
      reason: "empty" | "unavailable" | "insecure-context" | "rejected";
    };

export async function copyToClipboard(text: string): Promise<CopyStatus> {
  if (!text) {
    return { ok: false, reason: "empty" };
  }

  if (typeof window !== "undefined" && window.isSecureContext === false) {
    return { ok: false, reason: "insecure-context" };
  }

  if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
    return { ok: false, reason: "unavailable" };
  }

  try {
    await navigator.clipboard.writeText(text);
    return { ok: true };
  } catch {
    return { ok: false, reason: "rejected" };
  }
}
