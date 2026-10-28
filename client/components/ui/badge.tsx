import React from "react";
import { cn } from "@/lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "solid" | "outline" | "subtle" | "blue" | "coral";
  size?: "sm" | "md";
}

export function Badge({
  className,
  variant = "subtle",
  size = "md",
  ...props
}: BadgeProps) {
  const variantStyles = {
    solid: "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-[var(--shadow-pill)]",
    outline: "bg-transparent text-[var(--ink-secondary)] border border-[var(--card-line)]",
    subtle: "bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] border border-[var(--card-line)]",
    blue: "bg-[var(--color-blue)] text-white shadow-[0_4px_12px_rgba(131,162,219,0.3)]",
    coral: "bg-[var(--color-coral)] text-white shadow-[0_4px_12px_rgba(206,105,105,0.3)]",
  };

  const sizeStyles = {
    sm: "px-2 py-0.5 text-[10px]",
    md: "px-2.5 py-0.5 text-[11px]",
  };

  return (
    <div
      className={cn(
        "inline-flex items-center gap-1 rounded-full font-bold uppercase tracking-wide select-none transition-colors",
        variantStyles[variant],
        sizeStyles[size],
        className
      )}
      {...props}
    />
  );
}
