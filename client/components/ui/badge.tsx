import React from "react";
import { cn } from "@/lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "solid" | "outline" | "subtle" | "blue" | "coral" | "emerald";
  size?: "sm" | "md";
  dot?: boolean;
}

export function Badge({
  className,
  variant = "subtle",
  size = "md",
  dot = false,
  children,
  ...props
}: BadgeProps) {
  const variantStyles = {
    subtle:
      "bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] border border-[var(--card-line)]",
    solid:
      "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-[var(--shadow-pill)]",
    outline:
      "bg-transparent text-[var(--ink-secondary)] border border-[var(--card-line)]",
    blue:
      "bg-[var(--color-blue-dim)] text-[var(--color-blue)] border border-[var(--color-blue-border)]",
    coral:
      "bg-[var(--color-coral-dim)] text-[var(--color-coral)] border border-[var(--color-coral-border)]",
    emerald:
      "bg-[var(--color-emerald-dim)] text-[var(--color-emerald)] border border-[var(--color-emerald-border)]",
  };

  const sizeStyles = {
    sm: "px-2 py-0.5 text-[11px] gap-1.5",
    md: "px-2.5 py-1 text-xs gap-1.5",
  };

  const dotColors = {
    blue: "bg-[var(--color-blue)]",
    coral: "bg-[var(--color-coral)]",
    emerald: "bg-[var(--color-emerald)]",
    solid: "bg-[var(--ink-inverse)]",
    subtle: "bg-[var(--ink-secondary)]",
    outline: "bg-[var(--ink-secondary)]",
  };

  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full font-medium select-none transition-colors",
        variantStyles[variant],
        sizeStyles[size],
        className
      )}
      {...props}
    >
      {dot && (
        <span
          className={cn(
            "h-1.5 w-1.5 rounded-full shrink-0",
            dotColors[variant]
          )}
        />
      )}
      {children}
    </div>
  );
}
