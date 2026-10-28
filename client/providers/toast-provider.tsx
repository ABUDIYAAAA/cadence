"use client";

import React, { createContext, useContext, useState, useCallback, useRef } from "react";
import { cn } from "@/lib/utils";

export type ToastType = "info" | "success" | "error" | "warning";

export interface ToastItem {
  id: string;
  type: ToastType;
  title?: string;
  message: string;
  duration?: number;
  isExiting?: boolean;
}

interface ToastContextType {
  toasts: ToastItem[];
  toast: (options: {
    message: string;
    type?: ToastType;
    title?: string;
    duration?: number;
  }) => void;
  removeToast: (id: string) => void;
}

const ToastContext = createContext<ToastContextType | undefined>(undefined);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const timeoutsRef = useRef<Map<string, { timeoutId: NodeJS.Timeout; remaining: number; startedAt: number }>>(new Map());
  const isPausedRef = useRef(false);

  const removeToast = useCallback((id: string) => {
    // Clear timer
    const entry = timeoutsRef.current.get(id);
    if (entry) {
      clearTimeout(entry.timeoutId);
      timeoutsRef.current.delete(id);
    }

    // Trigger exit animation
    setToasts((prev) =>
      prev.map((t) => (t.id === id ? { ...t, isExiting: true } : t))
    );

    // Remove from DOM after exit animation completes (160ms)
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 160);
  }, []);

  const toast = useCallback(
    ({
      message,
      type = "info",
      title,
      duration = 4500,
    }: {
      message: string;
      type?: ToastType;
      title?: string;
      duration?: number;
    }) => {
      const id = Math.random().toString(36).substring(2, 9);
      const newToast: ToastItem = { id, type, title, message, duration, isExiting: false };

      setToasts((prev) => [...prev, newToast]);

      if (duration > 0) {
        const timeoutId = setTimeout(() => {
          removeToast(id);
        }, duration);

        timeoutsRef.current.set(id, {
          timeoutId,
          remaining: duration,
          startedAt: Date.now(),
        });
      }
    },
    [removeToast]
  );

  const handleMouseEnter = useCallback(() => {
    if (isPausedRef.current) return;
    isPausedRef.current = true;

    timeoutsRef.current.forEach((entry) => {
      clearTimeout(entry.timeoutId);
      const elapsed = Date.now() - entry.startedAt;
      entry.remaining = Math.max(0, entry.remaining - elapsed);
    });
  }, []);

  const handleMouseLeave = useCallback(() => {
    if (!isPausedRef.current) return;
    isPausedRef.current = false;

    timeoutsRef.current.forEach((entry, id) => {
      if (entry.remaining > 0) {
        entry.startedAt = Date.now();
        entry.timeoutId = setTimeout(() => {
          removeToast(id);
        }, entry.remaining);
      } else {
        removeToast(id);
      }
    });
  }, [removeToast]);

  return (
    <ToastContext.Provider value={{ toasts, toast, removeToast }}>
      {children}
      {/* Apple Floating Style Toast Container */}
      <div
        className="fixed bottom-6 right-6 z-50 flex flex-col gap-2.5 max-w-sm w-full pointer-events-none font-sans"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        {toasts.map((item) => (
          <div
            key={item.id}
            className={cn(
              "pointer-events-auto rounded-full bg-[#10141A] dark:bg-white text-white dark:text-[#10141A] px-4 py-2.5 shadow-[0_12px_28px_-6px_rgba(16,20,26,0.3)] dark:shadow-[0_12px_28px_-6px_rgba(255,255,255,0.2)] flex items-center justify-between gap-3 text-xs font-semibold transition-[transform,opacity] duration-160 ease-out transform",
              item.isExiting
                ? "opacity-0 translate-y-2 scale-95"
                : "opacity-100 translate-y-0 scale-100 animate-in slide-in-from-bottom-2 duration-160"
            )}
          >
            <div className="flex items-center gap-2 min-w-0">
              <span
                className={cn(
                  "h-2 w-2 rounded-full shrink-0",
                  item.type === "error" || item.type === "warning"
                    ? "bg-[#CE6969]"
                    : item.type === "success"
                    ? "bg-[#2EB88A]"
                    : "bg-[#83A2DB]"
                )}
              />
              <div className="truncate">
                {item.title && (
                  <span className="font-bold mr-1.5 uppercase tracking-wide opacity-80 text-[11px]">
                    {item.title}:
                  </span>
                )}
                <span>{item.message}</span>
              </div>
            </div>

            <button
              onClick={() => removeToast(item.id)}
              className="relative text-white/60 dark:text-[#10141A]/60 hover:text-white dark:hover:text-[#10141A] text-xs p-1 cursor-pointer transition-colors before:absolute before:-inset-2 before:content-['']"
              aria-label="Dismiss toast"
            >
              ✕
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used within a ToastProvider");
  }

  return {
    toast: context.toast,
    info: (message: string, title?: string) =>
      context.toast({ message, title, type: "info" }),
    success: (message: string, title?: string) =>
      context.toast({ message, title, type: "success" }),
    error: (message: string, title?: string) =>
      context.toast({ message, title, type: "error" }),
    warning: (message: string, title?: string) =>
      context.toast({ message, title, type: "warning" }),
  };
}
