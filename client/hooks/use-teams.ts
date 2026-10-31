"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { teamsService } from "@/services/teams.service";
import {
  AddMemberRequest,
  CreateTeamRequest,
  JoinTeamRequest,
  RenameTeamRequest,
  TeamFilter,
  UpdateTeamRequest,
  UpdateTeamSettingsRequest,
} from "@/types/teams";

export const TEAMS_QUERY_KEY = ["teams"] as const;

/** Anything that changes membership also changes the users list (team_id / team_name). */
function useInvalidateTeams() {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: TEAMS_QUERY_KEY });
    queryClient.invalidateQueries({ queryKey: ["users"] });
    // The judging status card is derived from the caller's team.
    queryClient.invalidateQueries({ queryKey: ["judging"] });
  };
}

/** Resolves to `null` data (not an error) when the caller has no team. */
export function useMyTeam(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...TEAMS_QUERY_KEY, "my-team"],
    queryFn: () => teamsService.getMyTeam(),
    retry: false,
    // Keep teammates' joins/kicks and freezes visible without a manual refresh.
    refetchInterval: 15_000,
    enabled: options?.enabled ?? true,
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

export function useTeamSettings(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...TEAMS_QUERY_KEY, "settings"],
    queryFn: () => teamsService.getSettings(),
    enabled: options?.enabled ?? true,
  });
}

// ---- participant mutations ----

export function useCreateTeam() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (payload: CreateTeamRequest) => teamsService.create(payload),
    onSuccess: invalidate,
  });
}

export function useJoinTeam() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (payload: JoinTeamRequest) => teamsService.join(payload),
    onSuccess: invalidate,
  });
}

export function useLeaveTeam() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: () => teamsService.leave(),
    onSuccess: invalidate,
  });
}

export function useRenameMyTeam() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (payload: RenameTeamRequest) => teamsService.renameMine(payload),
    onSuccess: invalidate,
  });
}

export function useRegenerateMyCode() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: () => teamsService.regenerateMyCode(),
    onSuccess: invalidate,
  });
}

export function useKickMember() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (userId: string) => teamsService.kickMember(userId),
    onSuccess: invalidate,
  });
}

// ---- staff mutations ----

export function useUpdateTeam() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateTeamRequest }) =>
      teamsService.update(id, payload),
    onSuccess: invalidate,
  });
}

export function useDeleteTeam() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (id: string) => teamsService.delete(id),
    onSuccess: invalidate,
  });
}

export function useAddTeamMember() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: ({ teamId, payload }: { teamId: string; payload: AddMemberRequest }) =>
      teamsService.addMember(teamId, payload),
    onSuccess: invalidate,
  });
}

export function useRemoveTeamMember() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: ({ teamId, userId }: { teamId: string; userId: string }) =>
      teamsService.removeMember(teamId, userId),
    onSuccess: invalidate,
  });
}

export function useRegenerateTeamCode() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (id: string) => teamsService.regenerateCode(id),
    onSuccess: invalidate,
  });
}

export function useSetTeamFrozen() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: ({ id, frozen }: { id: string; frozen: boolean }) =>
      frozen ? teamsService.freeze(id) : teamsService.unfreeze(id),
    onSuccess: invalidate,
  });
}

export function useSetAllTeamsFrozen() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (frozen: boolean) =>
      frozen ? teamsService.freezeAll() : teamsService.unfreezeAll(),
    onSuccess: invalidate,
  });
}

export function useUpdateTeamSettings() {
  const invalidate = useInvalidateTeams();
  return useMutation({
    mutationFn: (payload: UpdateTeamSettingsRequest) => teamsService.updateSettings(payload),
    onSuccess: invalidate,
  });
}
