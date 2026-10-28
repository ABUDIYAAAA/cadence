"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { usersService } from "@/services/users.service";
import {
  BulkCreateUsersRequest,
  BulkInviteRequest,
  CreateUserRequest,
  UpdateUserRequest,
  UserFilter,
} from "@/types/users";

export const USERS_QUERY_KEY = ["users"] as const;

export function useUsers(filters?: UserFilter) {
  return useQuery({
    queryKey: [...USERS_QUERY_KEY, "list", filters],
    queryFn: () => usersService.list(filters),
  });
}

export function useUser(id: string) {
  return useQuery({
    queryKey: [...USERS_QUERY_KEY, "detail", id],
    queryFn: () => usersService.getById(id),
    enabled: Boolean(id),
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CreateUserRequest) => usersService.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useBulkCreateUsers() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: BulkCreateUsersRequest) =>
      usersService.bulkCreate(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateUserRequest }) =>
      usersService.update(id, payload),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
      queryClient.invalidateQueries({
        queryKey: [...USERS_QUERY_KEY, "detail", variables.id],
      });
    },
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => usersService.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useSendInvite() {
  return useMutation({
    mutationFn: (id: string) => usersService.sendInvite(id),
  });
}

export function useBulkSendInvite() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: BulkInviteRequest) =>
      usersService.bulkSendInvite(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useTriggerResetPassword() {
  return useMutation({
    mutationFn: (id: string) => usersService.triggerResetPassword(id),
  });
}
