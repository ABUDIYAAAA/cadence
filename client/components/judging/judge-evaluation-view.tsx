"use client";

import React, { useState, useEffect, useMemo, useCallback } from "react";
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
} from "lucide-react";
import {
  useJudgePanels,
  useJudgeSlotDetails,
  useStartSlot,
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
              Check for Assignments
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
          <Metric label="Slot length" value={`${activePanel.slot_duration_mins}m`} />
          <Metric label="Buffer depth" value={activePanel.buffer_size} suffix="teams" />
        </MetricGrid>
      )}

      {/* Main Evaluator Work Area: Sidebar Slots & Active Scorecard Center */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Column (4 cols): Queue / Slot Schedule */}
        <div className="lg:col-span-4 space-y-3">
          <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-2">
            <span className="text-xs font-semibold text-[var(--ink-secondary)]">
              Room pitch schedule ({activePanel?.slots.length || 0})
            </span>
          </div>

          <div className="space-y-2 max-h-[700px] overflow-y-auto pr-1">
            {activePanel?.slots.map((s, idx) => {
              const isSelected = s.slot_id === activeSlotId;
              const isCurrent = s.status === "in_progress";
              const isDone = s.status === "completed";
              const isQueued = s.status === "queued";

              return (
                <div
                  key={s.slot_id}
                  onClick={() => setUserSelectedSlotId(s.slot_id)}
                  className={`rounded-[10px] p-3 cursor-pointer transition-all border ${
                    isSelected
                      ? "border-[var(--color-blue)] bg-[var(--color-blue-dim)] shadow-xs"
                      : "border-[var(--card-line)] bg-[var(--card-solid)] hover:border-[var(--card-line-hover)] hover:bg-[var(--card-solid-subtle)]"
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2.5">
                      <span className={`w-5 h-5 rounded-[4px] text-[10px] font-bold flex items-center justify-center shrink-0 ${
                        isSelected
                          ? "bg-[var(--color-blue)] text-white"
                          : "bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] border border-[var(--card-line)]"
                      }`}>
                        {idx + 1}
                      </span>
                      <div className="font-semibold text-xs truncate max-w-[170px] text-[var(--ink-primary)]">
                        {s.team_name}
                      </div>
                    </div>

                    <div className="flex items-center gap-1">
                      {s.is_my_scorecard_locked && (
                        <span title="Your scorecard is locked">
                          <Lock className="h-3 w-3 text-[var(--ink-secondary)]" />
                        </span>
                      )}
                      {isCurrent && (
                        <span className="inline-block w-2 h-2 rounded-full bg-[var(--color-coral)] animate-ping" />
                      )}
                    </div>
                  </div>

                  <div className="flex items-center justify-between mt-2 pt-2 border-t border-[var(--card-line)]/60 text-[10px] font-mono">
                    <span>
                      {isCurrent ? (
                        <strong className="text-[var(--color-coral)] font-semibold">Presenting now</strong>
                      ) : isDone ? (
                        <span className="text-[var(--color-emerald)] font-medium">Completed</span>
                      ) : isQueued ? (
                        <strong className="text-[var(--color-blue)] font-semibold">On deck</strong>
                      ) : (
                        <span className="text-[var(--ink-tertiary)]">Scheduled</span>
                      )}
                    </span>

                    {s.is_my_scorecard_locked ? (
                      <span className="text-[var(--ink-secondary)] font-medium">Locked</span>
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
  const { data: slot, isLoading, refetch } = useJudgeSlotDetails(slotId);

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
      panel={panel}
      onRefetch={refetch}
      onSlotAdvanced={onSlotAdvanced}
    />
  );
}

// Subcomponent: Form state initialized directly from slot props
function JudgeSlotScorecardForm({
  slot,
  panel,
  onRefetch,
  onSlotAdvanced,
}: {
  slot: JudgeSlotDetails;
  panel?: JudgePanelSummary;
  onRefetch: () => void;
  onSlotAdvanced: (nextSlotId?: string) => void;
}) {
  const toast = useToast();
  const startSlotMutation = useStartSlot();
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

  const [isLockConfirmOpen, setIsLockConfirmOpen] = useState(false);
  const [copiedLinkKey, setCopiedLinkKey] = useState<string | null>(null);

  // Handle score value change
  const handleScoreChange = (criterionId: string, val: number) => {
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
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to start pitch";
      toast.error(message);
    }
  };

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
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to save scores";
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
      toast.success(`Scorecard locked for team "${slot.team_name}".`);
      setIsLockConfirmOpen(false);
      onRefetch();
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to lock scores";
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
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to advance slot";
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

  return (
    <div className="space-y-6 font-sans">
      {/* Top Slot Header */}
      <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 sm:p-6 space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2.5">
              <span className="font-extrabold text-xl tracking-tight text-[var(--ink-primary)]">
                {slot.team_name}
              </span>
              <Badge variant="blue" className="text-xs font-mono font-semibold">
                Slot #{slot.sequence_order}
              </Badge>
            </div>
            <div className="text-xs text-[var(--ink-secondary)] mt-1">
              Room: <strong className="text-[var(--ink-primary)]">{slot.room}</strong> • Panel: <strong className="text-[var(--ink-primary)]">{slot.panel_name}</strong>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {isPresenting && (
              <Badge variant="coral" className="text-xs uppercase px-2.5 py-1 animate-pulse font-semibold">
                PRESENTING NOW
              </Badge>
            )}
            {isCompleted && (
              <Badge variant="solid" className="text-xs uppercase px-2.5 py-1 font-semibold">
                EVALUATION COMPLETED
              </Badge>
            )}
            {canStart && (
              <Button
                variant="blue"
                size="sm"
                onClick={handleStartSlot}
                disabled={startSlotMutation.isPending}
                className="text-xs gap-1.5 h-9 px-4 font-semibold"
              >
                <Play className="h-3.5 w-3.5 fill-current" />
                <span>START PITCH & TIMER</span>
              </Button>
            )}
          </div>
        </div>

        {/* Live Pitch Countdown Timer Component */}
        <JudgeCountdownTimer
          actualStartTime={slot.actual_start_time}
          durationMins={slot.slot_duration_mins}
          durationSeconds={slot.slot_duration_seconds}
          status={slot.status}
        />
      </div>

      {/* Submitted Repositories & Artifacts Section */}
      <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 space-y-3">
        <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-3">
          <div className="flex items-center gap-2">
            <Globe className="h-4 w-4 text-[var(--color-blue)]" />
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--ink-primary)]">
              Submitted Code Repositories & Artifacts
            </h3>
          </div>
          <span className="text-[10px] text-[var(--ink-tertiary)] uppercase font-semibold">
            {Object.keys(slot.submission_links || {}).length} Material(s)
          </span>
        </div>

        {Object.keys(slot.submission_links || {}).length === 0 ? (
          <div className="text-xs text-[var(--ink-tertiary)] italic p-4 border border-dashed border-[var(--card-line)] rounded-[16px] text-center">
            No repository or presentation links submitted yet by this squad.
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {Object.entries(slot.submission_links).map(([key, url]) => {
              const isGithub = key.toLowerCase().includes("git") || url.includes("github.com");
              const isSlides = key.toLowerCase().includes("slide") || url.includes("presentation");

              return (
                <div
                  key={key}
                  className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] flex items-center justify-between gap-3 text-xs"
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
                      <div className="font-bold uppercase text-[10px] text-[var(--ink-tertiary)] tracking-wider">{key}</div>
                      <a
                        href={url}
                        target="_blank"
                        rel="noreferrer"
                        className="truncate text-[var(--ink-primary)] hover:text-[var(--color-blue)] hover:underline block font-mono text-[11px]"
                      >
                        {url}
                      </a>
                    </div>
                  </div>

                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => handleCopyLink(key, url)}
                      title="Copy URL"
                      className="p-1.5 rounded-lg border border-[var(--card-line)] hover:border-[#83A2DB] bg-[var(--card-solid)] transition-colors cursor-pointer"
                    >
                      {copiedLinkKey === key ? (
                        <Check className="h-3 w-3 text-[#2EB88A]" />
                      ) : (
                        <Copy className="h-3 w-3 text-[var(--ink-secondary)]" />
                      )}
                    </button>
                    <a
                      href={url}
                      target="_blank"
                      rel="noreferrer"
                      title="Open in new tab"
                      className="p-1.5 rounded-lg border border-[var(--card-line)] hover:border-[#83A2DB] bg-[var(--card-solid)] text-[var(--ink-secondary)] transition-colors"
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
      <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 sm:p-6 space-y-5">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 border-b border-[var(--card-line)] pb-4">
          <div>
            <h3 className="text-sm font-bold uppercase tracking-wider text-[var(--ink-primary)]">
              Scoring Rubrics & Criterion Evaluations
            </h3>
            <p className="text-xs text-[var(--ink-secondary)] mt-0.5">
              Rate each criterion up to its maximum score. Weighted contributions sum automatically.
            </p>
          </div>

          <div className="text-right rounded-[16px] border border-[var(--card-line)] p-3 px-4 bg-[var(--card-solid-subtle)] shrink-0">
            <div className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] tracking-wider">Total Scorecard Rating</div>
            <div className="text-2xl font-extrabold text-[var(--ink-primary)] font-mono">
              {weightedTotal} <span className="text-xs text-[var(--ink-tertiary)] font-normal">/ {totalWeight} pts</span>
            </div>
          </div>
        </div>

        {/* Criteria Rating Rows */}
        <div className="space-y-4">
          {slot.criteria_scores.map((c, idx) => {
            const currentScore = formScores[c.criterion_id]?.score ?? 0;
            const currentNotes = formScores[c.criterion_id]?.notes ?? "";
            const weightedPoints =
              c.max_score > 0 ? Math.round(((currentScore / c.max_score) * c.weight) * 100) / 100 : 0;

            return (
              <div
                key={c.criterion_id}
                className={`rounded-[18px] border p-4.5 transition-all ${
                  isLocked
                    ? "border-[var(--card-line)] bg-[var(--card-solid-subtle)]/60"
                    : "border-[var(--card-line)] bg-[var(--card-solid-subtle)]"
                }`}
              >
                <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 pb-3 border-b border-[var(--card-line)]">
                  <div className="flex items-center gap-2.5">
                    <span className="w-5 h-5 rounded-full bg-[var(--color-dark)] text-[var(--ink-inverse)] text-[10px] font-bold flex items-center justify-center shrink-0">
                      {idx + 1}
                    </span>
                    <span className="font-bold text-sm text-[var(--ink-primary)]">
                      {c.criterion_name}
                    </span>
                  </div>

                  <div className="flex items-center gap-2.5 text-xs font-mono">
                    <Badge variant="subtle" className="text-[10px] font-semibold">
                      Weight: {c.weight}%
                    </Badge>
                    <span className="text-[var(--ink-secondary)]">
                      Contribution: <strong className="text-[var(--ink-primary)]">{weightedPoints} pts</strong>
                    </span>
                  </div>
                </div>

                {/* Score Controls */}
                <div className="grid grid-cols-1 sm:grid-cols-12 gap-4 mt-3.5">
                  <div className="sm:col-span-5 space-y-2.5">
                    <div className="flex items-center justify-between">
                      <label className="text-[11px] uppercase font-bold text-[var(--ink-secondary)] tracking-wider">
                        Score (0 to {c.max_score})
                      </label>
                      <span className="text-sm font-bold font-mono text-[var(--ink-primary)]">
                        {currentScore} / {c.max_score}
                      </span>
                    </div>

                    {!isLocked ? (
                      <div className="space-y-2.5">
                        {/* Interactive Range Slider */}
                        <input
                          type="range"
                          min={0}
                          max={c.max_score}
                          step={1}
                          value={currentScore}
                          onChange={(e) => handleScoreChange(c.criterion_id, Number(e.target.value))}
                          className="w-full accent-[#83A2DB] cursor-pointer"
                        />

                        {/* Quick Stepper Chips from preview2.html */}
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {[0, Math.round(c.max_score * 0.25), Math.round(c.max_score * 0.5), Math.round(c.max_score * 0.75), c.max_score].map(
                            (presetVal) => (
                              <button
                                key={presetVal}
                                type="button"
                                onClick={() => handleScoreChange(c.criterion_id, presetVal)}
                                className={`px-2.5 py-1 rounded-[10px] text-xs font-mono font-bold border transition-all cursor-pointer active:scale-95 ${
                                  currentScore === presetVal
                                    ? "bg-[var(--color-dark)] text-[var(--ink-inverse)] border-[var(--color-dark)] shadow-sm"
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
                      <div className="p-2.5 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] font-mono text-xs font-bold text-[var(--ink-primary)]">
                        {currentScore} pts (Locked)
                      </div>
                    )}
                  </div>

                  <div className="sm:col-span-7 space-y-1">
                    <label className="text-[11px] uppercase font-bold text-[var(--ink-secondary)] tracking-wider">
                      Evaluator Notes & Observations
                    </label>
                    <textarea
                      rows={2}
                      disabled={isLocked}
                      placeholder="Enter feedback or observations on this rubric..."
                      value={currentNotes}
                      onChange={(e) => handleNoteChange(c.criterion_id, e.target.value)}
                      className="w-full p-2.5 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] text-xs resize-none focus:outline-none focus:border-[#83A2DB] focus:ring-2 focus:ring-[#83A2DB]/20 disabled:bg-[#F8FAFD] disabled:text-[var(--ink-tertiary)] transition-all text-[var(--ink-primary)]"
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Action Controls Footer: Save Draft / Lock / Call Next */}
        <div className="border-t border-[var(--card-line)] pt-4 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div>
            {isLocked ? (
              <div className="flex items-center gap-2 text-xs font-bold text-[#2EB88A]">
                <CheckCircle2 className="h-4 w-4" />
                <span>YOUR SCORECARD IS LOCKED</span>
                <span className="text-[var(--ink-tertiary)] font-normal">
                  {slot.all_panel_judges_locked
                    ? "(All judges locked)"
                    : "(Waiting for other panel judges)"}
                </span>
              </div>
            ) : (
              <div className="text-xs text-[var(--ink-tertiary)]">
                Draft ratings are saved and visible only to you until locked.
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
                  disabled={submitScoresMutation.isPending}
                  className="text-xs flex items-center gap-1.5 h-9"
                >
                  <Save className="h-3.5 w-3.5" />
                  <span>SAVE DRAFT</span>
                </Button>

                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => setIsLockConfirmOpen(true)}
                  disabled={lockScoresMutation.isPending}
                  className="text-xs flex items-center gap-1.5 h-9"
                >
                  <Lock className="h-3.5 w-3.5" />
                  <span>LOCK IN SCORES</span>
                </Button>
              </>
            )}

            {/* CALL NEXT TEAM BUTTON */}
            {isLocked && !isCompleted && (
              <Button
                variant="blue"
                size="sm"
                onClick={handleCompleteAndNext}
                disabled={completeSlotMutation.isPending}
                className="text-xs flex items-center gap-1.5 h-9"
              >
                <span>CALL NEXT TEAM / COMPLETE</span>
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
          <div className="rounded-[16px] border border-[var(--card-line)] p-4 bg-[var(--card-solid-subtle)] space-y-2">
            <div className="text-sm font-bold text-[var(--ink-primary)]">{slot.team_name}</div>
            <div className="text-xs text-[var(--ink-secondary)]">
              Total Score: <strong className="text-[var(--ink-primary)] font-mono">{weightedTotal} / {totalWeight} pts</strong>
            </div>
          </div>

          <div className="flex items-start gap-3 p-3.5 rounded-[16px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] text-xs">
            <AlertTriangle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />
            <p className="text-[var(--color-coral)] leading-relaxed">
              Once locked, your scores cannot be modified. They will immediately contribute to the final leaderboard calculations.
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
              {lockScoresMutation.isPending ? "Locking..." : "Confirm & Lock Scores"}
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}

// Live Countdown Timer Subcomponent
function JudgeCountdownTimer({
  actualStartTime,
  durationMins,
  durationSeconds,
  status,
}: {
  actualStartTime?: string | null;
  durationMins: number;
  durationSeconds?: number;
  status: string;
}) {
  const totalSeconds = durationSeconds || durationMins * 60;

  const getSecondsRemaining = useCallback(() => {
    if (!actualStartTime || status !== "in_progress") {
      return null;
    }
    const startMs = new Date(actualStartTime).getTime();
    const nowMs = Date.now();
    const elapsedSecs = Math.floor((nowMs - startMs) / 1000);
    return totalSeconds - elapsedSecs;
  }, [actualStartTime, status, totalSeconds]);

  const [secondsRemaining, setSecondsRemaining] = useState<number | null>(getSecondsRemaining);

  useEffect(() => {
    if (status !== "in_progress" || !actualStartTime) return;
    const interval = setInterval(() => {
      setSecondsRemaining(getSecondsRemaining());
    }, 1000);
    return () => clearInterval(interval);
  }, [status, actualStartTime, getSecondsRemaining]);

  if (status !== "in_progress" || secondsRemaining === null) {
    return (
      <div className="rounded-[16px] border border-[var(--card-line)] p-3.5 px-4 bg-[var(--card-solid-subtle)] flex items-center justify-between text-xs text-[var(--ink-secondary)]">
        <span className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-[var(--ink-tertiary)]" />
          Allocated Slot Duration: <strong className="text-[var(--ink-primary)] font-mono">{durationMins} minutes</strong>
        </span>
        <span className="text-[var(--ink-tertiary)]">
          {status === "completed" ? "Timer Stopped (Finished)" : "Timer Idle (Click Start to Begin)"}
        </span>
      </div>
    );
  }

  const isOvertime = secondsRemaining < 0;
  const absRemaining = Math.abs(secondsRemaining);
  const minutes = Math.floor(absRemaining / 60);
  const seconds = absRemaining % 60;
  const formattedTime = `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;

  const elapsed = Math.max(0, totalSeconds - secondsRemaining);
  const elapsedMinutes = Math.floor(elapsed / 60);
  const elapsedSeconds = elapsed % 60;
  const formattedElapsed = `${String(elapsedMinutes).padStart(2, "0")}:${String(elapsedSeconds).padStart(2, "0")}`;

  const percentLeft = Math.max(0, Math.min(100, (secondsRemaining / totalSeconds) * 100));

  return (
    <div
      className={`rounded-[18px] p-4.5 transition-all ${
        isOvertime
          ? "border border-[#CE6969]/40 bg-[var(--color-coral-dim)]"
          : secondsRemaining <= 120
          ? "border border-[var(--color-coral-border)] bg-[var(--color-coral-dim)]/60"
          : "border border-[#83A2DB]/30 bg-[var(--color-blue-dim)]"
      }`}
    >
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Clock className={`h-6 w-6 ${isOvertime ? "text-[var(--color-coral)] animate-bounce" : "text-[var(--color-blue)]"}`} />
          <div>
            <div className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] tracking-wider">
              {isOvertime ? "OVERTIME WARNING" : "LIVE PITCH COUNTDOWN"}
            </div>
            <div className="timer-digits text-2xl sm:text-3xl font-extrabold tracking-tight font-mono text-[var(--color-coral)]">
              {isOvertime ? `+${formattedTime} OVERTIME` : `${formattedTime} REMAINING`}
            </div>
          </div>
        </div>

        <div className="text-right text-xs text-[var(--ink-secondary)]">
          <div className="font-mono">Elapsed: <strong className="text-[var(--ink-primary)]">{formattedElapsed}</strong> / {durationMins}:00</div>
          <div className="w-36 bg-[var(--card-line)] h-2 rounded-full mt-1.5 ml-auto overflow-hidden">
            <div
              className={`h-full rounded-full transition-all ${isOvertime ? "bg-[#CE6969]" : "bg-[#83A2DB]"}`}
              style={{ width: `${100 - percentLeft}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
