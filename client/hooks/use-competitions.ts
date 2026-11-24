"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { competitionsService } from "@/services/competitions.service";
import {
  Competition,
  CreateCompetitionRequest,
  UpdateCompetitionRequest,
} from "@/types/competitions";

export const COMPETITIONS_QUERY_KEY = ["competitions"] as const;

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
