import React from "react";
import { cn } from "@/lib/utils";

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
  const variantPrefix = {
    info: "[INFO]",
    warning: "[WARNING]",
    error: "[ERROR]",
    success: "[OK]",
  };

  return (
    <div
      role="alert"
      className={cn(
        "border border-black bg-neutral-50 p-4 font-mono text-sm text-black",
        className
      )}
      {...props}
    >
      <div className="flex items-start gap-2">
        <span className="font-bold text-xs uppercase tracking-wider text-black">
          {variantPrefix[variant]}
        </span>
        <div className="space-y-1">
          {title && <h5 className="font-bold tracking-tight">{title}</h5>}
          <div className="text-xs text-neutral-800 leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  );
}
