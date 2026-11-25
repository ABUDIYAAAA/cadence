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
        "flex flex-col items-center justify-center rounded-[12px] border border-dashed border-[var(--card-line)] bg-[var(--card-solid-subtle)]/60 px-6 py-10 text-center font-sans",
        className
      )}
    >
      <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-secondary)]">
        {Icon ? <Icon className="h-5 w-5" /> : <Inbox className="h-5 w-5" />}
      </div>
      <h4 className="text-sm font-semibold tracking-tight text-[var(--ink-primary)]">
        {title}
      </h4>
      {description && (
        <p className="mt-1 max-w-sm text-xs text-[var(--ink-secondary)] leading-relaxed">
          {description}
        </p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
