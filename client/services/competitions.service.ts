import { apiClient } from "../lib/api-client.ts";
import type { MessageResponse } from "../types/auth.ts";
import type {
  Competition,
  CompetitionListResponse,
  CompetitionResponse,
  CreateCompetitionRequest,
  UpdateCompetitionRequest,
} from "../types/competitions.ts";

export interface CompetitionsHttpClient {
  get<T>(url: string): Promise<{ data: T }>;
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
  };
}

export const competitionsService = createCompetitionsService(apiClient);
