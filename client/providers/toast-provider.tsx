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
      {/* Toast Container */}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none font-mono">
        {toasts.map((item) => (
          <div
            key={item.id}
            className={cn(
              "pointer-events-auto border-2 border-black bg-white p-3.5 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] transition-all animate-in slide-in-from-bottom-2 text-black",
              item.type === "error" && "border-black bg-neutral-50",
              item.type === "success" && "border-black bg-white"
            )}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="space-y-0.5">
                <div className="flex items-center gap-2">
                  <span className="text-[10px] uppercase font-bold tracking-wider px-1.5 py-0.2 bg-black text-white">
                    {item.type === "error"
                      ? "ERR"
                      : item.type === "success"
                      ? "OK"
                      : item.type === "warning"
                      ? "WARN"
                      : "INFO"}
                  </span>
                  {item.title && (
                    <h5 className="text-xs font-bold uppercase tracking-wider">
                      {item.title}
                    </h5>
                  )}
                </div>
                <p className="text-xs text-neutral-800 leading-snug pt-1">
                  {item.message}
                </p>
              </div>

              <button
                onClick={() => removeToast(item.id)}
                className="text-neutral-500 hover:text-black text-sm leading-none p-1 transition-colors cursor-pointer"
                aria-label="Dismiss toast"
              >
                ✕
              </button>
            </div>
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
