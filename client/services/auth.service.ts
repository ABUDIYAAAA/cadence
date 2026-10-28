import { apiClient } from "@/lib/api-client";
import {
  AuthResponse,
  ForgotPasswordRequest,
  LoginRequest,
  MessageResponse,
  ResetPasswordRequest,
  SetupPasswordRequest,
  UserResponse,
} from "@/types/auth";

export const authService = {
  async login(payload: LoginRequest): Promise<AuthResponse> {
    const { data } = await apiClient.post<AuthResponse>("/auth/login", payload);
    return data;
  },

  async me(): Promise<UserResponse> {
    const { data } = await apiClient.get<UserResponse>("/auth/me");
    return data;
  },

  async logout(): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>("/auth/logout");
    return data;
  },

  async refresh(): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>("/auth/refresh");
    return data;
  },

  async setupPassword(payload: SetupPasswordRequest): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      "/auth/setup-password",
      payload
    );
    return data;
  },

  async forgotPassword(payload: ForgotPasswordRequest): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      "/auth/forgot-password",
      payload
    );
    return data;
  },

  async resetPassword(payload: ResetPasswordRequest): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>(
      "/auth/reset-password",
      payload
    );
    return data;
  },
};
