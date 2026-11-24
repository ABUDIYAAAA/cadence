import { apiClient } from "@/lib/api-client";
import { MessageResponse } from "@/types/auth";
import {
  Competition,
  CompetitionListResponse,
  CreateCompetitionRequest,
  UpdateCompetitionRequest,
} from "@/types/competitions";

export const competitionsService = {
  async listByEvent(eventSlug: string): Promise<Competition[]> {
    const { data } = await apiClient.get<CompetitionListResponse>(
      `/e/${encodeURIComponent(eventSlug)}/competitions`
    );
    return Array.isArray(data?.competitions) ? data.competitions : [];
  },

  async createCompetition(
    eventSlug: string,
    payload: CreateCompetitionRequest
  ): Promise<Competition> {
    const { data } = await apiClient.post<Competition>(
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
    const { data } = await apiClient.patch<Competition>(
      `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}`,
      payload
    );
    return data;
  },

  async deleteCompetition(
    eventSlug: string,
    competitionSlug: string
  ): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(
      `/e/${encodeURIComponent(eventSlug)}/c/${encodeURIComponent(competitionSlug)}`
    );
    return data;
  },
};
