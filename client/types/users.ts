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
}

export interface CreateUserRequest {
  email: string;
  name: string;
  role?: UserRole;
  phone_number?: string;
  team_name?: string;
  team_id?: string;
  is_leader?: boolean;
  send_invite?: boolean;
}

export interface BulkCreateUsersRequest {
  users: CreateUserRequest[];
}

export interface BulkCreateResponse {
  total_processed: number;
  total_created: number;
  skipped_count: number;
  skipped_errors?: string[];
  invites_dispatched: number;
  users: UserDetailResponse[];
}

export interface UpdateUserRequest {
  name?: string;
  phone_number?: string;
  role?: UserRole;
  team_id?: string | null;
  is_active?: boolean;
}

export interface UserFilter {
  page?: number;
  limit?: number;
  role?: string;
  team_id?: string;
  search?: string;
  is_active?: boolean;
}

export interface BulkInviteRequest {
  user_ids?: string[];
  all_uninvited?: boolean;
}

export type PaginatedUsersResponse = PaginatedResponse<UserDetailResponse>;
