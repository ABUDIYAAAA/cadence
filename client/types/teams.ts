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
  /** Hidden from judges. */
  join_code?: string;
  leader_id?: string | null;
  leader_name?: string | null;
  leader_email?: string | null;
  member_count: number;
  /** This team's own freeze flag. */
  is_frozen: boolean;
  /** True when this team or all teams are frozen. */
  effective_frozen: boolean;
  /** Per-team size override, if any. */
  max_size?: number | null;
  effective_max_size: number;
  created_at: string;
  updated_at: string;
}

export interface TeamDetailResponse {
  id: string;
  name: string;
  join_code?: string;
  leader?: TeamMemberSummary | null;
  members: TeamMemberSummary[];
  is_frozen: boolean;
  effective_frozen: boolean;
  max_size?: number | null;
  effective_max_size: number;
  created_at: string;
  updated_at: string;
}

export interface CreateTeamRequest {
  name: string;
}

export interface JoinTeamRequest {
  code: string;
}

export interface RenameTeamRequest {
  name: string;
}

/** Staff edits. `max_size: 0` clears the per-team override. */
export interface UpdateTeamRequest {
  name?: string;
  leader_id?: string;
  max_size?: number;
}

export interface AddMemberRequest {
  user_id: string;
}

export interface TeamSettings {
  frozen_all: boolean;
  max_team_size: number;
  max_team_size_source: "database" | "environment";
}

export interface UpdateTeamSettingsRequest {
  max_team_size?: number;
}

export interface TeamFilter {
  page?: number;
  limit?: number;
  search?: string;
}

export type PaginatedTeamsResponse = PaginatedResponse<TeamResponse>;
