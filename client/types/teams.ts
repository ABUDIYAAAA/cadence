import { PaginatedResponse } from "./api";
import { UserRole } from "./auth";

export interface TeamMemberSummary {
  id: string;
  email: string;
  name: string;
  phone_number?: string | null;
  role: UserRole;
  is_leader: boolean;
  is_password_configured: boolean;
}

export interface TeamResponse {
  id: string;
  name: string;
  leader_id?: string | null;
  leader_name?: string | null;
  leader_email?: string | null;
  member_count: number;
  created_at: string;
  updated_at: string;
}

export interface TeamDetailResponse {
  id: string;
  name: string;
  leader?: TeamMemberSummary | null;
  members: TeamMemberSummary[];
  created_at: string;
  updated_at: string;
}

export interface CreateTeamRequest {
  name: string;
  leader_id?: string;
  initial_member_ids?: string[];
}

export interface UpdateTeamRequest {
  name?: string;
  leader_id?: string;
}

export interface AddMemberRequest {
  user_id: string;
}

export interface TeamFilter {
  page?: number;
  limit?: number;
  search?: string;
}

export type PaginatedTeamsResponse = PaginatedResponse<TeamResponse>;
