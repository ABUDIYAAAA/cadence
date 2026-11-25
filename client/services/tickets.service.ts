import { apiClient } from "../lib/api-client.ts";
import { normalizeCheckInCode } from "../lib/tickets-utils.ts";
import type {
  AnalyticsResponse,
  BulkSendTicketsRequest,
  BulkSendTicketsResponse,
  CheckInRequest,
  CheckInResponse,
  CreateScanSessionRequest,
  MyTicketResponse,
  ReleaseTicketsResponse,
  ScanSession,
  SendSingleTicketRequest,
  SessionReport,
  SessionScanResponse,
  SingleSendTicketResponse,
  TicketSettings,
  UpdateTicketSettingsRequest,
} from "../types/tickets.ts";

export interface TicketsHttpClient {
  get<T>(url: string): Promise<{ data: T }>;
  post<T>(url: string, payload?: unknown): Promise<{ data: T }>;
  patch<T>(url: string, payload?: unknown): Promise<{ data: T }>;
  put<T>(url: string, payload?: unknown): Promise<{ data: T }>;
  delete<T>(url: string): Promise<{ data: T }>;
}

export function createTicketsService(client: TicketsHttpClient = apiClient) {
  return {
    async getMyTicket(): Promise<MyTicketResponse> {
      const { data } = await client.get<MyTicketResponse>("/tickets/me");
      return data;
    },

    async checkIn(payload: CheckInRequest): Promise<CheckInResponse> {
      const { data } = await client.post<CheckInResponse>(
        "/tickets/check-in",
        payload
      );
      return data;
    },

    async checkInByCode(code: string): Promise<SessionScanResponse> {
      const { data } = await client.post<SessionScanResponse>(
        "/tickets/check-in/code",
        { code: code.trim() }
      );
      return data;
    },

    async getAnalytics(): Promise<AnalyticsResponse> {
      const { data } = await client.get<AnalyticsResponse>(
        "/tickets/analytics"
      );
      return data;
    },

    async bulkSend(
      payload: BulkSendTicketsRequest
    ): Promise<BulkSendTicketsResponse> {
      const { data } = await client.post<BulkSendTicketsResponse>(
        "/tickets/bulk-send",
        payload
      );
      return data;
    },

    async sendSingle(
      userId: string,
      payload?: SendSingleTicketRequest
    ): Promise<SingleSendTicketResponse> {
      const { data } = await client.post<SingleSendTicketResponse>(
        `/tickets/users/${encodeURIComponent(userId)}/send`,
        payload ?? {}
      );
      return data;
    },

    async release(
      payload: BulkSendTicketsRequest
    ): Promise<ReleaseTicketsResponse> {
      const { data } = await client.post<ReleaseTicketsResponse>(
        "/tickets/release",
        payload
      );
      return data;
    },

    async listSessions(): Promise<ScanSession[]> {
      const { data } = await client.get<ScanSession[]>("/tickets/sessions");
      return data;
    },

    async createSession(
      payload: CreateScanSessionRequest
    ): Promise<ScanSession> {
      const { data } = await client.post<ScanSession>(
        "/tickets/sessions",
        payload
      );
      return data;
    },

    async setSessionOpen(id: string, isOpen: boolean): Promise<ScanSession> {
      const { data } = await client.patch<ScanSession>(
        `/tickets/sessions/${encodeURIComponent(id)}`,
        { is_open: isOpen }
      );
      return data;
    },

    async generateSessionCode(
      sessionId: string,
      customCode?: string
    ): Promise<ScanSession> {
      const trimmed = normalizeCheckInCode(customCode);
      const { data } = await client.post<ScanSession>(
        `/tickets/sessions/${encodeURIComponent(sessionId)}/code`,
        trimmed ? { code: trimmed } : {}
      );
      return data;
    },

    async clearSessionCode(sessionId: string): Promise<ScanSession> {
      const { data } = await client.delete<ScanSession>(
        `/tickets/sessions/${encodeURIComponent(sessionId)}/code`
      );
      return data;
    },

    async generateDefaultCheckInCode(
      customCode?: string
    ): Promise<ScanSession> {
      const trimmed = normalizeCheckInCode(customCode);
      const { data } = await client.post<ScanSession>(
        "/tickets/checkin-code",
        trimmed ? { code: trimmed } : {}
      );
      return data;
    },

    async clearDefaultCheckInCode(): Promise<ScanSession> {
      const { data } = await client.delete<ScanSession>(
        "/tickets/checkin-code"
      );
      return data;
    },

    async scanInSession(
      id: string,
      ticketCode: string
    ): Promise<SessionScanResponse> {
      const { data } = await client.post<SessionScanResponse>(
        `/tickets/sessions/${encodeURIComponent(id)}/scan`,
        {
          ticket_code: ticketCode,
        }
      );
      return data;
    },

    async getSessionReport(id: string): Promise<SessionReport> {
      const { data } = await client.get<SessionReport>(
        `/tickets/sessions/${encodeURIComponent(id)}/report`
      );
      return data;
    },

    async getSettings(): Promise<TicketSettings> {
      const { data } = await client.get<TicketSettings>("/tickets/settings");
      return data;
    },

    async updateSettings(
      payload: UpdateTicketSettingsRequest
    ): Promise<TicketSettings> {
      const { data } = await client.put<TicketSettings>(
        "/tickets/settings",
        payload
      );
      return data;
    },
  };
}

export const ticketsService = createTicketsService(apiClient);
