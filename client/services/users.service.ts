import { apiClient, getTenantScope } from "@/lib/api-client";
import { eventsService } from "@/services/events.service";
import {
  BulkCreateResponse,
  BulkCreateUsersRequest,
  CreateUserRequest,
  PaginatedUsersResponse,
  UpdateUserRequest,
  UserDetailResponse,
  UserFilter,
} from "@/types/users";
import { AssignableEventRole, MessageResponse } from "@/types/auth";

function toAssignableRole(role?: string): AssignableEventRole | undefined {
  if (
    role === "admin" ||
    role === "judge" ||
    role === "volunteer" ||
    role === "participant"
  ) {
    return role;
  }
  return undefined;
}

export const usersService = {
  /**
   * Lists members in the active event when an event is selected,
   * falling back to platform-wide /users (dev-only) when no event is active.
   */
  async list(filters?: UserFilter): Promise<PaginatedUsersResponse> {
    const { eventSlug } = getTenantScope();
    if (eventSlug) {
      return eventsService.listEventUsers(eventSlug, {
        page: filters?.page,
        limit: filters?.limit,
        role: filters?.role,
        competition_id: filters?.competition_id,
        team_id: filters?.team_id,
        no_team: filters?.no_team,
        search: filters?.search,
        is_banned:
          filters?.is_banned !== undefined
            ? filters.is_banned
            : filters?.is_active !== undefined
            ? !filters.is_active
            : undefined,
        checkin_status: filters?.checkin_status,
      });
    }
    return this.listPlatform(filters);
  },

  // Platform-wide user endpoints (/api/v2/users — restricted to global dev role)
  async listPlatform(filters?: UserFilter): Promise<PaginatedUsersResponse> {
    const { data } = await apiClient.get<PaginatedUsersResponse>("/users", {
      params: filters,
    });
    return data;
  },

  async create(payload: CreateUserRequest): Promise<UserDetailResponse> {
    const { eventSlug } = getTenantScope();
    if (eventSlug) {
      return eventsService.addEventUser(eventSlug, {
        email: payload.email,
        name: payload.name,
        role: toAssignableRole(payload.role),
        phone_number: payload.phone_number,
      });
    }
    return this.createPlatform(payload);
  },

  async createPlatform(payload: CreateUserRequest): Promise<UserDetailResponse> {
    const { data } = await apiClient.post<UserDetailResponse>("/users", payload);
    return data;
  },

  async bulkCreate(
    payload: BulkCreateUsersRequest
  ): Promise<BulkCreateResponse> {
    const { eventSlug } = getTenantScope();
    if (eventSlug) {
      return eventsService.bulkAddEventUsers(eventSlug, {
        users: payload.users.map((u) => ({
          email: u.email,
          name: u.name,
          role: toAssignableRole(u.role),
          phone_number: u.phone_number,
          competition_name: u.competition_name,
          team_name: u.team_name,
        })),
      });
    }
    return this.bulkCreatePlatform(payload);
  },

  async bulkCreatePlatform(
    payload: BulkCreateUsersRequest
  ): Promise<BulkCreateResponse> {
    const { data } = await apiClient.post<BulkCreateResponse>(
      "/users/bulk",
      payload
    );
    return data;
  },

  async update(
    id: string,
    payload: UpdateUserRequest
  ): Promise<UserDetailResponse> {
    const { data } = await apiClient.patch<UserDetailResponse>(
      `/users/${id}`,
      payload
    );
    return data;
  },

  async delete(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(`/users/${id}`);
    return data;
  },

  async reactivate(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/users/${id}/reactivate`
    );
    return data;
  },

  async blacklist(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/users/${id}/blacklist`
    );
    return data;
  },

  async unblacklist(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/users/${id}/unblacklist`
    );
    return data;
  },
};
