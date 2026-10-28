"use client";

import React, { useEffect } from "react";
import { cn } from "@/lib/utils";
import { X } from "lucide-react";

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
      {/* Backdrop with subtle frosted blur */}
      <div
        className="fixed inset-0 bg-black/40 backdrop-blur-md transition-opacity duration-200"
        onClick={onClose}
      />

      {/* Modal Dialog Card */}
      <div
        className={cn(
          "relative z-50 w-full max-w-lg rounded-[24px] border border-[var(--card-border)] bg-[var(--card-solid)] p-6 sm:p-7 shadow-[var(--shadow-container)] text-[var(--ink-primary)] font-sans animate-in fade-in-0 zoom-in-95 duration-150",
          className
        )}
      >
        <div className="flex items-start justify-between pb-4 border-b border-[var(--card-line)]">
          <div>
            <h2 className="text-base sm:text-lg font-extrabold tracking-tight text-[var(--ink-primary)]">
              {title}
            </h2>
            {description && (
              <p className="mt-0.5 text-xs text-[var(--ink-secondary)] leading-relaxed">
                {description}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="h-8 w-8 rounded-full bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] flex items-center justify-center transition-all active:scale-90 cursor-pointer"
            aria-label="Close dialog"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="py-4 text-xs">{children}</div>

        {footer && (
          <div className="flex items-center justify-end gap-2.5 pt-4 border-t border-[var(--card-line)]">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
