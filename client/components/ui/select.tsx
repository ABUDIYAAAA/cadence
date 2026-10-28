import React from "react";
import { cn } from "@/lib/utils";

export interface SelectOption {
  label: string;
  value: string;
}

export interface SelectProps
  extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  error?: string;
  helperText?: string;
  options?: SelectOption[];
}

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  (
    { className, label, error, helperText, id, options, children, ...props },
    ref
  ) => {
    const selectId =
      id || (label ? label.toLowerCase().replace(/\s+/g, "-") : undefined);

    return (
      <div className="w-full space-y-1.5 text-left font-sans">
        {label && (
          <label
            htmlFor={selectId}
            className="block text-xs font-semibold text-[var(--ink-primary)]"
          >
            {label}
          </label>
        )}
        <div className="relative">
          <select
            id={selectId}
            ref={ref}
            className={cn(
              "flex h-[42px] w-full rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3.5 pr-8 text-xs text-[var(--ink-primary)] transition-all appearance-none focus:border-[var(--color-blue)] focus:outline-none focus:ring-3 focus:ring-[var(--color-blue)]/20 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer",
              error && "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20",
              className
            )}
            {...props}
          >
            {options
              ? options.map((opt) => (
                  <option key={opt.value} value={opt.value} className="bg-[var(--card-solid)] text-[var(--ink-primary)]">
                    {opt.label}
                  </option>
                ))
              : children}
          </select>
          <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-[var(--ink-secondary)]">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <path d="m6 9 6 6 6-6"/>
            </svg>
          </div>
        </div>
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

Select.displayName = "Select";
