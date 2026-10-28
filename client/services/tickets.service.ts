import { apiClient } from "@/lib/api-client";
import {
  AnalyticsResponse,
  BulkSendTicketsRequest,
  BulkSendTicketsResponse,
  CheckInRequest,
  CheckInResponse,
  MyTicketResponse,
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
};
