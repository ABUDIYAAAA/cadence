import React from "react";
import { cn } from "@/lib/utils";

export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {}

export function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      className={cn(
        "animate-pulse bg-neutral-200 border border-neutral-300 font-mono",
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
    <div className={cn("space-y-2", className)}>
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
        "border border-neutral-300 bg-white p-5 space-y-4 font-mono",
        className
      )}
    >
      <div className="flex justify-between items-center pb-3 border-b border-neutral-200">
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-4 w-16" />
      </div>
      <SkeletonText lines={4} />
      <div className="pt-3 border-t border-neutral-200 flex justify-end gap-2">
        <Skeleton className="h-8 w-20" />
      </div>
    </div>
  );
}
