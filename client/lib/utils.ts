import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

/** Whole-number percent (0-100) from the API's `attendance_rate`, which is already a percentage. */
export function attendancePercent(rate: number | null | undefined): number {
  if (!Number.isFinite(rate)) return 0;
  return Math.min(100, Math.max(0, Math.round(rate as number)));
}

/** Readable message from an API error ({ error } from the API client) or a thrown Error. */
export function apiErrorMessage(err: unknown, fallback: string): string {
  if (err && typeof err === "object") {
    const e = err as { error?: unknown; message?: unknown };
    if (typeof e.error === "string" && e.error) return e.error;
    if (typeof e.message === "string" && e.message) return e.message;
  }
  return fallback;
}

/** Split a comma-, semicolon-, or whitespace-separated domain input into normalized, deduplicated domains. */
export function parseEmailDomains(raw: string): string[] {
  const seen = new Set<string>();
  for (const part of raw.split(/[\s,;]+/)) {
    const d = part.trim().toLowerCase().replace(/^@+/, "");
    if (d) seen.add(d);
  }
  return Array.from(seen);
}

/** True when `email` matches any domain (or subdomain) in `excludeDomains`, mirroring server utils.IsEmailExcluded. */
export function isEmailExcluded(email: string, excludeDomains: string[]): boolean {
  const normalizedEmail = email.trim().toLowerCase();
  return excludeDomains.some((raw) => {
    const d = raw.trim().toLowerCase().replace(/^@+/, "");
    return Boolean(d) && (normalizedEmail.endsWith("@" + d) || normalizedEmail.endsWith("." + d));
  });
}
