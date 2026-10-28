"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { judgingService } from "@/services/judging.service";
import {
  ConfigureSlotsRequest,
  CreatePanelRequest,
  CreateRoundRequest,
  SubmitScoresRequest,
  UpdateSlotLinksRequest,
} from "@/types/judging";

export const JUDGING_QUERY_KEY = ["judging"] as const;

// Participant
export function useTeamJudgingStatus() {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "team-status"],
    queryFn: () => judgingService.getMyTeamStatus(),
    refetchInterval: 1000 * 15, // poll queue progress every 15s
  });
}

export function useUpdateSlotLinks() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      slotId,
      payload,
    }: {
      slotId: string;
      payload: UpdateSlotLinksRequest;
    }) => judgingService.updateSlotLinks(slotId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "team-status"],
      });
    },
  });
}

// Volunteer
export function useVolunteerSchedule() {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "volunteer-schedule"],
    queryFn: () => judgingService.getVolunteerSchedule(),
    refetchInterval: 1000 * 10, // volunteer updates frequently
  });
}

// Judge
export function useJudgePanels() {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "judge-panels"],
    queryFn: () => judgingService.getJudgePanels(),
  });
}

export function useJudgeSlotDetails(slotId: string) {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "slot-details", slotId],
    queryFn: () => judgingService.getJudgeSlotDetails(slotId),
    enabled: Boolean(slotId),
  });
}

export function useStartSlot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.startSlot(slotId),
    onSuccess: (_data, slotId) => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "slot-details", slotId],
      });
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "judge-panels"],
      });
    },
  });
}

export function useSubmitScores() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      slotId,
      payload,
    }: {
      slotId: string;
      payload: SubmitScoresRequest;
    }) => judgingService.submitScores(slotId, payload),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "slot-details", variables.slotId],
      });
    },
  });
}

export function useLockScores() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.lockScores(slotId),
    onSuccess: (_data, slotId) => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "slot-details", slotId],
      });
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "judge-panels"],
      });
    },
  });
}

export function useCompleteSlot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.completeSlot(slotId),
    onSuccess: (_data, slotId) => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "slot-details", slotId],
      });
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "judge-panels"],
      });
    },
  });
}

// Rounds listing & inspection (Admin & Judge)
export function useJudgingRounds(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "rounds"],
    queryFn: () => judgingService.listRounds(),
    enabled: options?.enabled ?? true,
  });
}

export function useJudgingRound(id: string) {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "round", id],
    queryFn: () => judgingService.getRound(id),
    enabled: Boolean(id),
  });
}

export function useAdminLeaderboard(roundId: string) {
  return useQuery({
    queryKey: [...JUDGING_QUERY_KEY, "leaderboard", roundId],
    queryFn: () => judgingService.getAdminLeaderboard(roundId),
    enabled: Boolean(roundId),
  });
}

export function useCreateRound() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CreateRoundRequest) => judgingService.createRound(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "rounds"],
      });
    },
  });
}

export function useActivateRound() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => judgingService.activateRound(id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...JUDGING_QUERY_KEY, "rounds"],
      });
    },
  });
}

export function useCreatePanel() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CreatePanelRequest) => judgingService.createPanel(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useConfigureSlots() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      panelId,
      payload,
    }: {
      panelId: string;
      payload: ConfigureSlotsRequest;
    }) => judgingService.configureSlots(panelId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}
