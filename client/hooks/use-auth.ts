"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authService } from "@/services/auth.service";
import {
  AuthResponse,
  ForgotPasswordRequest,
  LoginRequest,
  MessageResponse,
  ResetPasswordRequest,
  SetupPasswordRequest,
  UserResponse,
} from "@/types/auth";
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

export function useLogin() {
  const queryClient = useQueryClient();

  return useMutation<AuthResponse, ApiError, LoginRequest>({
    mutationFn: (payload: LoginRequest) => authService.login(payload),
    onSuccess: (data) => {
      queryClient.setQueryData(AUTH_QUERY_KEY, data.user);
    },
  });
}

export function useLogout() {
  const queryClient = useQueryClient();

  return useMutation<MessageResponse, ApiError, void>({
    mutationFn: () => authService.logout(),
    onSuccess: () => {
      queryClient.setQueryData(AUTH_QUERY_KEY, null);
      queryClient.clear();
    },
  });
}

export function useSetupPassword() {
  return useMutation<MessageResponse, ApiError, SetupPasswordRequest>({
    mutationFn: (payload: SetupPasswordRequest) => authService.setupPassword(payload),
  });
}

export function useForgotPassword() {
  return useMutation<MessageResponse, ApiError, ForgotPasswordRequest>({
    mutationFn: (payload: ForgotPasswordRequest) => authService.forgotPassword(payload),
  });
}

export function useResetPassword() {
  return useMutation<MessageResponse, ApiError, ResetPasswordRequest>({
    mutationFn: (payload: ResetPasswordRequest) => authService.resetPassword(payload),
  });
}
