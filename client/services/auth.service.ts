import { API_BASE_URL, apiClient } from "@/lib/api-client";
import { buildGoogleLoginUrl } from "@/lib/oauth-callback";
import { MessageResponse, UserResponse } from "@/types/auth";

export const authService = {
  getGoogleLoginUrl(redirectPath?: string | null): string {
    return buildGoogleLoginUrl(API_BASE_URL, redirectPath);
  },

  async me(): Promise<UserResponse> {
    const { data } = await apiClient.get<UserResponse>("/auth/me");
    return data;
  },

  async logout(): Promise<MessageResponse> {
    const { data } = await apiClient.post<MessageResponse>("/auth/logout");
    return data;
  },
};
