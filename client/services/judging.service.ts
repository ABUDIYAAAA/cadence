import { apiClient } from "@/lib/api-client";
import {
  AdminLeaderboardResponse,
  ConfigureSlotsRequest,
  CreatePanelRequest,
  CreateRoundRequest,
  JudgePanelSummary,
  JudgeSlotDetails,
  RoundSummary,
  SubmitScoresRequest,
  TeamStatusResponse,
  UpdateSlotLinksRequest,
  VolunteerPanelSchedule,
} from "@/types/judging";
import { MessageResponse } from "@/types/auth";

export const judgingService = {
  // Participant
  async getMyTeamStatus(): Promise<TeamStatusResponse> {
    const { data } = await apiClient.get<TeamStatusResponse>("/judging/teams/my-status");
    return data;
  },

  async updateSlotLinks(
    slotId: string,
    payload: UpdateSlotLinksRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.patch<MessageResponse>(
      `/judging/slots/${slotId}/links`,
      payload
    );
    return data;
  },

  // Volunteer
  async getVolunteerSchedule(): Promise<VolunteerPanelSchedule[]> {
    const { data } = await apiClient.get<VolunteerPanelSchedule[]>(
      "/judging/volunteer/schedule"
    );
    return data;
  },

  // Judge
  async getJudgePanels(): Promise<JudgePanelSummary[]> {
    const { data } = await apiClient.get<JudgePanelSummary[]>("/judging/judge/panels");
    return data;
  },

  async getJudgeSlotDetails(slotId: string): Promise<JudgeSlotDetails> {
    const { data } = await apiClient.get<JudgeSlotDetails>(
      `/judging/slots/${slotId}`
    );
    return data;
  },

  async startSlot(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/start`
    );
    return data;
  },

  async submitScores(
    slotId: string,
    payload: SubmitScoresRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/scores`,
      payload
    );
    return data;
  },

  async lockScores(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/lock`
    );
    return data;
  },

  async completeSlot(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/complete`
    );
    return data;
  },

  // Admin
  async createRound(payload: CreateRoundRequest): Promise<RoundSummary> {
    const { data } = await apiClient.post<RoundSummary>("/judging/rounds", payload);
    return data;
  },

  async listRounds(): Promise<RoundSummary[]> {
    const { data } = await apiClient.get<RoundSummary[]>("/judging/rounds");
    return data;
  },

  async getRound(id: string): Promise<RoundSummary> {
    const { data } = await apiClient.get<RoundSummary>(`/judging/rounds/${id}`);
    return data;
  },

  async activateRound(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/rounds/${id}/activate`
    );
    return data;
  },

  async getAdminLeaderboard(roundId: string): Promise<AdminLeaderboardResponse> {
    const { data } = await apiClient.get<AdminLeaderboardResponse>(
      `/judging/rounds/${roundId}/leaderboard`
    );
    return data;
  },

  async createPanel(payload: CreatePanelRequest): Promise<{ id: string }> {
    const { data } = await apiClient.post<{ id: string }>("/judging/panels", payload);
    return data;
  },

  async configureSlots(
    panelId: string,
    payload: ConfigureSlotsRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/panels/${panelId}/slots`,
      payload
    );
    return data;
  },
};
