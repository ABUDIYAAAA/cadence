import { apiClient } from "@/lib/api-client";
import {
  AssignedJudge,
  RoundPanelOverview,
  AdminLeaderboardResponse,
  ConfigureSlotsRequest,
  CreatePanelRequest,
  CreatePanelResponse,
  CreateRoundRequest,
  CreateRoundResponse,
  JudgePanelSummary,
  JudgeSlotDetails,
  MoveSlotRequest,
  RoundDetail,
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

  async pauseTimer(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/pause`
    );
    return data;
  },

  async resumeTimer(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/resume`
    );
    return data;
  },

  async extendTimer(slotId: string, seconds: number): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/extend`,
      { seconds }
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
  async createRound(payload: CreateRoundRequest): Promise<CreateRoundResponse> {
    const { data } = await apiClient.post<CreateRoundResponse>("/judging/rounds", payload);
    return data;
  },

  async listRounds(): Promise<RoundSummary[]> {
    const { data } = await apiClient.get<RoundSummary[]>("/judging/rounds");
    return data;
  },

  async getRound(id: string): Promise<RoundDetail> {
    const { data } = await apiClient.get<RoundDetail>(`/judging/rounds/${id}`);
    return data;
  },

  async endRound(id: string, force = false): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/rounds/${id}/end`,
      null,
      { params: force ? { force: true } : undefined }
    );
    return data;
  },

  async deleteRound(id: string, force = false): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(
      `/judging/rounds/${id}`,
      { params: force ? { force: true } : undefined }
    );
    return data;
  },

  async getRoundPanels(roundId: string): Promise<RoundPanelOverview[]> {
    const { data } = await apiClient.get<RoundPanelOverview[]>(
      `/judging/rounds/${roundId}/panels`
    );
    return data;
  },

  async getAssignedJudges(roundId: string): Promise<AssignedJudge[]> {
    const { data } = await apiClient.get<AssignedJudge[]>(
      `/judging/rounds/${roundId}/assigned-judges`
    );
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

  async createPanel(payload: CreatePanelRequest): Promise<CreatePanelResponse> {
    const { data } = await apiClient.post<CreatePanelResponse>("/judging/panels", payload);
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

  async moveSlot(
    slotId: string,
    payload: MoveSlotRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/move`,
      payload
    );
    return data;
  },

  async queueSlot(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/queue`
    );
    return data;
  },

  async unqueueSlot(slotId: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/judging/slots/${slotId}/unqueue`
    );
    return data;
  },
};
