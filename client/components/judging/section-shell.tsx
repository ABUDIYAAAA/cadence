"use client";

import React from "react";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * Shared building blocks for every Judging tab so spacing, type scale and
 * hierarchy stay identical: toolbar -> metrics -> primary surface.
 */

export function SectionToolbar({
  title,
  subtitle,
  onRefresh,
  isFetching,
  children,
}: {
  title: string;
  subtitle?: string;
  onRefresh?: () => void;
  isFetching?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div className="min-w-0">
        <div className="text-sm font-semibold text-[var(--ink-primary)]">{title}</div>
        {subtitle && <div className="text-xs text-[var(--ink-secondary)] mt-0.5 truncate">{subtitle}</div>}
      </div>
      <div className="flex items-center gap-2 flex-wrap">
        {onRefresh && (
          <Button
            variant="outline"
            size="sm"
            onClick={onRefresh}
            disabled={isFetching}
            className="flex items-center gap-1.5 text-xs h-9"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        )}
        {children}
      </div>
    </div>
  );
}

export function MetricGrid({
  children,
  cols = 4,
  className = "",
}: {
  children: React.ReactNode;
  cols?: 3 | 4;
  className?: string;
}) {
  const colsClass = cols === 3 ? "grid-cols-3" : "grid-cols-2 sm:grid-cols-4";
  return <div className={`grid ${colsClass} gap-3 ${className}`}>{children}</div>;
}

export function Metric({
  label,
  value,
  suffix,
  note,
  valueClassName = "text-[var(--ink-primary)]",
}: {
  label: string;
  value: React.ReactNode;
  suffix?: string;
  note?: string;
  valueClassName?: string;
}) {
  return (
    <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5 min-w-0">
      <div className="text-xs text-[var(--ink-secondary)] font-medium">{label}</div>
      <div className={`text-2xl font-bold font-mono mt-1 truncate ${valueClassName}`}>
        {value}
        {suffix && <span className="text-sm font-normal text-[var(--ink-tertiary)]"> {suffix}</span>}
      </div>
      {note && <div className="text-[11px] text-[var(--ink-secondary)] mt-0.5 truncate">{note}</div>}
    </div>
  );
}

export function Surface({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4 ${className}`}>
      {children}
    </div>
  );
}
