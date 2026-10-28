import { apiClient } from "@/lib/api-client";
import {
  AdminHistoryResponse,
  BroadcastNotificationRequest,
  BroadcastNotificationResponse,
  InboxResponse,
  SubscribeRequest,
  UnsubscribeRequest,
} from "@/types/notifications";
import { MessageResponse } from "@/types/auth";

export const notificationsService = {
  async getVAPIDPublicKey(): Promise<{ public_key: string }> {
    const { data } = await apiClient.get<{ public_key: string }>(
      "/notifications/vapid-public-key"
    );
    return data;
  },

  async subscribe(payload: SubscribeRequest): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      "/notifications/subscribe",
      payload
    );
    return data;
  },

  async unsubscribe(payload: UnsubscribeRequest): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      "/notifications/unsubscribe",
      payload
    );
    return data;
  },

  async getInbox(page = 1, limit = 20): Promise<InboxResponse> {
    const { data } = await apiClient.get<InboxResponse>("/notifications/inbox", {
      params: { page, limit },
    });
    return data;
  },

  async markAsRead(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.patch<MessageResponse>(
      `/notifications/${id}/read`
    );
    return data;
  },

  async markAllAsRead(): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      "/notifications/read-all"
    );
    return data;
  },

  async broadcast(
    payload: BroadcastNotificationRequest
  ): Promise<BroadcastNotificationResponse> {
    const { data } = await apiClient.post<BroadcastNotificationResponse>(
      "/notifications/broadcast",
      payload
    );
    return data;
  },

  async getAdminHistory(page = 1, limit = 20): Promise<AdminHistoryResponse> {
    const { data } = await apiClient.get<AdminHistoryResponse>(
      "/notifications/admin/history",
      {
        params: { page, limit },
      }
    );
    return data;
  },
};
