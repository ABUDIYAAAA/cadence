"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { competitionsService } from "@/services/competitions.service";
import {
  Competition,
  CreateCompetitionRequest,
  UpdateCompetitionRequest,
} from "@/types/competitions";
import {
  AddEventUserRequest,
  BanMemberRequest,
  BulkAddEventUsersRequest,
  EventUserFilter,
  UpdateMemberRoleRequest,
} from "@/types/events";
import { USERS_QUERY_KEY } from "@/hooks/use-users";

export const COMPETITIONS_QUERY_KEY = ["competitions"] as const;

export const competitionUsersKey = (
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) => [...COMPETITIONS_QUERY_KEY, eventSlug, competitionSlug, "users"] as const;

export function useCompetitions(
  eventSlug: string | null | undefined,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: [...COMPETITIONS_QUERY_KEY, eventSlug],
    queryFn: () => competitionsService.listByEvent(eventSlug!),
    enabled: Boolean(eventSlug) && (options?.enabled ?? true),
  });
}

export function useCreateCompetition(eventSlug: string | null | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateCompetitionRequest) =>
      competitionsService.createCompetition(eventSlug!, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<Competition[]>(
        [...COMPETITIONS_QUERY_KEY, eventSlug],
        (prev) => (prev ? [...prev, created] : [created])
      );
      queryClient.invalidateQueries({
        queryKey: [...COMPETITIONS_QUERY_KEY, eventSlug],
      });
    },
  });
}

export function useUpdateCompetition(eventSlug: string | null | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      competitionSlug,
      payload,
    }: {
      competitionSlug: string;
      payload: UpdateCompetitionRequest;
    }) =>
      competitionsService.updateCompetition(
        eventSlug!,
        competitionSlug,
        payload
      ),
    onSuccess: (updated, variables) => {
      queryClient.setQueryData<Competition[]>(
        [...COMPETITIONS_QUERY_KEY, eventSlug],
        (prev) =>
          prev
            ? prev.map((c) =>
                c.slug === variables.competitionSlug || c.id === updated.id
                  ? updated
                  : c
              )
            : [updated]
      );
      queryClient.invalidateQueries({
        queryKey: [...COMPETITIONS_QUERY_KEY, eventSlug],
      });
    },
  });
}

export function useDeleteCompetition(eventSlug: string | null | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (competitionSlug: string) =>
      competitionsService.deleteCompetition(eventSlug!, competitionSlug),
    onSuccess: (_data, deletedSlug) => {
      queryClient.setQueryData<Competition[]>(
        [...COMPETITIONS_QUERY_KEY, eventSlug],
        (prev) => (prev ? prev.filter((c) => c.slug !== deletedSlug) : [])
      );
      queryClient.invalidateQueries({
        queryKey: [...COMPETITIONS_QUERY_KEY, eventSlug],
      });
    },
  });
}

export function useCompetitionUsers(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined,
  filters?: EventUserFilter,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: [...competitionUsersKey(eventSlug, competitionSlug), filters],
    queryFn: () =>
      competitionsService.listCompetitionUsers(
        eventSlug!,
        competitionSlug!,
        filters
      ),
    enabled:
      Boolean(eventSlug) &&
      Boolean(competitionSlug) &&
      (options?.enabled ?? true),
  });
}

export function useAddCompetitionUser(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: AddEventUserRequest) =>
      competitionsService.addCompetitionUser(
        eventSlug!,
        competitionSlug!,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: competitionUsersKey(eventSlug, competitionSlug),
      });
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

export function useBulkAddCompetitionUsers(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: BulkAddEventUsersRequest) =>
      competitionsService.bulkAddCompetitionUsers(
        eventSlug!,
        competitionSlug!,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: competitionUsersKey(eventSlug, competitionSlug),
      });
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

export function useUpdateCompetitionMemberRole(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      userId,
      payload,
    }: {
      userId: string;
      payload: UpdateMemberRoleRequest;
    }) =>
      competitionsService.updateCompetitionMemberRole(
        eventSlug!,
        competitionSlug!,
        userId,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: competitionUsersKey(eventSlug, competitionSlug),
      });
    },
  });
}

export function useRemoveCompetitionMember(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) =>
      competitionsService.removeCompetitionMember(
        eventSlug!,
        competitionSlug!,
        userId
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: competitionUsersKey(eventSlug, competitionSlug),
      });
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

export function useBanCompetitionMember(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      userId,
      payload,
    }: {
      userId: string;
      payload?: BanMemberRequest;
    }) =>
      competitionsService.banCompetitionMember(
        eventSlug!,
        competitionSlug!,
        userId,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: competitionUsersKey(eventSlug, competitionSlug),
      });
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

export function useUnbanCompetitionMember(
  eventSlug: string | null | undefined,
  competitionSlug: string | null | undefined
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) =>
      competitionsService.unbanCompetitionMember(
        eventSlug!,
        competitionSlug!,
        userId
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: competitionUsersKey(eventSlug, competitionSlug),
      });
    },
  });
}
