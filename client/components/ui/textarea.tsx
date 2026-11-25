import React from "react";
import { cn } from "@/lib/utils";

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, label, error, helperText, id, ...props }, ref) => {
    const textareaId =
      id || (label ? label.toLowerCase().replace(/\s+/g, "-") : undefined);

    return (
      <div className="w-full space-y-1.5 text-left font-sans">
        {(label || error) && (
          <div className="flex items-baseline justify-between gap-2">
            {label && (
              <label
                htmlFor={textareaId}
                className="block text-xs font-semibold text-[var(--ink-primary)]"
              >
                {label}
              </label>
            )}
            {error && (
              <span
                role="alert"
                className="ml-auto text-right text-[11px] font-medium text-[var(--color-coral)]"
              >
                {error}
              </span>
            )}
          </div>
        )}
        <textarea
          id={textareaId}
          ref={ref}
          aria-invalid={Boolean(error) || undefined}
          className={cn(
            "flex min-h-[90px] w-full rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5 text-xs text-[var(--ink-primary)] placeholder:text-[var(--ink-tertiary)] transition-[border-color,box-shadow,background-color] duration-150 ease-out focus:border-[var(--color-blue)] focus:outline-none focus:ring-3 focus:ring-[var(--color-blue)]/20 disabled:cursor-not-allowed disabled:opacity-50",
            error &&
              "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20 focus:border-[var(--color-coral)] focus:ring-[var(--color-coral)]/25",
            className
          )}
          {...props}
        />
        {helperText && (
          <p className="text-xs text-[var(--ink-secondary)]">{helperText}</p>
        )}
      </div>
    );
  }
);

Textarea.displayName = "Textarea";
