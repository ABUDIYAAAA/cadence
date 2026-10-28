import { apiClient } from "@/lib/api-client";
import {
  AddMemberRequest,
  CreateTeamRequest,
  PaginatedTeamsResponse,
  TeamDetailResponse,
  TeamFilter,
  UpdateTeamRequest,
} from "@/types/teams";
import { MessageResponse } from "@/types/auth";

export const teamsService = {
  async getMyTeam(): Promise<TeamDetailResponse> {
    const { data } = await apiClient.get<TeamDetailResponse>("/teams/my-team");
    return data;
  },

  async list(filters?: TeamFilter): Promise<PaginatedTeamsResponse> {
    const { data } = await apiClient.get<PaginatedTeamsResponse>("/teams", {
      params: filters,
    });
    return data;
  },

  async getById(id: string): Promise<TeamDetailResponse> {
    const { data } = await apiClient.get<TeamDetailResponse>(`/teams/${id}`);
    return data;
  },

  async create(payload: CreateTeamRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClient.post<TeamDetailResponse>("/teams", payload);
    return data;
  },

  async update(id: string, payload: UpdateTeamRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClient.patch<TeamDetailResponse>(`/teams/${id}`, payload);
    return data;
  },

  async delete(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(`/teams/${id}`);
    return data;
  },

  async addMember(teamId: string, payload: AddMemberRequest): Promise<TeamDetailResponse> {
    const { data } = await apiClient.post<TeamDetailResponse>(
      `/teams/${teamId}/members`,
      payload
    );
    return data;
  },

  async removeMember(teamId: string, userId: string): Promise<TeamDetailResponse> {
    const { data } = await apiClient.delete<TeamDetailResponse>(
      `/teams/${teamId}/members/${userId}`
    );
    return data;
  },
};
