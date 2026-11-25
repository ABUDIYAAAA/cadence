import React from "react";
import { cn } from "@/lib/utils";

export type SkeletonProps = React.HTMLAttributes<HTMLDivElement>;

export function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-[8px] bg-[var(--card-line)]/75",
        className
      )}
      {...props}
    />
  );
}

export function SkeletonText({
  lines = 3,
  className,
}: {
  lines?: number;
  className?: string;
}) {
  return (
    <div className={cn("space-y-2.5", className)}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          className={cn("h-4", i === lines - 1 ? "w-2/3" : "w-full")}
        />
      ))}
    </div>
  );
}

export function SkeletonCard({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] p-6 space-y-4 shadow-[0_4px_16px_-2px_rgba(24,32,48,0.04)]",
        className
      )}
    >
      <div className="flex justify-between items-center pb-3.5 border-b border-[var(--card-line)]">
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-5 w-16 rounded-full" />
      </div>
      <SkeletonText lines={3} />
      <div className="pt-3.5 border-t border-[var(--card-line)] flex justify-end gap-2">
        <Skeleton className="h-9 w-24 rounded-[14px]" />
      </div>
    </div>
  );
}

/** Full-width skeleton matching the Overview (/dashboard/dev, /dashboard/admin) layout. */
export function DashboardOverviewSkeleton() {
  return (
    <div className="font-sans space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
        <div className="space-y-1.5">
          <Skeleton className="h-3.5 w-32" />
          <Skeleton className="h-7 w-44" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-9 w-28 rounded-[10px]" />
          <Skeleton className="h-9 w-24 rounded-[10px]" />
          <Skeleton className="h-9 w-28 rounded-[10px]" />
        </div>
      </div>

      <div className="rounded-[20px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 sm:p-6 space-y-5">
        <div className="flex flex-col lg:flex-row lg:items-end justify-between gap-6">
          <div className="space-y-2">
            <Skeleton className="h-3.5 w-24" />
            <Skeleton className="h-12 w-40" />
          </div>
          <div className="grid grid-cols-3 gap-6">
            {[0, 1, 2].map((i) => (
              <div key={i} className="space-y-1.5">
                <Skeleton className="h-3 w-14" />
                <Skeleton className="h-6 w-10" />
              </div>
            ))}
          </div>
        </div>
        <Skeleton className="h-2 w-full rounded-full" />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {[0, 1, 2].map((i) => (
          <div
            key={i}
            className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 space-y-2.5"
          >
            <div className="flex items-center justify-between">
              <Skeleton className="h-3.5 w-20" />
              <Skeleton className="h-4 w-4 rounded-[4px]" />
            </div>
            <Skeleton className="h-7 w-14" />
            <Skeleton className="h-3 w-32" />
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <div className="xl:col-span-2 rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 space-y-4">
          <div className="flex items-center justify-between">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-3.5 w-16" />
          </div>
          <div className="divide-y divide-[var(--card-line)]">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="flex items-center justify-between gap-4 py-2.5">
                <div className="space-y-1.5 flex-1">
                  <Skeleton className="h-3.5 w-3/5" />
                  <Skeleton className="h-3 w-2/5" />
                </div>
                <Skeleton className="h-3 w-12 shrink-0" />
              </div>
            ))}
          </div>
        </div>
        <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 space-y-4">
          <div className="flex items-center justify-between">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-3.5 w-14" />
          </div>
          <div className="divide-y divide-[var(--card-line)]">
            {[0, 1, 2].map((i) => (
              <div key={i} className="flex items-center justify-between gap-3 py-2.5">
                <div className="flex items-center gap-3 flex-1">
                  <Skeleton className="h-8 w-8 rounded-[6px] shrink-0" />
                  <div className="space-y-1.5 flex-1">
                    <Skeleton className="h-3.5 w-2/3" />
                    <Skeleton className="h-3 w-1/3" />
                  </div>
                </div>
                <Skeleton className="h-4 w-4 shrink-0" />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/** Full-width skeleton matching table-based admin pages (Users, Audit, Events). */
export function AdminTablePageSkeleton({
  showMetrics = false,
}: {
  showMetrics?: boolean;
}) {
  return (
    <div className="font-sans space-y-6">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="space-y-2">
          <div className="flex items-center gap-3">
            <Skeleton className="h-7 w-36" />
            <Skeleton className="h-5 w-16 rounded-full" />
          </div>
          <Skeleton className="h-4 w-72" />
        </div>
        <div className="flex items-center gap-2">
          <Skeleton className="h-9 w-24 rounded-[8px]" />
          <Skeleton className="h-9 w-28 rounded-[8px]" />
        </div>
      </div>

      {showMetrics && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3.5">
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] space-y-2"
            >
              <Skeleton className="h-3 w-24" />
              <Skeleton className="h-6 w-12" />
            </div>
          ))}
        </div>
      )}

      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        <div className="flex items-center justify-between gap-3">
          <Skeleton className="h-9 w-full max-w-md rounded-[8px]" />
          <Skeleton className="h-9 w-24 rounded-[8px] hidden sm:block" />
        </div>
        <div className="divide-y divide-[var(--card-line)]">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="flex items-center gap-4 py-3">
              <Skeleton className="h-8 w-8 rounded-[8px] shrink-0" />
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/5 hidden sm:block" />
              <Skeleton className="h-5 w-16 rounded-full" />
              <Skeleton className="h-4 w-20 ml-auto" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
