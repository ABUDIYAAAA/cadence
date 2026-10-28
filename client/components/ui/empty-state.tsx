import React from "react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

export function EmptyState({
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center border border-dashed border-neutral-300 p-8 text-center font-mono",
        className
      )}
    >
      <div className="mb-2 text-2xl font-light text-neutral-400">[—]</div>
      <h4 className="text-sm font-semibold uppercase tracking-wider text-black">
        {title}
      </h4>
      {description && (
        <p className="mt-1 max-w-sm text-xs text-neutral-500">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
