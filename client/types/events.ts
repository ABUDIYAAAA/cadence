import { AssignableEventRole, EventRole, UserRole } from "./auth";
import { CheckinStatus } from "./users";

export interface Event {
  id: string;
  slug: string;
  name: string;
  description?: string | null;
  logo_url?: string | null;
  member_count?: number;
  created_at: string;
  updated_at: string;
}

export interface EventResponse extends Event {
  my_role?: EventRole | null;
}

export interface CreateEventRequest {
  slug: string;
  name: string;
  description?: string;
  logo_url?: string;
  owner_email?: string;
  owner_id?: string;
  owner_name?: string;
}

export interface UpdateEventRequest {
  name?: string;
  description?: string;
  logo_url?: string;
}

export interface TransferOwnershipRequest {
  new_owner_user_id?: string;
  new_owner_email?: string;
  new_owner_name?: string;
}

export interface UpdateMemberRoleRequest {
  role: AssignableEventRole;
}

export interface AddEventUserRequest {
  email: string;
  name?: string;
  role?: AssignableEventRole;
  phone_number?: string;
}

export interface BulkAddEventUsersRequest {
  users: AddEventUserRequest[];
}

export interface EventUserDetailResponse {
  id: string;
  email: string;
  name: string;
  phone_number?: string | null;
  avatar_url?: string | null;
  role: UserRole;
  is_banned: boolean;
  banned_at?: string | null;
  team_id?: string | null;
  team_name?: string | null;
  is_leader?: boolean | null;
  created_at: string;
  checkin_status: CheckinStatus;
  checked_in_at?: string | null;
}

export interface BulkAddEventUsersResponse {
  total_processed: number;
  total_created: number;
  total_added: number;
  skipped_count: number;
  skipped_errors?: string[];
  users: EventUserDetailResponse[];
}

export interface EventUserFilter {
  page?: number;
  limit?: number;
  role?: string;
  competition_id?: string;
  team_id?: string;
  no_team?: boolean;
  search?: string;
  is_banned?: boolean;
  checkin_status?: CheckinStatus;
}

export interface EventUserListResponse {
  users: EventUserDetailResponse[];
  total: number;
  page: number;
  limit: number;
}

export interface BanMemberRequest {
  reason?: string;
}
