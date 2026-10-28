"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { teamsService } from "@/services/teams.service";
import {
  AddMemberRequest,
  CreateTeamRequest,
  TeamFilter,
  UpdateTeamRequest,
} from "@/types/teams";

export const TEAMS_QUERY_KEY = ["teams"] as const;

export function useMyTeam() {
  return useQuery({
    queryKey: [...TEAMS_QUERY_KEY, "my-team"],
    queryFn: () => teamsService.getMyTeam(),
    retry: false,
  });
}

export function useTeams(filters?: TeamFilter) {
  return useQuery({
    queryKey: [...TEAMS_QUERY_KEY, "list", filters],
    queryFn: () => teamsService.list(filters),
  });
}

export function useTeam(id: string) {
  return useQuery({
    queryKey: [...TEAMS_QUERY_KEY, "detail", id],
    queryFn: () => teamsService.getById(id),
    enabled: Boolean(id),
  });
}

export function useCreateTeam() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CreateTeamRequest) => teamsService.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TEAMS_QUERY_KEY });
    },
  });
}

export function useUpdateTeam() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateTeamRequest }) =>
      teamsService.update(id, payload),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: TEAMS_QUERY_KEY });
      queryClient.invalidateQueries({
        queryKey: [...TEAMS_QUERY_KEY, "detail", variables.id],
      });
    },
  });
}

export function useDeleteTeam() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => teamsService.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TEAMS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useAddTeamMember() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      teamId,
      payload,
    }: {
      teamId: string;
      payload: AddMemberRequest;
    }) => teamsService.addMember(teamId, payload),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: [...TEAMS_QUERY_KEY, "detail", variables.teamId],
      });
      queryClient.invalidateQueries({ queryKey: [...TEAMS_QUERY_KEY, "list"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useRemoveTeamMember() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ teamId, userId }: { teamId: string; userId: string }) =>
      teamsService.removeMember(teamId, userId),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: [...TEAMS_QUERY_KEY, "detail", variables.teamId],
      });
      queryClient.invalidateQueries({ queryKey: [...TEAMS_QUERY_KEY, "list"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}
