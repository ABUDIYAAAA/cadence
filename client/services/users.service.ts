import { apiClient } from "@/lib/api-client";
import {
  BulkCreateResponse,
  BulkCreateUsersRequest,
  CreateUserRequest,
  PaginatedUsersResponse,
  UpdateUserRequest,
  UserDetailResponse,
  UserFilter,
} from "@/types/users";
import { MessageResponse } from "@/types/auth";

export const usersService = {
  async list(filters?: UserFilter): Promise<PaginatedUsersResponse> {
    const { data } = await apiClient.get<PaginatedUsersResponse>("/users", {
      params: filters,
    });
    return data;
  },

  async getById(id: string): Promise<UserDetailResponse> {
    const { data } = await apiClient.get<UserDetailResponse>(`/users/${id}`);
    return data;
  },

  async create(payload: CreateUserRequest): Promise<UserDetailResponse> {
    const { data } = await apiClient.post<UserDetailResponse>("/users", payload);
    return data;
  },

  async bulkCreate(payload: BulkCreateUsersRequest): Promise<BulkCreateResponse> {
    const { data } = await apiClient.post<BulkCreateResponse>("/users/bulk", payload);
    return data;
  },

  async update(id: string, payload: UpdateUserRequest): Promise<UserDetailResponse> {
    const { data } = await apiClient.patch<UserDetailResponse>(`/users/${id}`, payload);
    return data;
  },

  async delete(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.delete<MessageResponse>(`/users/${id}`);
    return data;
  },

  async triggerResetPassword(id: string): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      `/users/${id}/trigger-reset-password`
    );
    return data;
  },
};
