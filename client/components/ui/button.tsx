import React from "react";
import { cn } from "@/lib/utils";
import { Spinner } from "./spinner";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "outline" | "ghost" | "destructive" | "blue";
  size?: "sm" | "md" | "lg" | "icon";
  isLoading?: boolean;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = "primary",
      size = "md",
      isLoading = false,
      disabled,
      children,
      ...props
    },
    ref
  ) => {
    const baseStyles =
      "inline-flex items-center justify-center font-sans font-semibold tracking-tight select-none cursor-pointer outline-none transition-[transform,opacity,background-color,border-color,box-shadow] duration-[130ms] ease-[cubic-bezier(0.25,1,0.35,1)] active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50";

    const variantStyles = {
      primary:
        "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-[var(--shadow-dark)] hover:opacity-90 border border-transparent",
      secondary:
        "bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] border border-[var(--card-line)] hover:bg-[var(--card-solid)] hover:text-[var(--ink-primary)]",
      outline:
        "bg-[var(--card-solid)] text-[var(--ink-primary)] border border-[var(--card-line)] shadow-[var(--shadow-card)] hover:bg-[var(--card-solid-subtle)]",
      ghost:
        "bg-transparent text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] border border-transparent",
      destructive:
        "bg-[var(--color-coral)] text-white shadow-[0_4px_14px_rgba(206,105,105,0.35)] hover:opacity-90 border border-transparent",
      blue:
        "bg-[var(--color-blue)] text-white shadow-[0_4px_14px_rgba(131,162,219,0.35)] hover:opacity-90 border border-transparent",
    };

    const sizeStyles = {
      sm: "relative h-8 px-3 text-xs gap-1.5 rounded-[10px] before:absolute before:-inset-1.5 before:content-['']",
      md: "h-[38px] px-4 text-xs gap-2 rounded-[14px]",
      lg: "h-11 px-6 text-sm gap-2.5 rounded-[14px]",
      icon: "relative h-[38px] w-[38px] p-0 rounded-full before:absolute before:-inset-1 before:content-['']",
    };

    return (
      <button
        ref={ref}
        disabled={disabled || isLoading}
        className={cn(
          baseStyles,
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
        {...props}
      >
        {isLoading && <Spinner size="sm" className="mr-1.5" />}
        {children}
      </button>
    );
  }
);

Button.displayName = "Button";
