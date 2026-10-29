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
