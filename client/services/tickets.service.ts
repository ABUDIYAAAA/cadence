import { apiClient } from "@/lib/api-client";
import {
  AnalyticsResponse,
  BulkSendTicketsRequest,
  BulkSendTicketsResponse,
  CheckInRequest,
  CheckInResponse,
  CreateScanSessionRequest,
  MyTicketResponse,
  ReleaseTicketsResponse,
  ScanSession,
  SessionReport,
  SessionScanResponse,
  TicketSettings,
  SingleSendTicketResponse,
} from "@/types/tickets";

export const ticketsService = {
  async getMyTicket(): Promise<MyTicketResponse> {
    const { data } = await apiClient.get<MyTicketResponse>("/tickets/me");
    return data;
  },

  async checkIn(payload: CheckInRequest): Promise<CheckInResponse> {
    const { data } = await apiClient.post<CheckInResponse>("/tickets/check-in", payload);
    return data;
  },

  async getAnalytics(): Promise<AnalyticsResponse> {
    const { data } = await apiClient.get<AnalyticsResponse>("/tickets/analytics");
    return data;
  },

  async bulkSend(payload: BulkSendTicketsRequest): Promise<BulkSendTicketsResponse> {
    const { data } = await apiClient.post<BulkSendTicketsResponse>(
      "/tickets/bulk-send",
      payload
    );
    return data;
  },

  async sendSingle(userId: string): Promise<SingleSendTicketResponse> {
    const { data } = await apiClient.post<SingleSendTicketResponse>(
      `/tickets/users/${userId}/send`
    );
    return data;
  },

  async release(payload: BulkSendTicketsRequest): Promise<ReleaseTicketsResponse> {
    const { data } = await apiClient.post<ReleaseTicketsResponse>("/tickets/release", payload);
    return data;
  },

  async listSessions(): Promise<ScanSession[]> {
    const { data } = await apiClient.get<ScanSession[]>("/tickets/sessions");
    return data;
  },

  async createSession(payload: CreateScanSessionRequest): Promise<ScanSession> {
    const { data } = await apiClient.post<ScanSession>("/tickets/sessions", payload);
    return data;
  },

  async setSessionOpen(id: string, isOpen: boolean): Promise<ScanSession> {
    const { data } = await apiClient.patch<ScanSession>(`/tickets/sessions/${id}`, { is_open: isOpen });
    return data;
  },

  async scanInSession(id: string, ticketCode: string): Promise<SessionScanResponse> {
    const { data } = await apiClient.post<SessionScanResponse>(`/tickets/sessions/${id}/scan`, {
      ticket_code: ticketCode,
    });
    return data;
  },

  async getSessionReport(id: string): Promise<SessionReport> {
    const { data } = await apiClient.get<SessionReport>(`/tickets/sessions/${id}/report`);
    return data;
  },

  async getSettings(): Promise<TicketSettings> {
    const { data } = await apiClient.get<TicketSettings>("/tickets/settings");
    return data;
  },

  async updateSettings(internalEmailDomains: string[]): Promise<TicketSettings> {
    const { data } = await apiClient.put<TicketSettings>("/tickets/settings", {
      internal_email_domains: internalEmailDomains,
    });
    return data;
  },
};
