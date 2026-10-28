"use client";

import React, { createContext, useContext, useState, useCallback } from "react";
import { cn } from "@/lib/utils";

export type ToastType = "info" | "success" | "error" | "warning";

export interface ToastItem {
  id: string;
  type: ToastType;
  title?: string;
  message: string;
  duration?: number;
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

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
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
      const newToast: ToastItem = { id, type, title, message, duration };

      setToasts((prev) => [...prev, newToast]);

      if (duration > 0) {
        setTimeout(() => {
          removeToast(id);
        }, duration);
      }
    },
    [removeToast]
  );

  return (
    <ToastContext.Provider value={{ toasts, toast, removeToast }}>
      {children}
      {/* Apple Floating Style Toast Container */}
      <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2.5 max-w-sm w-full pointer-events-none font-sans">
        {toasts.map((item) => (
          <div
            key={item.id}
            className={cn(
              "pointer-events-auto rounded-full bg-[#10141A] dark:bg-white text-white dark:text-[#10141A] px-4 py-2.5 shadow-[0_12px_28px_-6px_rgba(16,20,26,0.3)] dark:shadow-[0_12px_28px_-6px_rgba(255,255,255,0.2)] flex items-center justify-between gap-3 text-xs font-semibold animate-in slide-in-from-bottom-2 fade-in-0 duration-200"
            )}
          >
            <div className="flex items-center gap-2 min-w-0">
              <span
                className={cn(
                  "h-2 w-2 rounded-full shrink-0",
                  item.type === "error" || item.type === "warning"
                    ? "bg-[#CE6969]"
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
              className="text-white/60 dark:text-[#10141A]/60 hover:text-white dark:hover:text-[#10141A] text-xs p-1 cursor-pointer transition-colors"
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
