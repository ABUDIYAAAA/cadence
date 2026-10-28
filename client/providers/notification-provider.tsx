"use client";

import React, { createContext, useContext, useEffect, useState, useCallback } from "react";
import { useAuth } from "./auth-provider";
import { useToast } from "./toast-provider";
import {
  isPushSupported,
  syncPushSubscription,
  unsubscribePush,
} from "@/lib/push-notifications";
import { Bell, X } from "lucide-react";
import { Button } from "@/components/ui/button";

interface NotificationContextType {
  isSupported: boolean;
  permission: NotificationPermission | "unsupported";
  isSubscribed: boolean;
  isLoading: boolean;
  enablePush: () => Promise<boolean>;
  disablePush: () => Promise<boolean>;
}

const NotificationContext = createContext<NotificationContextType | undefined>(
  undefined
);

export function NotificationProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const { isAuthenticated, user } = useAuth();
  const toast = useToast();

  const [isSupported, setIsSupported] = useState(false);
  const [permission, setPermission] = useState<
    NotificationPermission | "unsupported"
  >("unsupported");
  const [isSubscribed, setIsSubscribed] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [showPromptBanner, setShowPromptBanner] = useState(false);

  // Initial browser support check
  useEffect(() => {
    const supported = isPushSupported();
    setIsSupported(supported);
    if (supported) {
      setPermission(Notification.permission);
    }
  }, []);

  // When authenticated, check status and sync
  useEffect(() => {
    if (!isAuthenticated || !user || !isSupported) {
      setShowPromptBanner(false);
      return;
    }

    const currentPermission = Notification.permission;
    setPermission(currentPermission);

    if (currentPermission === "granted") {
      // Sync subscription silently in background
      setIsLoading(true);
      syncPushSubscription()
        .then((result) => {
          setIsSubscribed(result.success);
        })
        .finally(() => {
          setIsLoading(false);
        });
    } else if (currentPermission === "default") {
      // Check if user dismissed prompt this session
      const dismissed = sessionStorage.getItem("cadence_push_prompt_dismissed");
      if (!dismissed) {
        setShowPromptBanner(true);
      }
    }
  }, [isAuthenticated, user, isSupported]);

  const handleEnablePush = useCallback(async (): Promise<boolean> => {
    setIsLoading(true);
    try {
      const result = await syncPushSubscription({ promptIfDefault: true });
      setPermission(result.status);

      if (result.success) {
        setIsSubscribed(true);
        setShowPromptBanner(false);
        toast.success(
          "Real-time notifications enabled for this device.",
          "PUSH REGISTERED"
        );
        return true;
      } else {
        if (result.status === "denied") {
          toast.warning(
            "Notifications were blocked in your browser settings.",
            "PERMISSION DENIED"
          );
        }
        return false;
      }
    } catch (err: any) {
      toast.error(
        err?.message || "Failed to register notifications.",
        "SUBSCRIPTION ERROR"
      );
      return false;
    } finally {
      setIsLoading(false);
    }
  }, [toast]);

  const handleDisablePush = useCallback(async (): Promise<boolean> => {
    setIsLoading(true);
    try {
      const success = await unsubscribePush();
      if (success) {
        setIsSubscribed(false);
        toast.info("Notifications paused for this browser session.", "UNSUBSCRIBED");
      }
      return success;
    } finally {
      setIsLoading(false);
    }
  }, [toast]);

  const handleDismissBanner = () => {
    setShowPromptBanner(false);
    sessionStorage.setItem("cadence_push_prompt_dismissed", "true");
  };

  return (
    <NotificationContext.Provider
      value={{
        isSupported,
        permission,
        isSubscribed,
        isLoading,
        enablePush: handleEnablePush,
        disablePush: handleDisablePush,
      }}
    >
      {/* Monochrome Push Permission Banner */}
      {showPromptBanner && isAuthenticated && (
        <div className="border-b-2 border-black bg-neutral-100 font-mono text-black px-4 py-2.5">
          <div className="mx-auto max-w-7xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div className="flex items-center gap-2.5">
              <span className="flex items-center justify-center h-6 w-6 border border-black bg-black text-white shrink-0">
                <Bell className="h-3.5 w-3.5" />
              </span>
              <p className="text-xs leading-tight">
                <strong>Enable push notifications</strong> to receive live queue position
                updates, judging room calls, and broadcast announcements.
              </p>
            </div>

            <div className="flex items-center gap-2 self-end sm:self-auto shrink-0">
              <Button
                variant="primary"
                size="sm"
                onClick={handleEnablePush}
                isLoading={isLoading}
                className="h-7 text-xs px-3"
              >
                Enable Notifications
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleDismissBanner}
                className="h-7 w-7 p-0"
                aria-label="Dismiss banner"
              >
                <X className="h-3.5 w-3.5 text-neutral-600 hover:text-black" />
              </Button>
            </div>
          </div>
        </div>
      )}

      {children}
    </NotificationContext.Provider>
  );
}

export function useNotificationsContext() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error(
      "useNotificationsContext must be used within a NotificationProvider"
    );
  }
  return context;
}
