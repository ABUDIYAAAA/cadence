"use client";

import React, { useEffect, useState, useRef } from "react";
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
  const [rendered, setRendered] = useState(open);
  const [visible, setVisible] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousActiveElement = useRef<HTMLElement | null>(null);

  // Derive rendered state when open changes to true
  if (open && !rendered) {
    setRendered(true);
  }

  useEffect(() => {
    if (open) {
      previousActiveElement.current = document.activeElement as HTMLElement;
      const timer = setTimeout(() => {
        setVisible(true);
      }, 10);
      return () => clearTimeout(timer);
    } else {
      const frame = requestAnimationFrame(() => {
        setVisible(false);
      });
      const timer = setTimeout(() => {
        setRendered(false);
        previousActiveElement.current?.focus();
      }, 150);
      return () => {
        cancelAnimationFrame(frame);
        clearTimeout(timer);
      };
    }
  }, [open]);

  useEffect(() => {
    if (!rendered || !visible) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }

      if (e.key === "Tab" && dialogRef.current) {
        const focusableElements = dialogRef.current.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        if (focusableElements.length === 0) return;

        const first = focusableElements[0];
        const last = focusableElements[focusableElements.length - 1];

        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [rendered, visible, onClose]);

  // Focus first interactive element on appearance
  useEffect(() => {
    if (rendered && visible && dialogRef.current) {
      const focusable = dialogRef.current.querySelector<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      focusable?.focus();
    }
  }, [rendered, visible]);

  if (!rendered) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop with frosted blur & smooth fade exit */}
      <div
        className={cn(
          "fixed inset-0 bg-black/40 backdrop-blur-md transition-opacity duration-150 ease-out",
          visible ? "opacity-100" : "opacity-0"
        )}
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Modal Dialog Card with smooth scale & fade enter/exit */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialog-title"
        aria-describedby={description ? "dialog-description" : undefined}
        ref={dialogRef}
        className={cn(
          "relative z-50 flex w-full max-w-lg max-h-[calc(100dvh-2rem)] flex-col rounded-[24px] border border-[var(--card-border)] bg-[var(--card-solid)] p-6 sm:p-7 shadow-[var(--shadow-container)] text-[var(--ink-primary)] font-sans transition-all duration-150 ease-out transform",
          visible
            ? "opacity-100 scale-100 translate-y-0"
            : "opacity-0 scale-95 translate-y-1",
          className
        )}
      >
        <div className="flex shrink-0 items-start justify-between pb-4 border-b border-[var(--card-line)]">
          <div>
            <h2
              id="dialog-title"
              className="text-base sm:text-lg font-extrabold tracking-tight text-[var(--ink-primary)]"
            >
              {title}
            </h2>
            {description && (
              <p
                id="dialog-description"
                className="mt-0.5 text-xs text-[var(--ink-secondary)] leading-relaxed"
              >
                {description}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="relative h-8 w-8 rounded-full bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] flex items-center justify-center transition-transform active:scale-90 cursor-pointer before:absolute before:-inset-2 before:content-['']"
            aria-label="Close dialog"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* The body scrolls when content is taller than the screen; header and footer stay put */}
        <div className="-mx-1 min-h-0 flex-1 overflow-y-auto overscroll-contain px-1 py-4 text-xs">{children}</div>

        {footer && (
          <div className="flex shrink-0 items-center justify-end gap-2.5 pt-4 border-t border-[var(--card-line)]">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
