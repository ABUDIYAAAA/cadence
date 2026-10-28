"use client";

import React, { createContext, useContext, useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "./auth-provider";
import { useToast } from "./toast-provider";
import {
  isPushSupported,
  syncPushSubscription,
  unsubscribePush,
} from "@/lib/push-notifications";
import { notificationsService } from "@/services/notifications.service";
import { NOTIFICATIONS_QUERY_KEY } from "@/hooks/use-notifications";
import { Bell, X, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface LiveNotificationPayload {
  title: string;
  body: string;
  url?: string;
  notification_id?: string;
}

interface NotificationContextType {
  isSupported: boolean;
  permission: NotificationPermission | "unsupported";
  isSubscribed: boolean;
  isLoading: boolean;
  liveNotification: LiveNotificationPayload | null;
  enablePush: () => Promise<boolean>;
  disablePush: () => Promise<boolean>;
  dismissLiveNotification: () => void;
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
  const queryClient = useQueryClient();

  const [isSupported, setIsSupported] = useState(false);
  const [permission, setPermission] = useState<
    NotificationPermission | "unsupported"
  >("unsupported");
  const [isSubscribed, setIsSubscribed] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [showPromptBanner, setShowPromptBanner] = useState(false);
  const [liveNotification, setLiveNotification] =
    useState<LiveNotificationPayload | null>(null);

  // Initial browser support check
  useEffect(() => {
    requestAnimationFrame(() => {
      const supported = isPushSupported();
      setIsSupported(supported);
      if (supported) {
        setPermission(Notification.permission);
      }
    });
  }, []);

  // Listen to Service Worker postMessage for real-time live push notifications
  useEffect(() => {
    if (typeof window === "undefined" || !("serviceWorker" in navigator)) return;

    const handleMessage = (event: MessageEvent) => {
      if (event.data?.type === "LIVE_PUSH_NOTIFICATION" && event.data.payload) {
        setLiveNotification(event.data.payload);
        queryClient.invalidateQueries({ queryKey: NOTIFICATIONS_QUERY_KEY });
      }
    };

    navigator.serviceWorker.addEventListener("message", handleMessage);
    return () => {
      navigator.serviceWorker.removeEventListener("message", handleMessage);
    };
  }, [queryClient]);

  // When authenticated, check status and sync
  useEffect(() => {
    if (!isAuthenticated || !user || !isSupported) {
      return;
    }

    requestAnimationFrame(() => {
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
    });
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
    } catch (err: unknown) {
      toast.error(
        (err as Error)?.message || "Failed to register notifications.",
        "Subscription Error"
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

  const handleDismissPromptBanner = () => {
    setShowPromptBanner(false);
    sessionStorage.setItem("cadence_push_prompt_dismissed", "true");
  };

  const handleDismissLiveNotification = () => {
    if (liveNotification?.notification_id) {
      notificationsService
        .markAsRead(liveNotification.notification_id)
        .catch(() => {});
      queryClient.invalidateQueries({ queryKey: NOTIFICATIONS_QUERY_KEY });
    }
    setLiveNotification(null);
  };

  return (
    <NotificationContext.Provider
      value={{
        isSupported,
        permission,
        isSubscribed,
        isLoading,
        liveNotification,
        enablePush: handleEnablePush,
        disablePush: handleDisablePush,
        dismissLiveNotification: handleDismissLiveNotification,
      }}
    >
      {/* 1. Live Incoming Notification Banner (Black background with white text) */}
      {liveNotification && isAuthenticated && (
        <div className="border-b-2 border-black bg-black text-white font-mono px-4 py-2.5 animate-in slide-in-from-top-2 z-50">
          <div className="mx-auto max-w-7xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div className="flex items-center gap-2.5 min-w-0">
              <span className="flex items-center justify-center h-6 w-6 border border-white bg-white text-black shrink-0 font-bold text-xs">
                !
              </span>
              <div className="text-xs min-w-0">
                <strong className="uppercase mr-1.5 font-bold">
                  {liveNotification.title}:
                </strong>
                <span className="text-neutral-300">{liveNotification.body}</span>
              </div>
            </div>

            <div className="flex items-center gap-2 self-end sm:self-auto shrink-0">
              {Boolean(
                liveNotification.url &&
                  liveNotification.url.trim() &&
                  liveNotification.url !== "/"
              ) && (
                <Link
                  href={liveNotification.url!}
                  onClick={handleDismissLiveNotification}
                  className="h-7 text-xs px-3 bg-white text-black font-bold uppercase inline-flex items-center gap-1 hover:bg-neutral-200 transition-colors"
                >
                  <span>Open</span>
                  <ExternalLink className="h-3 w-3" />
                </Link>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={handleDismissLiveNotification}
                className="h-7 w-7 p-0 text-white hover:bg-neutral-800 cursor-pointer"
                aria-label="Dismiss alert"
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* 2. Push Permission Prompt Banner (Neutral background with black text) */}
      {!liveNotification && showPromptBanner && isAuthenticated && (
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
                onClick={handleDismissPromptBanner}
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
