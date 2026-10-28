import React from "react";
import { cn } from "@/lib/utils";

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = "text", label, error, helperText, id, ...props }, ref) => {
    const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, "-") : undefined);

    return (
      <div className="w-full space-y-1.5 text-left font-sans">
        {label && (
          <label
            htmlFor={inputId}
            className="block text-xs font-semibold text-[var(--ink-primary)]"
          >
            {label}
          </label>
        )}
        <input
          id={inputId}
          type={type}
          ref={ref}
          className={cn(
            "flex h-[42px] w-full rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3.5 text-xs text-[var(--ink-primary)] placeholder:text-[var(--ink-tertiary)] transition-all focus:border-[var(--color-blue)] focus:outline-none focus:ring-3 focus:ring-[var(--color-blue)]/20 disabled:cursor-not-allowed disabled:opacity-50",
            error && "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20",
            className
          )}
          {...props}
        />
        {error && (
          <p className="text-xs text-[var(--color-coral)] font-medium pt-0.5">{error}</p>
        )}
        {helperText && !error && (
          <p className="text-xs text-[var(--ink-secondary)]">{helperText}</p>
        )}
      </div>
    );
  }
);

Input.displayName = "Input";
