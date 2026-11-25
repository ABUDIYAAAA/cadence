"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authService } from "@/services/auth.service";
import { clearTenantScope } from "@/lib/api-client";
import { MessageResponse, UserResponse } from "@/types/auth";
import { ApiError } from "@/types/api";

export const AUTH_QUERY_KEY = ["auth", "me"] as const;

export function useCurrentUser() {
  return useQuery<UserResponse, ApiError>({
    queryKey: AUTH_QUERY_KEY,
    queryFn: () => authService.me(),
    retry: false,
    staleTime: 1000 * 60 * 5, // 5 mins
  });
}

export function useLogout() {
  const queryClient = useQueryClient();

  return useMutation<MessageResponse, ApiError, void>({
    mutationFn: () => authService.logout(),
    onSuccess: () => {
      clearTenantScope();
      queryClient.setQueryData(AUTH_QUERY_KEY, null);
      queryClient.clear();
    },
  });
}
