"use client";

import React, { useEffect } from "react";
import { cn } from "@/lib/utils";
import { Button } from "./button";

interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  className?: string;
}

export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  className,
}: DialogProps) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && open) {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/60 backdrop-blur-xs transition-opacity"
        onClick={onClose}
      />

      {/* Modal Dialog */}
      <div className={cn("relative z-50 w-full max-w-lg border-2 border-black bg-white p-6 shadow-2xl font-mono", className)}>
        <div className="flex items-start justify-between pb-3 border-b border-neutral-300">
          <div>
            <h2 className="text-base font-bold uppercase tracking-wider text-black">
              {title}
            </h2>
            {description && (
              <p className="mt-1 text-xs text-neutral-600">{description}</p>
            )}
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="h-7 w-7 p-0 text-base leading-none"
            aria-label="Close"
          >
            ✕
          </Button>
        </div>

        <div className="py-4">{children}</div>

        {footer && (
          <div className="flex justify-end gap-2 pt-3 border-t border-neutral-200">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
