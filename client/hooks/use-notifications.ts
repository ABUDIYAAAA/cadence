"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { notificationsService } from "@/services/notifications.service";
import {
  BroadcastNotificationRequest,
  SubscribeRequest,
  UnsubscribeRequest,
} from "@/types/notifications";

export const NOTIFICATIONS_QUERY_KEY = ["notifications"] as const;

export function useNotificationInbox(page = 1, limit = 20) {
  return useQuery({
    queryKey: [...NOTIFICATIONS_QUERY_KEY, "inbox", page, limit],
    queryFn: () => notificationsService.getInbox(page, limit),
    refetchInterval: 1000 * 30, // poll for in-app unread alerts
  });
}

export function useVAPIDPublicKey() {
  return useQuery({
    queryKey: [...NOTIFICATIONS_QUERY_KEY, "vapid-key"],
    queryFn: () => notificationsService.getVAPIDPublicKey(),
    staleTime: Infinity,
  });
}

export function useMarkNotificationAsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => notificationsService.markAsRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NOTIFICATIONS_QUERY_KEY });
    },
  });
}

export function useMarkAllNotificationsAsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => notificationsService.markAllAsRead(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NOTIFICATIONS_QUERY_KEY });
    },
  });
}

export function useBroadcastNotification() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: BroadcastNotificationRequest) =>
      notificationsService.broadcast(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...NOTIFICATIONS_QUERY_KEY, "history"],
      });
    },
  });
}

export function useAdminNotificationHistory(page = 1, limit = 20) {
  return useQuery({
    queryKey: [...NOTIFICATIONS_QUERY_KEY, "history", page, limit],
    queryFn: () => notificationsService.getAdminHistory(page, limit),
  });
}

export function useSubscribePush() {
  return useMutation({
    mutationFn: (payload: SubscribeRequest) =>
      notificationsService.subscribe(payload),
  });
}

export function useUnsubscribePush() {
  return useMutation({
    mutationFn: (payload: UnsubscribeRequest) =>
      notificationsService.unsubscribe(payload),
  });
}
