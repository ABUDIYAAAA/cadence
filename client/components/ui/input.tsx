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
      <div className="w-full space-y-1 text-left font-mono">
        {label && (
          <label
            htmlFor={inputId}
            className="block text-xs uppercase tracking-wider text-neutral-700"
          >
            {label}
          </label>
        )}
        <input
          id={inputId}
          type={type}
          ref={ref}
          className={cn(
            "flex h-9 w-full border border-neutral-300 bg-white px-3 py-1 text-sm text-black placeholder:text-neutral-400 transition-colors focus:border-black focus:outline-none focus:ring-1 focus:ring-black disabled:cursor-not-allowed disabled:bg-neutral-100 disabled:opacity-50",
            error && "border-black ring-1 ring-black",
            className
          )}
          {...props}
        />
        {error && (
          <p className="text-xs text-neutral-900 font-medium">! {error}</p>
        )}
        {helperText && !error && (
          <p className="text-xs text-neutral-500">{helperText}</p>
        )}
      </div>
    );
  }
);

Input.displayName = "Input";
