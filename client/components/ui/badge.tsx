import React from "react";
import { cn } from "@/lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "solid" | "outline" | "subtle";
  size?: "sm" | "md";
}

export function Badge({
  className,
  variant = "outline",
  size = "md",
  ...props
}: BadgeProps) {
  const variantStyles = {
    solid: "bg-black text-white border border-black",
    outline: "bg-white text-black border border-black",
    subtle: "bg-neutral-100 text-neutral-800 border border-neutral-300",
  };

  const sizeStyles = {
    sm: "px-1.5 py-0.2 text-[10px]",
    md: "px-2.5 py-0.5 text-xs",
  };

  return (
    <div
      className={cn(
        "inline-flex items-center uppercase tracking-wider font-mono font-medium transition-colors select-none",
        variantStyles[variant],
        sizeStyles[size],
        className
      )}
      {...props}
    />
  );
}
