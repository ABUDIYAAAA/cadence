import React from "react";
import { cn } from "@/lib/utils";
import { Info, AlertTriangle, AlertCircle, CheckCircle2 } from "lucide-react";

interface AlertProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "info" | "warning" | "error" | "success";
  title?: string;
}

export function Alert({
  variant = "info",
  title,
  children,
  className,
  ...props
}: AlertProps) {
  const variantStyles = {
    info: "bg-[var(--color-blue-dim)] border-[var(--color-blue-border)] text-[var(--ink-primary)]",
    success: "bg-[var(--color-blue-dim)] border-[var(--color-blue-border)] text-[var(--ink-primary)]",
    warning: "bg-[var(--color-coral-dim)] border-[var(--color-coral-border)] text-[var(--color-coral)]",
    error: "bg-[var(--color-coral-dim)] border-[var(--color-coral-border)] text-[var(--color-coral)]",
  };

  const icons = {
    info: <Info className="h-4 w-4 text-[var(--color-blue)] shrink-0 mt-0.5" />,
    success: <CheckCircle2 className="h-4 w-4 text-[var(--color-blue)] shrink-0 mt-0.5" />,
    warning: <AlertTriangle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />,
    error: <AlertCircle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />,
  };

  return (
    <div
      role="alert"
      className={cn(
        "rounded-[16px] border p-4 font-sans text-xs shadow-sm flex items-start gap-3",
        variantStyles[variant],
        className
      )}
      {...props}
    >
      {icons[variant]}
      <div className="space-y-1 flex-1 min-w-0">
        {title && (
          <h5 className="font-extrabold text-xs tracking-tight uppercase">
            {title}
          </h5>
        )}
        <div className="text-xs leading-relaxed opacity-90">{children}</div>
      </div>
    </div>
  );
}
