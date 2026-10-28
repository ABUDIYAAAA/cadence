import React from "react";
import { cn } from "@/lib/utils";

export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {}

export function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-[12px] bg-[var(--card-line)]",
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
