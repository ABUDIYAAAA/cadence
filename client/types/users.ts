import { UserRole } from "./auth";
import { PaginatedResponse } from "./api";

export interface UserDetailResponse {
  id: string;
  email: string;
  name: string;
  phone_number?: string | null;
  role: UserRole;
  team_id?: string | null;
  team_name?: string | null;
  is_leader: boolean;
  is_password_configured: boolean;
  is_active: boolean;
  created_at: string;
  /** Present on the list endpoint only. */
  checkin_status?: CheckinStatus;
  checked_in_at?: string | null;
}

export type CheckinStatus = "checked_in" | "not_checked_in" | "no_pass";

export interface CreateUserRequest {
  email: string;
  name: string;
  role?: UserRole;
  phone_number?: string;
}

export interface BulkCreateUsersRequest {
  users: CreateUserRequest[];
}

export interface BulkCreateResponse {
  total_processed: number;
  total_created: number;
  skipped_count: number;
  skipped_errors?: string[];
  users: UserDetailResponse[];
}

export interface UpdateUserRequest {
  name?: string;
  phone_number?: string;
  role?: UserRole;
  is_active?: boolean;
}

export interface UserFilter {
  page?: number;
  limit?: number;
  role?: string;
  team_id?: string;
  /** Only users who are not on any team. */
  no_team?: boolean;
  search?: string;
  is_active?: boolean;
  checkin_status?: CheckinStatus;
}

export type PaginatedUsersResponse = PaginatedResponse<UserDetailResponse>;
