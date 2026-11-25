import { apiClient } from "../lib/api-client.ts";
import {
  normalizeEventUser,
  normalizeEventUsersPage,
} from "../lib/tenant-scope.ts";
import type { MessageResponse } from "../types/auth.ts";
import type {
  Competition,
  CompetitionListResponse,
  CompetitionResponse,
  CreateCompetitionRequest,
  UpdateCompetitionRequest,
} from "../types/competitions.ts";
import type {
  AddEventUserRequest,
  BanMemberRequest,
  BulkAddEventUsersRequest,
  BulkAddEventUsersResponse,
  EventUserDetailResponse,
  EventUserFilter,
  EventUserListResponse,
  UpdateMemberRoleRequest,
} from "../types/events.ts";
import type {
  BulkCreateResponse,
  PaginatedUsersResponse,
  UserDetailResponse,
} from "../types/users.ts";

export interface CompetitionsHttpClient {
  get<T>(url: string, config?: { params?: unknown }): Promise<{ data: T }>;
  post<T>(url: string, payload?: unknown): Promise<{ data: T }>;
  patch<T>(url: string, payload?: unknown): Promise<{ data: T }>;
  delete<T>(url: string): Promise<{ data: T }>;
}

export function createCompetitionsService(
  client: CompetitionsHttpClient = apiClient
) {
  return {
    async listByEvent(eventSlug: string): Promise<Competition[]> {
      const { data } = await client.get<CompetitionListResponse>(
        `/e/${encodeURIComponent(eventSlug)}/competitions`
      );
      return Array.isArray(data?.competitions) ? data.competitions : [];
    },

    async getCompetition(
      eventSlug: string,
      competitionSlug: string
    ): Promise<CompetitionResponse> {
      const { data } = await client.get<CompetitionResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}`
      );
      return data;
    },

    async createCompetition(
      eventSlug: string,
      payload: CreateCompetitionRequest
    ): Promise<Competition> {
      const { data } = await client.post<Competition>(
        `/e/${encodeURIComponent(eventSlug)}/competitions`,
        payload
      );
      return data;
    },

    async updateCompetition(
      eventSlug: string,
      competitionSlug: string,
      payload: UpdateCompetitionRequest
    ): Promise<Competition> {
      const { data } = await client.patch<Competition>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}`,
        payload
      );
      return data;
    },

    async deleteCompetition(
      eventSlug: string,
      competitionSlug: string
    ): Promise<MessageResponse> {
      const { data } = await client.delete<MessageResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}`
      );
      return data;
    },

    // Competition-scoped member management (/e/{eventSlug}/c/{competitionSlug}/...)
    async listCompetitionUsers(
      eventSlug: string,
      competitionSlug: string,
      filters?: EventUserFilter
    ): Promise<PaginatedUsersResponse> {
      const { data } = await client.get<EventUserListResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/users`,
        { params: filters }
      );
      return normalizeEventUsersPage(data);
    },

    async addCompetitionUser(
      eventSlug: string,
      competitionSlug: string,
      payload: AddEventUserRequest
    ): Promise<UserDetailResponse> {
      const { data } = await client.post<EventUserDetailResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/users`,
        payload
      );
      return normalizeEventUser(data);
    },

    async bulkAddCompetitionUsers(
      eventSlug: string,
      competitionSlug: string,
      payload: BulkAddEventUsersRequest
    ): Promise<BulkCreateResponse> {
      const { data } = await client.post<BulkAddEventUsersResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/users/bulk`,
        payload
      );
      return {
        total_processed: data.total_processed,
        total_created: (data.total_created ?? 0) + (data.total_added ?? 0),
        total_added: data.total_added,
        teams_created: data.teams_created,
        teams_assigned: data.teams_assigned,
        skipped_count: data.skipped_count,
        skipped_errors: data.skipped_errors,
        users: (data.users ?? []).map(normalizeEventUser),
      };
    },

    async updateCompetitionMemberRole(
      eventSlug: string,
      competitionSlug: string,
      userId: string,
      payload: UpdateMemberRoleRequest
    ): Promise<MessageResponse> {
      const { data } = await client.patch<MessageResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/members/${encodeURIComponent(userId)}`,
        payload
      );
      return data;
    },

    async removeCompetitionMember(
      eventSlug: string,
      competitionSlug: string,
      userId: string
    ): Promise<MessageResponse> {
      const { data } = await client.delete<MessageResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/members/${encodeURIComponent(userId)}`
      );
      return data;
    },

    async banCompetitionMember(
      eventSlug: string,
      competitionSlug: string,
      userId: string,
      payload?: BanMemberRequest
    ): Promise<MessageResponse> {
      const { data } = await client.post<MessageResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/members/${encodeURIComponent(userId)}/ban`,
        payload ?? {}
      );
      return data;
    },

    async unbanCompetitionMember(
      eventSlug: string,
      competitionSlug: string,
      userId: string
    ): Promise<MessageResponse> {
      const { data } = await client.post<MessageResponse>(
        `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}/members/${encodeURIComponent(userId)}/unban`,
        {}
      );
      return data;
    },
  };
}

export const competitionsService = createCompetitionsService(apiClient);
