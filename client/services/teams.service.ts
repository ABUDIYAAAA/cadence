import { apiClientV2 } from "@/lib/api-client";
import {
  AddMemberRequest,
  CreateTeamRequest,
  JoinTeamRequest,
  PaginatedTeamsResponse,
  RenameTeamRequest,
  TeamDetailResponse,
  TeamFilter,
  TeamSettings,
  UpdateTeamRequest,
  UpdateTeamSettingsRequest,
} from "@/types/teams";
import { MessageResponse } from "@/types/auth";
import { ApiError } from "@/types/api";

export const teamsService = {
  // ---- participant flows ----
  /** Resolves to null when the caller has no team (the server answers 404). */
  async getMyTeam(): Promise<TeamDetailResponse | null> {
    try {
      const { data } = await apiClientV2.get<TeamDetailResponse>("/teams/my-team");
      return data;
    } catch (err) {
      if ((err as ApiError | undefined)?.status === 404) return null;
      throw err;
    }
  },

  async create(payload: CreateTeamRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.post<TeamDetailResponse>("/teams", payload);
    return data;
  },

  async join(payload: JoinTeamRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.post<TeamDetailResponse>("/teams/join", payload);
    return data;
  },

  async leave(): Promise<MessageResponse> {
    const { data } = await apiClientV2.post<MessageResponse>("/teams/leave");
    return data;
  },

  async renameMine(payload: RenameTeamRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.patch<TeamDetailResponse>("/teams/my-team", payload);
    return data;
  },

  async regenerateMyCode(): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.post<TeamDetailResponse>(
      "/teams/my-team/regenerate-code"
    );
    return data;
  },

  async kickMember(userId: string): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.delete<TeamDetailResponse>(
      `/teams/my-team/members/${userId}`
    );
    return data;
  },

  // ---- staff / read flows ----
  async list(filters?: TeamFilter): Promise<PaginatedTeamsResponse> {
    const { data } = await apiClientV2.get<PaginatedTeamsResponse>("/teams", {
      params: filters,
    });
    return data;
  },

  async getById(id: string): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.get<TeamDetailResponse>(`/teams/${id}`);
    return data;
  },

  async update(id: string, payload: UpdateTeamRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.patch<TeamDetailResponse>(`/teams/${id}`, payload);
    return data;
  },

  async delete(id: string): Promise<MessageResponse> {
    const { data } = await apiClientV2.delete<MessageResponse>(`/teams/${id}`);
    return data;
  },

  async addMember(teamId: string, payload: AddMemberRequest): Promise<MessageResponse> {
    const { data } = await apiClientV2.post<MessageResponse>(
      `/teams/${teamId}/members`,
      payload
    );
    return data;
  },

  async removeMember(teamId: string, userId: string): Promise<MessageResponse> {
    const { data } = await apiClientV2.delete<MessageResponse>(
      `/teams/${teamId}/members/${userId}`
    );
    return data;
  },

  async regenerateCode(id: string): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.post<TeamDetailResponse>(
      `/teams/${id}/regenerate-code`
    );
    return data;
  },

  async freeze(id: string): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.post<TeamDetailResponse>(`/teams/${id}/freeze`);
    return data;
  },

  async unfreeze(id: string): Promise<TeamDetailResponse> {
    const { data } = await apiClientV2.post<TeamDetailResponse>(`/teams/${id}/unfreeze`);
    return data;
  },

  async freezeAll(): Promise<TeamSettings> {
    const { data } = await apiClientV2.post<TeamSettings>("/teams/freeze-all");
    return data;
  },

  async unfreezeAll(): Promise<TeamSettings> {
    const { data } = await apiClientV2.post<TeamSettings>("/teams/unfreeze-all");
    return data;
  },

  async getSettings(): Promise<TeamSettings> {
    const { data } = await apiClientV2.get<TeamSettings>("/teams/settings");
    return data;
  },

  async updateSettings(payload: UpdateTeamSettingsRequest): Promise<TeamSettings> {
    const { data } = await apiClientV2.patch<TeamSettings>("/teams/settings", payload);
    return data;
  },
};
