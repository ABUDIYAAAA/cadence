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
      <div className="w-full space-y-1 text-left font-mono">
        {label && (
          <label
            htmlFor={textareaId}
            className="block text-xs uppercase tracking-wider text-neutral-700"
          >
            {label}
          </label>
        )}
        <textarea
          id={textareaId}
          ref={ref}
          className={cn(
            "flex min-h-[90px] w-full border border-neutral-300 bg-white p-3 text-sm text-black placeholder:text-neutral-400 transition-colors focus:border-black focus:outline-none focus:ring-1 focus:ring-black disabled:cursor-not-allowed disabled:bg-neutral-100 disabled:opacity-50",
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

Textarea.displayName = "Textarea";
