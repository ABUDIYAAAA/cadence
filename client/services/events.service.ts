import { apiClient } from "@/lib/api-client";
import {
  normalizeEventUser,
  normalizeEventUsersPage,
} from "@/lib/tenant-scope";
import { MessageResponse } from "@/types/auth";
import {
  AddEventUserRequest,
  BanMemberRequest,
  BulkAddEventUsersRequest,
  BulkAddEventUsersResponse,
  CreateEventRequest,
  Event,
  EventResponse,
  EventUserDetailResponse,
  EventUserFilter,
  EventUserListResponse,
  TransferOwnershipRequest,
  UpdateEventRequest,
  UpdateMemberRoleRequest,
} from "@/types/events";
import {
  BulkCreateResponse,
  PaginatedUsersResponse,
  UserDetailResponse,
} from "@/types/users";

export { normalizeEventUser, normalizeEventUsersPage };

export const eventsService = {
  // Global event endpoints (/api/v2/events)
  async listEvents(): Promise<Event[]> {
    const { data } = await apiClient.get<Event[]>("/events");
    return Array.isArray(data) ? data : [];
  },

  async createEvent(payload: CreateEventRequest): Promise<Event> {
    const { data } = await apiClient.post<Event>("/events", payload);
    return data;
  },

  async deleteEvent(eventSlug: string): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(
      `/events/${encodeURIComponent(eventSlug)}`
    );
    return data;
  },

  // Event-scoped endpoints (/api/v2/e/{eventSlug})
  async getEvent(eventSlug: string): Promise<EventResponse> {
    const { data } = await apiClient.get<EventResponse>(
      `/e/${encodeURIComponent(eventSlug)}`
    );
    return data;
  },

  async updateEvent(
    eventSlug: string,
    payload: UpdateEventRequest
  ): Promise<Event> {
    const { data } = await apiClient.patch<Event>(
      `/e/${encodeURIComponent(eventSlug)}`,
      payload
    );
    return data;
  },

  async listEventUsers(
    eventSlug: string,
    filters?: EventUserFilter
  ): Promise<PaginatedUsersResponse> {
    const { data } = await apiClient.get<EventUserListResponse>(
      `/e/${encodeURIComponent(eventSlug)}/users`,
      { params: filters }
    );
    return normalizeEventUsersPage(data);
  },

  async addEventUser(
    eventSlug: string,
    payload: AddEventUserRequest
  ): Promise<UserDetailResponse> {
    const { data } = await apiClient.post<EventUserDetailResponse>(
      `/e/${encodeURIComponent(eventSlug)}/users`,
      payload
    );
    return normalizeEventUser(data);
  },

  async bulkAddEventUsers(
    eventSlug: string,
    payload: BulkAddEventUsersRequest
  ): Promise<BulkCreateResponse> {
    const { data } = await apiClient.post<BulkAddEventUsersResponse>(
      `/e/${encodeURIComponent(eventSlug)}/users/bulk`,
      payload
    );
    return {
      total_processed: data.total_processed,
      total_created: (data.total_created ?? 0) + (data.total_added ?? 0),
      total_added: data.total_added,
      skipped_count: data.skipped_count,
      skipped_errors: data.skipped_errors,
      users: (data.users ?? []).map(normalizeEventUser),
    };
  },

  async updateMemberRole(
    eventSlug: string,
    userId: string,
    payload: UpdateMemberRoleRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.patch<MessageResponse>(
      `/e/${encodeURIComponent(eventSlug)}/members/${encodeURIComponent(userId)}`,
      payload
    );
    return data;
  },

  async removeMember(
    eventSlug: string,
    userId: string
  ): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(
      `/e/${encodeURIComponent(eventSlug)}/members/${encodeURIComponent(userId)}`
    );
    return data;
  },

  async banMember(
    eventSlug: string,
    userId: string,
    payload?: BanMemberRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/e/${encodeURIComponent(eventSlug)}/members/${encodeURIComponent(userId)}/ban`,
      payload ?? {}
    );
    return data;
  },

  async unbanMember(
    eventSlug: string,
    userId: string
  ): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/e/${encodeURIComponent(eventSlug)}/members/${encodeURIComponent(userId)}/unban`
    );
    return data;
  },

  async transferOwnership(
    eventSlug: string,
    payload: TransferOwnershipRequest
  ): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/e/${encodeURIComponent(eventSlug)}/transfer-ownership`,
      payload
    );
    return data;
  },
};
