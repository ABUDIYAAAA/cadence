import { PaginatedResponse } from "./api";

export interface AuditLogFilter {
  page?: number;
  limit?: number;
  action?: string;
  actor_id?: string;
  target_type?: string;
  target_id?: string;
  search?: string;
  from_date?: string;
  to_date?: string;
}

export interface AuditLogResponse {
  id: string;
  actor_id?: string | null;
  actor_email?: string | null;
  actor_role?: string | null;
  action: string;
  target_type: string;
  target_id?: string | null;
  description: string;
  metadata: Record<string, unknown>;
  ip_address?: string | null;
  user_agent?: string | null;
  device_id?: string | null;
  created_at: string;
}

export type PaginatedAuditLogsResponse = PaginatedResponse<AuditLogResponse>;
