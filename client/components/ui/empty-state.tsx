import React from "react";
import { cn } from "@/lib/utils";
import { Inbox } from "lucide-react";

interface EmptyStateProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
}

export function EmptyState({
  title,
  description,
  action,
  icon: Icon,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center rounded-[20px] border border-dashed border-[var(--color-blue-border)] bg-[var(--card-solid)]/70 backdrop-blur-md p-10 text-center font-sans",
        className
      )}
    >
      <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-[var(--color-blue-dim)] text-[var(--color-blue)]">
        {Icon ? <Icon className="h-6 w-6" /> : <Inbox className="h-6 w-6" />}
      </div>
      <h4 className="text-sm font-extrabold tracking-tight text-[var(--ink-primary)]">
        {title}
      </h4>
      {description && (
        <p className="mt-1.5 max-w-sm text-xs text-[var(--ink-secondary)] leading-relaxed">
          {description}
        </p>
      )}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
