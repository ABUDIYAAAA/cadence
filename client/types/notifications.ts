export interface SubscriptionKeys {
  p256dh: string;
  auth: string;
}

export interface SubscribeRequest {
  endpoint: string;
  keys: SubscriptionKeys;
}

export interface UnsubscribeRequest {
  endpoint: string;
}

export interface BroadcastNotificationRequest {
  title: string;
  body: string;
  action_url?: string;
  icon_url?: string;
  target_type: "broadcast" | "team" | "role" | "user";
  target_id?: string;
}

export interface BroadcastNotificationResponse {
  notification_id: string;
  recipients_count: number;
  enqueued_pushes: number;
  message: string;
}

export interface NotificationItem {
  id: string;
  title: string;
  body: string;
  action_url?: string | null;
  icon_url?: string | null;
  is_read: boolean;
  read_at?: string | null;
  created_at: string;
}

export interface InboxResponse {
  notifications: NotificationItem[];
  unread_count: number;
  total_count: number;
  page: number;
  limit: number;
}

export interface AdminNotificationHistoryItem {
  id: string;
  title: string;
  body: string;
  action_url?: string | null;
  target_type: string;
  target_id?: string | null;
  created_by_email?: string | null;
  recipients_count: number;
  read_count: number;
  created_at: string;
}

export interface AdminHistoryResponse {
  items: AdminNotificationHistoryItem[];
  total_count: number;
  page: number;
  limit: number;
}
