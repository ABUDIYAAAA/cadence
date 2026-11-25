"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { buildScopedQueryKey, isScopeReady } from "@/lib/api-client";
import { useTenant } from "@/providers/tenant-provider";
import { judgingService } from "@/services/judging.service";
import {
  ConfigureSlotsRequest,
  CreatePanelRequest,
  CreateRoundRequest,
  ImportTeamCriteriaRequest,
  MoveSlotRequest,
  UpdateSlotTimingRequest,
  UnlockScorecardRequest,
  SetTeamCriterionRequest,
  SubmitScoresRequest,
  UpdateMySubmissionRequest,
} from "@/types/judging";

export const JUDGING_QUERY_KEY = ["judging"] as const;

// Judges on one panel see each other's actions within this window (paused while the tab is hidden).
const JUDGE_SYNC_INTERVAL_MS = 3000;

function useJudgingScope() {
  const { eventSlug, competitionSlug } = useTenant();
  const scope = { eventSlug, competitionSlug };
  const ready = isScopeReady("competition", scope);
  const key = (...parts: unknown[]) =>
    buildScopedQueryKey("judging", "competition", scope, ...parts);
  return { scope, ready, key };
}

// Participant
export function useTeamJudgingStatus(options?: { enabled?: boolean }) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("team-status"),
    queryFn: () => judgingService.getMyTeamStatus(),
    enabled: ready && (options?.enabled ?? true),
    refetchInterval: 1000 * 15, // poll queue progress every 15s
  });
}

// Volunteer
export function useVolunteerSchedule(options?: { enabled?: boolean }) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("volunteer-schedule"),
    queryFn: async () => (await judgingService.getVolunteerSchedule()) ?? [],
    enabled: ready && (options?.enabled ?? true),
    refetchInterval: 1000 * 10, // volunteer updates frequently
  });
}

// Judge
export function useJudgePanels(options?: { enabled?: boolean }) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("judge-panels"),
    queryFn: async () => (await judgingService.getJudgePanels()) ?? [],
    enabled: ready && (options?.enabled ?? true),
    // Slot start/complete by any judge on the panel moves current_slot for everyone.
    refetchInterval: JUDGE_SYNC_INTERVAL_MS,
  });
}

export function useJudgeSlotDetails(slotId: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("slot-details", slotId),
    queryFn: () => judgingService.getJudgeSlotDetails(slotId),
    enabled: ready && Boolean(slotId),
    // Clock, locks and slot status are shared by every judge on the panel.
    refetchInterval: JUDGE_SYNC_INTERVAL_MS,
  });
}

export function useStartSlot() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.startSlot(slotId),
    onSuccess: (_data, slotId) => {
      queryClient.invalidateQueries({
        queryKey: key("slot-details", slotId),
      });
      queryClient.invalidateQueries({
        queryKey: key("judge-panels"),
      });
    },
  });
}

function useTimerMutation<TVars>(
  fn: (vars: TVars) => Promise<unknown>,
  getSlotId: (vars: TVars) => string
) {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

  return useMutation({
    mutationFn: fn,
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({
        queryKey: key("slot-details", getSlotId(vars)),
      });
    },
  });
}

export function usePauseTimer() {
  return useTimerMutation(
    (slotId: string) => judgingService.pauseTimer(slotId),
    (slotId) => slotId
  );
}

export function useResumeTimer() {
  return useTimerMutation(
    (slotId: string) => judgingService.resumeTimer(slotId),
    (slotId) => slotId
  );
}

export function useExtendTimer() {
  return useTimerMutation(
    ({ slotId, seconds }: { slotId: string; seconds: number }) =>
      judgingService.extendTimer(slotId, seconds),
    ({ slotId }) => slotId
  );
}

export function useSubmitScores() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

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
        queryKey: key("slot-details", variables.slotId),
      });
    },
  });
}

export function useLockScores() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.lockScores(slotId),
    onSuccess: (_data, slotId) => {
      queryClient.invalidateQueries({
        queryKey: key("slot-details", slotId),
      });
      queryClient.invalidateQueries({
        queryKey: key("judge-panels"),
      });
    },
  });
}

export function useCompleteSlot() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.completeSlot(slotId),
    onSuccess: (_data, slotId) => {
      queryClient.invalidateQueries({
        queryKey: key("slot-details", slotId),
      });
      queryClient.invalidateQueries({
        queryKey: key("judge-panels"),
      });
    },
  });
}

// Rounds listing & inspection (Admin & Judge)
export function useJudgingRounds(options?: { enabled?: boolean }) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("rounds"),
    queryFn: async () => (await judgingService.listRounds()) ?? [],
    enabled: ready && (options?.enabled ?? true),
  });
}

export function useJudgingRound(id: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("round", id),
    queryFn: () => judgingService.getRound(id),
    enabled: ready && Boolean(id),
  });
}

export function useMySubmission(options?: { enabled?: boolean }) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("my-submission"),
    queryFn: () => judgingService.getMySubmission(),
    enabled: ready && (options?.enabled ?? true),
  });
}

export function useUpdateMySubmission() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();
  return useMutation({
    mutationFn: (payload: UpdateMySubmissionRequest) => judgingService.updateMySubmission(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: key("my-submission") });
      queryClient.invalidateQueries({ queryKey: key("team-status") });
    },
  });
}

