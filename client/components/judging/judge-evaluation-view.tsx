"use client";

import React, { useState, useEffect, useMemo } from "react";
import { apiErrorMessage } from "@/lib/utils";
import {
  Clock,
  CheckCircle2,
  Lock,
  Play,
  ArrowRight,
  ExternalLink,
  Copy,
  Check,
  Code2,
  Globe,
  FileText,
  AlertTriangle,
  RefreshCw,
  Save,
  Pause,
  Plus,
  Users,
} from "lucide-react";
import {
  useJudgePanels,
  useJudgeSlotDetails,
  useStartSlot,
  usePauseTimer,
  useResumeTimer,
  useExtendTimer,
  useSubmitScores,
  useLockScores,
  useCompleteSlot,
} from "@/hooks/use-judging";
import { useToast } from "@/providers/toast-provider";
import {
  JudgePanelSummary,
  JudgeSlotDetails,
  CriterionScoreInput,
} from "@/types/judging";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog } from "@/components/ui/dialog";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function JudgeEvaluationView() {
  const { data: rawPanels, isLoading: panelsLoading, refetch: refetchPanels, isFetching } = useJudgePanels();
  const panels = useMemo(() => (Array.isArray(rawPanels) ? rawPanels : []), [rawPanels]);

  // Active panel selection (user override or first panel)
  const [userSelectedPanelId, setUserSelectedPanelId] = useState<string | null>(null);
  const activePanelId = userSelectedPanelId || (panels.length > 0 ? panels[0].panel_id : null);

  const activePanel: JudgePanelSummary | undefined = useMemo(() => {
    if (panels.length === 0) return undefined;
    return panels.find((p) => p.panel_id === activePanelId) || panels[0];
  }, [panels, activePanelId]);

  // Active slot selection (user override or current presenting/next slot)
  const [userSelectedSlotId, setUserSelectedSlotId] = useState<string | null>(null);

  const activeSlotId = useMemo(() => {
    if (!activePanel || !Array.isArray(activePanel.slots)) return null;
    if (userSelectedSlotId && activePanel.slots.some((s) => s.slot_id === userSelectedSlotId)) {
      return userSelectedSlotId;
    }
    if (activePanel.current_slot) {
      return activePanel.current_slot.slot_id;
    }
    const nextInLine = activePanel.slots.find((s) => s.status !== "completed");
    return nextInLine ? nextInLine.slot_id : activePanel.slots[0]?.slot_id || null;
  }, [userSelectedSlotId, activePanel]);

  if (panelsLoading) {
    return (
      <div className="font-sans space-y-4">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (panels.length === 0) {
    return (
      <div className="font-sans space-y-4">
        <EmptyState
          title="No judging panels assigned"
          description="You are currently not assigned to evaluate any rooms or panels. Event administrators allocate evaluators to rooms during round setup."
          action={
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetchPanels()}
              className="mt-4 text-xs"
            >
              <RefreshCw className="h-3 w-3 mr-1.5" />
              Check for assignments
            </Button>
          }
        />
      </div>
    );
  }

  return (
    <div className="font-sans space-y-6">
      <SectionToolbar
        title="Scorecards"
        subtitle={activePanel ? `${activePanel.round_name} · ${activePanel.room} — ${activePanel.panel_name}` : undefined}
        onRefresh={() => refetchPanels()}
        isFetching={isFetching}
      />

      {/* Multi-Panel Tabs if assigned to >1 panel */}
      {panels.length > 1 && (
        <div className="flex items-center gap-2 overflow-x-auto">
          <span className="text-xs font-medium text-[var(--ink-secondary)] shrink-0">Your panels</span>
          {panels.map((p) => {
            const isActive = p.panel_id === activePanel?.panel_id;
            return (
              <button
                key={p.panel_id}
                onClick={() => {
                  setUserSelectedPanelId(p.panel_id);
                  setUserSelectedSlotId(null);
                }}
                className={`px-3.5 py-1.5 text-xs font-semibold rounded-full transition-colors cursor-pointer ${
                  isActive
                    ? "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-sm"
                    : "bg-[var(--card-solid)] text-[var(--ink-secondary)] border border-[var(--card-line)] hover:text-[var(--ink-primary)]"
                }`}
              >
                {p.panel_name} ({p.room})
              </button>
            );
          })}
        </div>
      )}

      {activePanel && (
        <MetricGrid>
          <Metric label="Room" value={activePanel.room} />
          <Metric
            label="Teams evaluated"
            value={activePanel.completed_slots}
            suffix={`/ ${activePanel.total_slots}`}
            valueClassName="text-[var(--color-blue)]"
          />
          <Metric label="Slot duration" value={`${activePanel.slot_duration_mins}m`} />
          <Metric label="Round" value={activePanel.round_name} note={activePanel.round_status === "completed" ? "Finished" : "In progress"} />
        </MetricGrid>
      )}

      {/* Main Evaluator Work Area: Sidebar Slots & Active Scorecard Center */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        {/* Left Column (4 cols): Queue / Slot Schedule */}
        <div className="lg:col-span-4 rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 space-y-3">
          <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-3">
            <span className="text-xs font-semibold text-[var(--ink-primary)]">
              Pitch schedule ({activePanel?.slots.length || 0})
            </span>
            <span className="text-[11px] text-[var(--ink-tertiary)] font-mono">
              {activePanel?.completed_slots || 0} done
            </span>
          </div>

          <div className="space-y-1.5 max-h-[720px] overflow-y-auto pr-0.5">
            {activePanel?.slots.map((s, idx) => {
              const isSelected = s.slot_id === activeSlotId;
              const isCurrent = s.status === "in_progress";
              const isDone = s.status === "completed";
              const isQueued = s.status === "queued";

              return (
                <div
                  key={s.slot_id}
                  onClick={() => setUserSelectedSlotId(s.slot_id)}
                  className={`rounded-[12px] p-3 cursor-pointer transition-all border ${
                    isSelected
                      ? "border-[var(--color-blue)] bg-[var(--color-blue-dim)] shadow-xs"
                      : "border-[var(--card-line)] bg-[var(--card-solid)] hover:border-[var(--card-line-hover)] hover:bg-[var(--card-solid-subtle)]"
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2.5 truncate">
                      <span className={`w-5 h-5 rounded-[6px] text-[10px] font-bold flex items-center justify-center shrink-0 ${
                        isSelected
                          ? "bg-[var(--color-blue)] text-white"
                          : "bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] border border-[var(--card-line)]"
                      }`}>
                        {idx + 1}
                      </span>
                      <div className="font-semibold text-xs truncate text-[var(--ink-primary)]">
                        {s.team_name}
                      </div>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0">
                      {s.is_my_scorecard_locked && (
                        <span title="Your scorecard is locked">
                          <Lock className="h-3 w-3 text-[var(--color-emerald)]" />
                        </span>
                      )}
                      {isCurrent && (
                        <span className="inline-block w-2 h-2 rounded-full bg-[var(--color-coral)] animate-ping" />
                      )}
                    </div>
                  </div>

                  <div className="flex items-center justify-between mt-2 pt-2 border-t border-[var(--card-line)]/60 text-[10px]">
                    <span>
                      {isCurrent ? (
                        <strong className="text-[var(--color-coral)] font-semibold">Presenting now</strong>
                      ) : isDone ? (
                        <span className="text-[var(--color-emerald)] font-medium">Completed</span>
                      ) : isQueued ? (
                        <strong
                          className="text-[var(--color-blue)] font-semibold"
                          title="Volunteers have been asked to bring this team to the room"
                        >
                          On deck
                        </strong>
                      ) : (
                        <span className="text-[var(--ink-tertiary)]">Scheduled</span>
                      )}
                    </span>

                    {s.is_my_scorecard_locked ? (
                      <span className="text-[var(--color-emerald)] font-medium">Locked</span>
                    ) : (
                      <span className="text-[var(--ink-tertiary)]">Ungraded</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right Column (8 cols): Scorecard, Countdown Timer, Submissions */}
        <div className="lg:col-span-8">
          {activeSlotId ? (
            <JudgeSlotScorecardContainer
              slotId={activeSlotId}
              panel={activePanel}
              onSlotAdvanced={(nextSlotId) => {
                refetchPanels();
                if (nextSlotId) setUserSelectedSlotId(nextSlotId);
              }}
            />
          ) : (
            <EmptyState
              title="No slot selected"
              description="Select a team slot from the pitch schedule on the left to evaluate their submission."
            />
          )}
        </div>
      </div>
    </div>
  );
}

// Wrapper to fetch slot data and key the form by slotId
function JudgeSlotScorecardContainer({
  slotId,
  panel,
  onSlotAdvanced,
}: {
  slotId: string;
  panel?: JudgePanelSummary;
  onSlotAdvanced: (nextSlotId?: string) => void;
}) {
  const { data: slot, isLoading, refetch, dataUpdatedAt } = useJudgeSlotDetails(slotId);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (!slot) {
    return (
      <EmptyState
        title="Slot data not found"
        description="Could not load the evaluation details for this slot."
      />
    );
  }

  return (
    <JudgeSlotScorecardForm
      key={slot.slot_id}
      slot={slot}
      receivedAt={dataUpdatedAt}
      panel={panel}
      onRefetch={refetch}
      onSlotAdvanced={onSlotAdvanced}
    />
  );
}

// Subcomponent: Form state initialized directly from slot props
function JudgeSlotScorecardForm({
  slot,
  receivedAt,
  panel,
  onRefetch,
  onSlotAdvanced,
}: {
  slot: JudgeSlotDetails;
  receivedAt: number;
  panel?: JudgePanelSummary;
  onRefetch: () => void;
  onSlotAdvanced: (nextSlotId?: string) => void;
}) {
  const toast = useToast();
  const startSlotMutation = useStartSlot();
  const pauseTimerMutation = usePauseTimer();
  const resumeTimerMutation = useResumeTimer();
  const extendTimerMutation = useExtendTimer();
  const submitScoresMutation = useSubmitScores();
  const lockScoresMutation = useLockScores();
  const completeSlotMutation = useCompleteSlot();

  // Local scores state initialized directly without useEffect
  const [formScores, setFormScores] = useState<Record<string, { score: number; notes: string }>>(() => {
    const initial: Record<string, { score: number; notes: string }> = {};
    slot.criteria_scores.forEach((c) => {
      initial[c.criterion_id] = {
        score: c.score !== undefined && c.score !== null ? c.score : 0,
        notes: c.notes || "",
      };
    });
    return initial;
  });

  // Criteria the judge has deliberately scored (saved earlier, or set this session).
  // Locking requires every criterion, so an untouched default of 0 doesn't count.
  const [scoredIds, setScoredIds] = useState<Set<string>>(
    () => new Set(slot.criteria_scores.filter((c) => c.score != null).map((c) => c.criterion_id))
  );
  const scoredCount = slot.criteria_scores.filter((c) => scoredIds.has(c.criterion_id)).length;
  const criteriaCount = slot.criteria_scores.length;
  const allCriteriaScored = scoredCount === criteriaCount;

  const [isLockConfirmOpen, setIsLockConfirmOpen] = useState(false);
  const [copiedLinkKey, setCopiedLinkKey] = useState<string | null>(null);

  // Handle score value change
  const handleScoreChange = (criterionId: string, val: number) => {
    setScoredIds((prev) => (prev.has(criterionId) ? prev : new Set(prev).add(criterionId)));
    setFormScores((prev) => ({
      ...prev,
      [criterionId]: {
        ...prev[criterionId],
        score: val,
      },
    }));
  };

  // Handle score note change
  const handleNoteChange = (criterionId: string, notes: string) => {
    setFormScores((prev) => ({
      ...prev,
      [criterionId]: {
        ...prev[criterionId],
        notes,
      },
    }));
  };

  // Compute live weighted total score
  const weightedTotal = useMemo(() => {
    let sum = 0;
    slot.criteria_scores.forEach((c) => {
      const current = formScores[c.criterion_id]?.score ?? 0;
      if (c.max_score > 0) {
        sum += (current / c.max_score) * c.weight;
      }
    });
    return Math.round(sum * 100) / 100;
  }, [slot, formScores]);

  // Total possible weight
  const totalWeight = useMemo(() => {
    return slot.criteria_scores.reduce((acc, c) => acc + c.weight, 0);
  }, [slot]);

  // 1. Action: Start Slot Pitch
  const handleStartSlot = async () => {
    try {
      await startSlotMutation.mutateAsync(slot.slot_id);
      toast.success(`Pitch started for team "${slot.team_name}"! Timer is running.`);
      onRefetch();
    } catch (err: unknown) {
      const message =
        apiErrorMessage(err, "Failed to start pitch");
      toast.error(message);
    }
  };

  // Timer controls: pause / resume / add time
  const runTimerAction = async (action: () => Promise<unknown>, success: string) => {
    try {
      await action();
      toast.success(success);
      onRefetch();
    } catch (err: unknown) {
      const message =
        apiErrorMessage(err, "Timer action failed");
      toast.error(message);
    }
  };

  const handlePauseTimer = () =>
    runTimerAction(() => pauseTimerMutation.mutateAsync(slot.slot_id), "Timer paused.");
  const handleResumeTimer = () =>
    runTimerAction(() => resumeTimerMutation.mutateAsync(slot.slot_id), "Timer resumed.");
  const handleExtendTimer = (seconds: number) =>
    runTimerAction(
      () => extendTimerMutation.mutateAsync({ slotId: slot.slot_id, seconds }),
      `Added ${seconds / 60} min to the pitch.`
    );

  // 2. Action: Save Draft Scores
  const handleSaveDraft = async () => {
    const scoresPayload: CriterionScoreInput[] = Object.entries(formScores).map(
      ([criterion_id, val]) => ({
        criterion_id,
        score: val.score,
        notes: val.notes ? val.notes : undefined,
      })
    );

    try {
      await submitScoresMutation.mutateAsync({
        slotId: slot.slot_id,
        payload: { scores: scoresPayload },
      });
      toast.success("Scores saved as draft.");
      onRefetch();
    } catch (err: unknown) {
      const message =
        apiErrorMessage(err, "Failed to save scores");
      toast.error(message);
    }
  };

  // 3. Action: Lock Scores
  const handleConfirmLock = async () => {
    try {
      // First save current values
      const scoresPayload: CriterionScoreInput[] = Object.entries(formScores).map(
        ([criterion_id, val]) => ({
          criterion_id,
          score: val.score,
          notes: val.notes ? val.notes : undefined,
        })
      );
      await submitScoresMutation.mutateAsync({
        slotId: slot.slot_id,
        payload: { scores: scoresPayload },
      });

      // Then lock
      await lockScoresMutation.mutateAsync(slot.slot_id);
      toast.success(`Scorecard locked for "${slot.team_name}". The pitch has ended.`);
      setIsLockConfirmOpen(false);
      onRefetch();
    } catch (err: unknown) {
      const message =
        apiErrorMessage(err, "Failed to lock scores");
      toast.error(message);
    }
  };

  // 4. Action: Call Next Team / Complete Slot
  const handleCompleteAndNext = async () => {
    try {
      await completeSlotMutation.mutateAsync(slot.slot_id);
      toast.success(`Slot for "${slot.team_name}" completed! Advancing to next team.`);

      // Find the next sequential slot in this panel
      if (panel) {
        const currentIndex = panel.slots.findIndex((s) => s.slot_id === slot.slot_id);
        const nextSlot = panel.slots[currentIndex + 1];
        onSlotAdvanced(nextSlot ? nextSlot.slot_id : undefined);
      } else {
        onSlotAdvanced();
      }
    } catch (err: unknown) {
      const message =
        apiErrorMessage(err, "Failed to advance slot");
      toast.error(message);
    }
  };

  const handleCopyLink = (key: string, url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedLinkKey(key);
    setTimeout(() => setCopiedLinkKey(null), 2000);
    toast.info(`Copied ${key} URL to clipboard`);
  };

  const isLocked = slot.is_my_scorecard_locked;
  const isPresenting = slot.status === "in_progress";
  const isCompleted = slot.status === "completed";
  const canStart = slot.status === "scheduled" || slot.status === "queued";
  // Scores can only be entered while the pitch is running (the server enforces this too).
  const canScore = isPresenting && !isLocked;

  return (
    <div className="space-y-5 font-sans">
      {/* Top Slot Header */}
      <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2.5">
              <span className="font-bold text-lg tracking-tight text-[var(--ink-primary)]">
                {slot.team_name}
              </span>
              <Badge variant="subtle" size="sm" className="font-mono">
                Slot #{slot.sequence_order}
              </Badge>
            </div>
            <div className="text-xs text-[var(--ink-secondary)] mt-1">
              Room: <strong className="text-[var(--ink-primary)]">{slot.room}</strong> · Panel: <strong className="text-[var(--ink-primary)]">{slot.panel_name}</strong>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {isPresenting && (
              <Badge variant="coral" dot size="sm" className="font-semibold animate-pulse">
                Presenting now
              </Badge>
            )}
            {isCompleted && (
              <Badge variant="emerald" size="sm" className="font-semibold">
                Evaluation completed
              </Badge>
            )}
            {canStart && (
              <Button
                variant="primary"
                size="sm"
                onClick={handleStartSlot}
                disabled={startSlotMutation.isPending}
                className="text-xs gap-1.5 h-9 px-4 font-semibold active:scale-[0.97] transition-transform"
              >
                <Play className="h-3.5 w-3.5 fill-current" />
                <span>Start pitch</span>
              </Button>
            )}
          </div>
        </div>

        {/* Live Pitch Countdown Timer Component */}
        <JudgeCountdownTimer
          slot={slot}
          receivedAt={receivedAt}
          onPause={handlePauseTimer}
          onResume={handleResumeTimer}
          onExtend={handleExtendTimer}
          isBusy={
            pauseTimerMutation.isPending ||
            resumeTimerMutation.isPending ||
            extendTimerMutation.isPending
          }
        />
      </div>

      {/* Submitted Repositories & Artifacts Section */}
      <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 space-y-3">
        <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-3">
          <div className="flex items-center gap-2">
            <Globe className="h-4 w-4 text-[var(--color-blue)]" />
            <h3 className="text-xs font-semibold text-[var(--ink-primary)]">
              Submission materials
            </h3>
          </div>
          <span className="text-[11px] text-[var(--ink-tertiary)]">
            {Object.keys(slot.submission_links || {}).length} link(s)
          </span>
        </div>

        {Object.keys(slot.submission_links || {}).length === 0 ? (
          <div className="text-xs text-[var(--ink-tertiary)] italic p-4 border border-dashed border-[var(--card-line)] rounded-[12px] text-center">
            No repository or presentation links submitted yet by this squad.
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            {Object.entries(slot.submission_links).map(([key, url]) => {
              const isGithub = key.toLowerCase().includes("git") || url.includes("github.com");
              const isSlides = key.toLowerCase().includes("slide") || url.includes("presentation");

              return (
                <div
                  key={key}
                  className="rounded-[12px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)] flex items-center justify-between gap-3 text-xs"
                >
                  <div className="flex items-center gap-2.5 truncate">
                    {isGithub ? (
                      <Code2 className="h-4 w-4 shrink-0 text-[var(--color-blue)]" />
                    ) : isSlides ? (
                      <FileText className="h-4 w-4 shrink-0 text-[var(--color-blue)]" />
                    ) : (
                      <Globe className="h-4 w-4 shrink-0 text-[var(--color-blue)]" />
                    )}
                    <div className="truncate">
                      <div className="font-semibold capitalize text-[11px] text-[var(--ink-primary)]">{key}</div>
                      <a
                        href={url}
                        target="_blank"
                        rel="noreferrer"
                        className="truncate text-[var(--ink-secondary)] hover:text-[var(--color-blue)] hover:underline block font-mono text-[11px]"
                      >
                        {url}
                      </a>
                    </div>
                  </div>

                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => handleCopyLink(key, url)}
                      title="Copy URL"
                      className="p-1.5 rounded-[8px] border border-[var(--card-line)] hover:border-[var(--color-blue)] bg-[var(--card-solid)] transition-colors cursor-pointer"
                    >
                      {copiedLinkKey === key ? (
                        <Check className="h-3 w-3 text-[var(--color-emerald)]" />
                      ) : (
                        <Copy className="h-3 w-3 text-[var(--ink-secondary)]" />
                      )}
                    </button>
                    <a
                      href={url}
                      target="_blank"
                      rel="noreferrer"
                      title="Open in new tab"
                      className="p-1.5 rounded-[8px] border border-[var(--card-line)] hover:border-[var(--color-blue)] bg-[var(--card-solid)] text-[var(--ink-secondary)] transition-colors"
                    >
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Evaluation Rubrics & Scorecard */}
      <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 border-b border-[var(--card-line)] pb-3">
          <div>
            <h3 className="text-sm font-semibold text-[var(--ink-primary)]">
              Rubrics evaluation
            </h3>
            <p className="text-xs text-[var(--ink-secondary)] mt-0.5">
              Grade each criterion up to its maximum score. Total rating calculates automatically.
            </p>
          </div>

          <div className="text-right rounded-[12px] border border-[var(--card-line)] p-2.5 px-3.5 bg-[var(--card-solid-subtle)] shrink-0">
            <div className="text-[11px] font-medium text-[var(--ink-tertiary)]">Scorecard rating</div>
            <div className="text-xl font-bold text-[var(--ink-primary)] font-mono">
              {weightedTotal} <span className="text-xs text-[var(--ink-tertiary)] font-normal">/ {totalWeight} pts</span>
            </div>
          </div>
        </div>

        {canStart && (
          <div
            role="status"
            className="flex items-start gap-2.5 p-3 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-xs text-[var(--ink-secondary)]"
          >
            <Lock className="h-4 w-4 shrink-0 mt-0.5" />
            <p>Scoring opens when you start the pitch. Starting it also starts the timer.</p>
          </div>
        )}

        {/* Criteria Rating Rows */}
        <div className="space-y-3">
          {slot.criteria_scores.map((c, idx) => {
            const currentScore = formScores[c.criterion_id]?.score ?? 0;
            const currentNotes = formScores[c.criterion_id]?.notes ?? "";
            const weightedPoints =
              c.max_score > 0 ? Math.round(((currentScore / c.max_score) * c.weight) * 100) / 100 : 0;

            return (
              <div
                key={c.criterion_id}
                className={`rounded-[14px] border p-4 transition-all ${
                  isLocked
                    ? "border-[var(--card-line)] bg-[var(--card-solid-subtle)]/60"
                    : "border-[var(--card-line)] bg-[var(--card-solid-subtle)]"
                }`}
              >
                <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 pb-2.5 border-b border-[var(--card-line)]">
                  <div className="flex items-center gap-2">
                    <span className="w-5 h-5 rounded-full bg-[var(--color-dark)] text-[var(--ink-inverse)] text-[10px] font-bold flex items-center justify-center shrink-0">
                      {idx + 1}
                    </span>
                    <span className="font-semibold text-xs text-[var(--ink-primary)]">
                      {c.criterion_name}
                    </span>
                  </div>

                  <div className="flex items-center gap-2.5 text-xs font-mono">
                    <Badge variant="subtle" size="sm">
                      Weight: {c.weight}%
                    </Badge>
                    <span className="text-[var(--ink-secondary)]">
                      Contribution: <strong className="text-[var(--ink-primary)]">{weightedPoints} pts</strong>
                    </span>
                  </div>
                </div>

                {/* Score Controls */}
                <div className="grid grid-cols-1 sm:grid-cols-12 gap-4 mt-3">
                  <div className="sm:col-span-5 space-y-2">
                    <div className="flex items-center justify-between">
                      <label className="text-xs font-medium text-[var(--ink-secondary)]">
                        Score (0 to {c.max_score})
                      </label>
                      <span className="text-xs font-bold font-mono text-[var(--ink-primary)]">
                        {currentScore} / {c.max_score}
                      </span>
                    </div>

                    {!isLocked ? (
                      <div className="space-y-2">
                        {/* Interactive Range Slider */}
                        <input
                          type="range"
                          min={0}
                          max={c.max_score}
                          step={1}
                          value={currentScore}
                          onChange={(e) => handleScoreChange(c.criterion_id, Number(e.target.value))}
                          disabled={!canScore}
                          className="w-full accent-[var(--color-blue)] cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
                        />

                        {/* Quick Stepper Chips */}
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {[0, Math.round(c.max_score * 0.25), Math.round(c.max_score * 0.5), Math.round(c.max_score * 0.75), c.max_score].map(
                            (presetVal) => (
                              <button
                                key={presetVal}
                                type="button"
                                onClick={() => handleScoreChange(c.criterion_id, presetVal)}
                                disabled={!canScore}
                                className={`disabled:cursor-not-allowed disabled:opacity-50 px-2 py-0.5 rounded-[8px] text-xs font-mono font-semibold border transition-all cursor-pointer active:scale-95 ${
                                  currentScore === presetVal
                                    ? "bg-[var(--color-dark)] text-[var(--ink-inverse)] border-[var(--color-dark)] shadow-xs"
                                    : "bg-[var(--card-solid)] text-[var(--ink-secondary)] border-[var(--card-line)] hover:text-[var(--ink-primary)]"
                                }`}
                              >
                                {presetVal}
                              </button>
                            )
                          )}
                        </div>
                      </div>
                    ) : (
                      <div className="p-2 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] font-mono text-xs font-bold text-[var(--ink-primary)]">
                        {currentScore} pts (Locked)
                      </div>
                    )}
                  </div>

                  <div className="sm:col-span-7 space-y-1">
                    <label className="text-xs font-medium text-[var(--ink-secondary)]">
                      Notes & feedback
                    </label>
                    <textarea
                      rows={2}
                      disabled={!canScore}
                      placeholder="Add observations or feedback..."
                      value={currentNotes}
                      onChange={(e) => handleNoteChange(c.criterion_id, e.target.value)}
                      className="w-full p-2.5 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] text-xs resize-none focus:outline-none focus:border-[var(--color-blue)] focus:ring-1 focus:ring-[var(--color-blue)] disabled:text-[var(--ink-tertiary)] transition-all text-[var(--ink-primary)]"
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <PanelScoringProgress slot={slot} />

        {/* Action Controls Footer: Save Draft / Lock / Call Next */}
        <div className="border-t border-[var(--card-line)] pt-3.5 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div>
            {isLocked ? (
              <div className="flex items-center gap-2 text-xs font-semibold text-[var(--color-emerald)]">
                <CheckCircle2 className="h-4 w-4" />
                <span>Your scorecard is locked</span>
                <span className="text-[var(--ink-tertiary)] font-normal">
                  {slot.all_panel_judges_locked
                    ? "(all panel judges locked)"
                    : `(${slot.locked_judges_count} of ${slot.total_judges_count} judges locked)`}
                </span>
              </div>
            ) : (
              <div className="text-xs text-[var(--ink-tertiary)]" aria-live="polite">
                {canScore && !allCriteriaScored
                  ? `Score every criterion to lock in (${scoredCount} of ${criteriaCount} scored).`
                  : "Draft ratings are saved and private to you until locked."}
              </div>
            )}
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {!isLocked && (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleSaveDraft}
                  disabled={!canScore || submitScoresMutation.isPending}
                  className="text-xs flex items-center gap-1.5 h-9 active:scale-[0.97] transition-transform"
                >
                  <Save className="h-3.5 w-3.5" />
                  <span>Save draft</span>
                </Button>

                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => setIsLockConfirmOpen(true)}
                  disabled={!canScore || !allCriteriaScored || lockScoresMutation.isPending}
                  className="text-xs flex items-center gap-1.5 h-9 font-semibold active:scale-[0.97] transition-transform"
                >
                  <Lock className="h-3.5 w-3.5" />
                  <span>Lock in scores</span>
                </Button>
              </>
            )}

            {/* CALL NEXT TEAM BUTTON */}
            {isLocked && !isCompleted && (
              <Button
                variant="primary"
                size="sm"
                onClick={handleCompleteAndNext}
                disabled={!slot.all_panel_judges_locked || completeSlotMutation.isPending}
                title={slot.all_panel_judges_locked ? undefined : "Waiting for all panel judges to lock their scores"}
                className="text-xs flex items-center gap-1.5 h-9 font-semibold active:scale-[0.97] transition-transform"
              >
                <span>Call next team</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Lock Confirmation Dialog */}
      <Dialog
        open={isLockConfirmOpen}
        onClose={() => setIsLockConfirmOpen(false)}
        title="Submit scorecard ratings"
        description="Finalize this evaluation and submit ratings to the leaderboard."
      >
        <div className="space-y-4 pt-2 font-sans">
          <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] space-y-1">
            <div className="text-sm font-semibold text-[var(--ink-primary)]">{slot.team_name}</div>
            <div className="text-xs text-[var(--ink-secondary)]">
              Total Score: <strong className="text-[var(--ink-primary)] font-mono">{weightedTotal} / {totalWeight} pts</strong>
            </div>
          </div>

          <div className="flex items-start gap-2.5 p-3 rounded-[12px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] text-xs">
            <AlertTriangle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />
            <p className="text-[var(--color-coral)] leading-relaxed">
              Once locked, your scores cannot be modified and the pitch timer stops for the whole panel. They will immediately contribute to the final leaderboard rankings.
            </p>
          </div>

          <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsLockConfirmOpen(false)}
              disabled={lockScoresMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={handleConfirmLock}
              disabled={lockScoresMutation.isPending}
            >
              {lockScoresMutation.isPending ? "Locking..." : "Confirm & lock scores"}
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}

// Panel scoring progress: who has locked, and what's blocking the next pitch
function PanelScoringProgress({ slot }: { slot: JudgeSlotDetails }) {
  const total = slot.total_judges_count;
  const locked = slot.locked_judges_count;
  const remaining = total - locked;
  const allLocked = slot.all_panel_judges_locked;
  const progress = total > 0 ? locked / total : 0;

  if (slot.status === "completed" || total === 0) return null;

  return (
    <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5 space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-xs font-semibold text-[var(--ink-primary)]">
          <Users className="h-4 w-4 text-[var(--ink-secondary)]" />
          <span>Panel scoring progress</span>
        </div>
        <span className="font-mono tabular-nums text-xs text-[var(--ink-secondary)]">
          <strong className="text-[var(--ink-primary)]">{locked}</strong> of {total} locked
        </span>
      </div>

      <div
        role="progressbar"
        aria-label="Judges who have locked their scores"
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={locked}
        className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--card-line)]"
      >
        <div
          className={`h-full w-full origin-left rounded-full transition-transform duration-300 ease-[var(--ease-out)] ${
            allLocked ? "bg-[var(--color-emerald)]" : "bg-[var(--color-blue)]"
          }`}
          style={{ transform: `scaleX(${progress})` }}
        />
      </div>

      <ul className="flex flex-wrap gap-1.5">
        {slot.panel_judges.map((j) => (
          <li
            key={j.judge_id}
            className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-medium ${
              j.is_locked
                ? "border-[var(--color-emerald)]/30 text-[var(--color-emerald)]"
                : "border-[var(--card-line)] text-[var(--ink-secondary)]"
            }`}
          >
            {j.is_locked ? <Lock className="h-3 w-3" /> : <Clock className="h-3 w-3" />}
            <span>{j.name}</span>
            <span className="sr-only">{j.is_locked ? "locked" : "still scoring"}</span>
          </li>
        ))}
      </ul>

      <p className="text-xs text-[var(--ink-secondary)]" aria-live="polite">
        {allLocked
          ? "All judges have locked. You can move on to the next pitch."
          : `Next pitch unlocks when all judges lock — waiting on ${remaining} more ${remaining === 1 ? "judge" : "judges"}.`}
      </p>
    </div>
  );
}

// Live Countdown Timer Subcomponent
const EXTEND_OPTIONS = [60, 120, 300];

function formatClock(totalSecs: number) {
  const abs = Math.abs(totalSecs);
  return `${String(Math.floor(abs / 60)).padStart(2, "0")}:${String(abs % 60).padStart(2, "0")}`;
}

function JudgeCountdownTimer({
  slot,
  receivedAt,
  onPause,
  onResume,
  onExtend,
  isBusy,
}: {
  slot: JudgeSlotDetails;
  /** When `slot` was fetched (ms epoch); the countdown runs from here. */
  receivedAt: number;
  onPause: () => void;
  onResume: () => void;
  onExtend: (seconds: number) => void;
  isBusy: boolean;
}) {
  const status = slot.status;
  const isRunning = status === "in_progress";
  const isPaused = isRunning && slot.is_paused;
  const baseSeconds = slot.slot_duration_seconds || slot.slot_duration_mins * 60;
  const totalSeconds = baseSeconds + (slot.extra_seconds || 0);

  // The server computes remaining_seconds at response time, so counting down from when the
  // response arrived needs no client/server clock comparison.
  const [now, setNow] = useState(() => Date.now());
  const isFrozen = slot.is_paused || slot.pitch_ended;

  useEffect(() => {
    if (!isRunning || isFrozen) return;
    const interval = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(interval);
  }, [isRunning, isFrozen]);

  let secondsRemaining: number | null = null;
  if (isRunning && slot.remaining_seconds != null) {
    const sinceReceived = isFrozen ? 0 : Math.max(0, Math.floor((now - receivedAt) / 1000));
    secondsRemaining = slot.remaining_seconds - sinceReceived;
  }

  if (!isRunning || secondsRemaining === null) {
    return (
      <div className="rounded-[12px] border border-[var(--card-line)] p-3 px-4 bg-[var(--card-solid-subtle)] flex items-center justify-between text-xs text-[var(--ink-secondary)]">
        <span className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-[var(--ink-tertiary)]" />
          Allocated pitch duration: <strong className="text-[var(--ink-primary)] font-mono">{slot.slot_duration_mins} minutes</strong>
        </span>
        <span className="text-[var(--ink-tertiary)]">
          {status === "completed" ? "Timer stopped" : "Timer ready (click start to begin)"}
        </span>
      </div>
    );
  }

  const isOvertime = secondsRemaining < 0;
  const elapsed = Math.max(0, totalSeconds - secondsRemaining);
  const progress = Math.max(0, Math.min(1, elapsed / totalSeconds));
  const extraMins = Math.round((slot.extra_seconds || 0) / 60);

  const isEnded = slot.pitch_ended;
  const tone = isPaused || isEnded
    ? "border border-[var(--card-line)] bg-[var(--card-solid-subtle)]"
    : isOvertime
    ? "border border-[#CE6969]/40 bg-[var(--color-coral-dim)]"
    : secondsRemaining <= 120
    ? "border border-[var(--color-coral-border)] bg-[var(--color-coral-dim)]/60"
    : "border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)]";

  const controlBtn =
    "inline-flex h-9 items-center gap-1.5 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 text-xs font-semibold text-[var(--ink-primary)] transition-[transform,background-color] duration-150 ease-[var(--ease-out)] hover:bg-[var(--card-solid-subtle)] active:scale-[0.97] disabled:opacity-50 disabled:pointer-events-none cursor-pointer";

  return (
    <div className={`rounded-[14px] p-4 space-y-3.5 transition-colors duration-200 ${tone}`}>
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          {isEnded ? (
            <CheckCircle2 className="h-5 w-5 text-[var(--color-emerald)]" />
          ) : isPaused ? (
            <Pause className="h-5 w-5 text-[var(--ink-secondary)]" />
          ) : (
            <Clock className={`h-5 w-5 ${isOvertime ? "text-[var(--color-coral)] animate-pulse" : "text-[var(--color-blue)]"}`} />
          )}
          <div>
            <div className="text-[11px] font-medium text-[var(--ink-secondary)]">
              {isEnded ? "Pitch ended" : isPaused ? "Timer paused" : isOvertime ? "Overtime warning" : "Pitch countdown"}
            </div>
            <div className="text-2xl font-bold tracking-tight font-mono tabular-nums text-[var(--ink-primary)]">
              {isOvertime
                ? `+${formatClock(secondsRemaining)} overtime`
                : `${formatClock(secondsRemaining)} ${isEnded ? "left" : "remaining"}`}
            </div>
          </div>
        </div>

        <div className="text-right text-xs text-[var(--ink-secondary)] w-full sm:w-auto">
          <div className="font-mono tabular-nums">
            Elapsed: <strong className="text-[var(--ink-primary)]">{formatClock(elapsed)}</strong> / {formatClock(totalSeconds)}
            {extraMins > 0 && <span className="text-[var(--ink-tertiary)]"> (+{extraMins}m added)</span>}
          </div>
          <div className="w-full sm:w-36 bg-[var(--card-line)] h-2 rounded-full mt-1.5 sm:ml-auto overflow-hidden">
            <div
              className={`h-full w-full origin-left rounded-full transition-transform duration-1000 ease-linear ${isOvertime ? "bg-[var(--color-coral)]" : "bg-[var(--color-blue)]"}`}
              style={{ transform: `scaleX(${progress})` }}
            />
          </div>
        </div>
      </div>

      {!isEnded && (
      <div className="flex items-center gap-2 flex-wrap border-t border-[var(--card-line)] pt-3">
        {isPaused ? (
          <button type="button" onClick={onResume} disabled={isBusy} className={controlBtn}>
            <Play className="h-3.5 w-3.5 fill-current" />
            <span>Resume</span>
          </button>
        ) : (
          <button type="button" onClick={onPause} disabled={isBusy} className={controlBtn}>
            <Pause className="h-3.5 w-3.5 fill-current" />
            <span>Pause</span>
          </button>
        )}
        <span className="mx-1 h-5 w-px bg-[var(--card-line)]" aria-hidden />
        {EXTEND_OPTIONS.map((secs) => (
          <button
            key={secs}
            type="button"
            onClick={() => onExtend(secs)}
            disabled={isBusy}
            aria-label={`Add ${secs / 60} ${secs === 60 ? "minute" : "minutes"}`}
            className={`${controlBtn} font-mono`}
          >
            <Plus className="h-3 w-3" />
            <span>{secs / 60} min</span>
          </button>
        ))}
      </div>
      )}
    </div>
  );
}