export function useSubmissionGate(
  roundId?: string | null,
  options?: { enabled?: boolean }
) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("submission-gate", roundId ?? ""),
    queryFn: () => judgingService.getSubmissionGate(roundId),
    enabled: ready && (options?.enabled ?? true),
  });
}

export function useSubmissions(options?: { enabled?: boolean }) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("submissions"),
    queryFn: () => judgingService.listSubmissions(),
    enabled: ready && (options?.enabled ?? true),
  });
}

export function useSetSubmissionGate() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();
  return useMutation({
    mutationFn: (payload: { roundId?: string | null; sessionId: string | null }) =>
      judgingService.setSubmissionGate(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: key("submission-gate") });
      queryClient.invalidateQueries({ queryKey: key("team-status") });
      queryClient.invalidateQueries({ queryKey: key("my-submission") });
    },
  });
}

export function useAdminLeaderboard(roundId: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("leaderboard", roundId),
    queryFn: () => judgingService.getAdminLeaderboard(roundId),
    enabled: ready && Boolean(roundId),
    // Standings change whenever any judge locks a scorecard, and that happens in other browsers.
    refetchInterval: 1000 * 5,
  });
}

/** Every judge's mark on every criterion (including pending ones) for a round. */
export function useScoreSheet(roundId: string, enabled = true) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("score-sheet", roundId),
    queryFn: () => judgingService.getScoreSheet(roundId),
    enabled: ready && enabled && Boolean(roundId),
    refetchInterval: enabled ? 1000 * 10 : false,
  });
}

export function useRoundAnalytics(roundId: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("analytics", roundId),
    queryFn: () => judgingService.getRoundAnalytics(roundId),
    enabled: ready && Boolean(roundId),
    refetchInterval: 1000 * 10,
  });
}

export function useCreateRound() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

  return useMutation({
    mutationFn: (payload: CreateRoundRequest) => judgingService.createRound(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("rounds"),
      });
    },
  });
}

export function useActivateRound() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();

  return useMutation({
    mutationFn: (id: string) => judgingService.activateRound(id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("rounds"),
      });
    },
  });
}

export function useEndRound() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, force }: { id: string; force?: boolean }) =>
      judgingService.endRound(id, force),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useDeleteRound() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, force }: { id: string; force?: boolean }) =>
      judgingService.deleteRound(id, force),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useRoundPanels(roundId?: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("round-panels", roundId),
    queryFn: () => judgingService.getRoundPanels(roundId as string),
    enabled: ready && Boolean(roundId),
  });
}

export function useRoundTeams(roundId?: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("round-teams", roundId),
    queryFn: () => judgingService.getRoundTeams(roundId as string),
    enabled: ready && Boolean(roundId),
  });
}

export function useImportTeamCriteria() {
  const queryClient = useQueryClient();
  const { key } = useJudgingScope();
  return useMutation({
    mutationFn: ({ roundId, payload }: { roundId: string; payload: ImportTeamCriteriaRequest }) =>
      judgingService.importTeamCriteria(roundId, payload),
    onSuccess: (_data, { roundId }) => {
      queryClient.invalidateQueries({ queryKey: key("round-teams", roundId) });
      queryClient.invalidateQueries({ queryKey: key("round-panels", roundId) });
    },
  });
}

export function useAssignedJudges(roundId?: string) {
  const { ready, key } = useJudgingScope();
  return useQuery({
    queryKey: key("assigned-judges", roundId),
    queryFn: () => judgingService.getAssignedJudges(roundId as string),
    enabled: ready && Boolean(roundId),
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

export function useMoveSlot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      slotId,
      payload,
    }: {
      slotId: string;
      payload: MoveSlotRequest;
    }) => judgingService.moveSlot(slotId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useUpdateSlotTiming() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      slotId,
      payload,
    }: {
      slotId: string;
      payload: UpdateSlotTimingRequest;
    }) => judgingService.updateSlotTiming(slotId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useSetPanelJudges() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ panelId, judgeIds }: { panelId: string; judgeIds: string[] }) =>
      judgingService.setPanelJudges(panelId, judgeIds),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useUpdateRoundDuration() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      roundId,
      payload,
    }: {
      roundId: string;
      payload: { slot_duration_mins: number; reset_custom: boolean };
    }) => judgingService.updateRoundDuration(roundId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useSetTeamCriterion() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      roundId,
      teamId,
      payload,
    }: {
      roundId: string;
      teamId: string;
      payload: SetTeamCriterionRequest;
    }) => judgingService.setTeamCriterion(roundId, teamId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useDeleteTeamCriterion() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ roundId, teamId }: { roundId: string; teamId: string }) =>
      judgingService.deleteTeamCriterion(roundId, teamId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useUnlockScorecard() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ slotId, payload }: { slotId: string; payload?: UnlockScorecardRequest }) =>
      judgingService.unlockScorecard(slotId, payload),
    onSuccess: () => {
      // Unlocking changes the judge's card, the room's current team and the standings.
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useQueueSlot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.queueSlot(slotId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}

export function useUnqueueSlot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slotId: string) => judgingService.unqueueSlot(slotId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: JUDGING_QUERY_KEY });
    },
  });
}
