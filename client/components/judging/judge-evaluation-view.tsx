"use client";

import React, { useState, useEffect, useMemo, useCallback } from "react";
import {
  Award,
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
  DoorOpen,
  Layers,
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
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function JudgeEvaluationView() {
  const { data: panels = [], isLoading: panelsLoading, refetch: refetchPanels, isFetching } = useJudgePanels();

  // Active panel selection (user override or first panel)
  const [userSelectedPanelId, setUserSelectedPanelId] = useState<string | null>(null);
  const activePanelId = userSelectedPanelId || panels[0]?.panel_id || null;

  const activePanel: JudgePanelSummary | undefined = useMemo(() => {
    return panels.find((p) => p.panel_id === activePanelId) || panels[0];
  }, [panels, activePanelId]);

  // Active slot selection (user override or current presenting/next slot)
  const [userSelectedSlotId, setUserSelectedSlotId] = useState<string | null>(null);

  const activeSlotId = useMemo(() => {
    if (!activePanel) return null;
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
      <div className="p-6 font-mono max-w-7xl mx-auto space-y-4">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (panels.length === 0) {
    return (
      <div className="font-mono space-y-4 max-w-4xl mx-auto py-6">
        <EmptyState
          title="NO JUDGING PANELS ASSIGNED"
          description="You are currently not assigned to evaluate any rooms or panels. Event administrators allocate evaluators to rooms during round setup."
          action={
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetchPanels()}
              className="mt-4 border-black text-xs"
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
    <div className="font-mono space-y-6 max-w-7xl mx-auto">
      {/* Top Header & Panel Switcher */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b-2 border-black pb-4">
        <div>
          <div className="flex items-center gap-2">
            <Award className="h-6 w-6 text-black" />
            <h1 className="text-2xl font-bold tracking-tight uppercase">
              Judge Evaluation Center
            </h1>
          </div>
          <p className="text-sm text-neutral-600 mt-1">
            Grade team pitches, review submitted code repositories, run countdown timers, and lock in scorecards.
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetchPanels()}
            disabled={isFetching}
            className="flex items-center gap-1.5 border-black text-black hover:bg-neutral-100"
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            REFRESH
          </Button>
        </div>
      </div>

      {/* Multi-Panel Tabs if assigned to >1 panel */}
      {panels.length > 1 && (
        <div className="flex items-center gap-2 border-b border-black pb-3 overflow-x-auto">
          <span className="text-xs uppercase font-bold text-neutral-500 shrink-0">Your Panels:</span>
          {panels.map((p) => (
            <button
              key={p.panel_id}
              onClick={() => {
                setUserSelectedPanelId(p.panel_id);
                setUserSelectedSlotId(null);
              }}
              className={`px-3 py-1.5 text-xs font-bold uppercase border transition-all ${
                p.panel_id === activePanel?.panel_id
                  ? "bg-black text-white border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
                  : "bg-white text-neutral-700 border-neutral-300 hover:border-black"
              }`}
            >
              {p.panel_name} ({p.room})
            </button>
          ))}
        </div>
      )}

      {/* Panel Context Banner */}
      {activePanel && (
        <div className="border border-black p-4 bg-neutral-50 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <DoorOpen className="h-5 w-5 text-black" />
              <span className="font-bold text-base uppercase text-black">
                {activePanel.room} — {activePanel.panel_name}
              </span>
              <Badge variant="outline" className="border-black text-[10px] uppercase bg-white">
                {activePanel.round_name}
              </Badge>
            </div>
            <div className="text-xs text-neutral-600 flex items-center gap-3">
              <span>Duration: <strong>{activePanel.slot_duration_mins}m per pitch</strong></span>
              <span>•</span>
              <span>Buffer Depth: <strong>{activePanel.buffer_size} teams</strong></span>
            </div>
          </div>

          <div className="text-left md:text-right shrink-0">
            <div className="text-xs font-bold font-mono">
              Progress: {activePanel.completed_slots} / {activePanel.total_slots} Teams Graded
            </div>
            <div className="w-36 bg-neutral-200 h-2 border border-black mt-1 overflow-hidden">
              <div
                className="bg-black h-full transition-all"
                style={{
                  width: `${
                    activePanel.total_slots > 0
                      ? Math.round((activePanel.completed_slots / activePanel.total_slots) * 100)
                      : 0
                  }%`,
                }}
              />
            </div>
          </div>
        </div>
      )}

      {/* Main Evaluator Work Area: Sidebar Slots & Active Scorecard Center */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Column (4 cols): Queue / Slot Schedule */}
        <div className="lg:col-span-4 space-y-3">
          <div className="flex items-center justify-between border-b border-black pb-2">
            <span className="text-xs uppercase font-bold text-black flex items-center gap-1.5">
              <Layers className="h-4 w-4" />
              Room Pitch Schedule ({activePanel?.slots.length || 0})
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
                  className={`border-2 p-3 cursor-pointer transition-all ${
                    isSelected
                      ? "border-black bg-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] ring-1 ring-black"
                      : "border-neutral-300 bg-neutral-50/70 hover:border-black hover:bg-white"
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <span className="w-5 h-5 bg-black text-white text-[10px] font-bold flex items-center justify-center shrink-0">
                        {idx + 1}
                      </span>
                      <div className="font-bold text-xs uppercase truncate max-w-[170px]">
                        {s.team_name}
                      </div>
                    </div>

                    <div className="flex items-center gap-1">
                      {s.is_my_scorecard_locked && (
                        <span title="Your scorecard is locked">
                          <Lock className="h-3 w-3 text-black" />
                        </span>
                      )}
                      {isCurrent && (
                        <span className="inline-block w-2 h-2 rounded-full bg-black animate-ping" />
                      )}
                    </div>
                  </div>

                  <div className="flex items-center justify-between mt-2 pt-2 border-t border-neutral-200 text-[10px] text-neutral-500 font-mono">
                    <span>
                      {isCurrent ? (
                        <strong className="text-black uppercase">PRESENTING NOW</strong>
                      ) : isDone ? (
                        "COMPLETED"
                      ) : isQueued ? (
                        <strong className="text-neutral-700">ON DECK</strong>
                      ) : (
                        "SCHEDULED"
                      )}
                    </span>

                    {s.is_my_scorecard_locked ? (
                      <span className="text-black font-semibold">LOCKED</span>
                    ) : (
                      <span className="text-neutral-400">UNGRADED</span>
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
              title="NO SLOT SELECTED"
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
        title="SLOT DATA NOT FOUND"
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
    <div className="space-y-6">
      {/* Top Slot Header */}
      <div className="border-2 border-black p-5 bg-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <span className="font-extrabold text-xl uppercase tracking-tight text-black">
                {slot.team_name}
              </span>
              <Badge variant="outline" className="border-black font-mono text-xs">
                Slot #{slot.sequence_order}
              </Badge>
            </div>
            <div className="text-xs text-neutral-500 font-mono mt-0.5">
              Room: {slot.room} • Panel: {slot.panel_name}
            </div>
          </div>

          <div className="flex items-center gap-2">
            {isPresenting && (
              <Badge variant="solid" className="bg-black text-white text-xs uppercase px-2.5 py-1 animate-pulse">
                PRESENTING NOW
              </Badge>
            )}
            {isCompleted && (
              <Badge variant="outline" className="border-black bg-neutral-100 text-black text-xs uppercase px-2.5 py-1">
                EVALUATION COMPLETED
              </Badge>
            )}
            {canStart && (
              <Button
                variant="primary"
                size="sm"
                onClick={handleStartSlot}
                disabled={startSlotMutation.isPending}
                className="bg-black text-white hover:bg-neutral-800 text-xs gap-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
              >
                <Play className="h-3.5 w-3.5 fill-white" />
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
      <div className="border-2 border-black p-5 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-3">
        <div className="flex items-center justify-between border-b border-neutral-200 pb-2">
          <div className="flex items-center gap-2">
            <Globe className="h-4 w-4 text-black" />
            <h3 className="text-xs font-bold uppercase tracking-wider text-black">
              Submitted Code Repositories & Artifacts
            </h3>
          </div>
          <span className="text-[10px] text-neutral-500 uppercase">
            {Object.keys(slot.submission_links || {}).length} Material(s)
          </span>
        </div>

        {Object.keys(slot.submission_links || {}).length === 0 ? (
          <div className="text-xs text-neutral-400 italic p-3 border border-dashed border-neutral-200 text-center">
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
                  className="border border-black p-3 bg-neutral-50 flex items-center justify-between gap-3 text-xs"
                >
                  <div className="flex items-center gap-2.5 truncate">
                    {isGithub ? (
                      <Code2 className="h-4 w-4 shrink-0 text-black" />
                    ) : isSlides ? (
                      <FileText className="h-4 w-4 shrink-0 text-black" />
                    ) : (
                      <Globe className="h-4 w-4 shrink-0 text-black" />
                    )}
                    <div className="truncate">
                      <div className="font-bold uppercase text-[10px] text-neutral-500">{key}</div>
                      <a
                        href={url}
                        target="_blank"
                        rel="noreferrer"
                        className="truncate text-black hover:underline block font-mono text-[11px]"
                      >
                        {url}
                      </a>
                    </div>
                  </div>

                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => handleCopyLink(key, url)}
                      title="Copy URL"
                      className="p-1 border border-neutral-300 hover:border-black bg-white"
                    >
                      {copiedLinkKey === key ? (
                        <Check className="h-3 w-3 text-black" />
                      ) : (
                        <Copy className="h-3 w-3 text-neutral-600" />
                      )}
                    </button>
                    <a
                      href={url}
                      target="_blank"
                      rel="noreferrer"
                      title="Open in new tab"
                      className="p-1 border border-neutral-300 hover:border-black bg-white text-black"
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
      <div className="border-2 border-black p-5 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-5">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 border-b border-black pb-3">
          <div>
            <h3 className="text-sm font-bold uppercase tracking-wider text-black">
              Scoring Rubrics & Criterion Evaluations
            </h3>
            <p className="text-xs text-neutral-500 mt-0.5">
              Rate each criterion up to its maximum score. Weighted contributions sum automatically.
            </p>
          </div>

          <div className="text-right border border-black p-2 bg-neutral-50 shrink-0">
            <div className="text-[10px] uppercase font-bold text-neutral-600">Total Scorecard Rating</div>
            <div className="text-xl font-extrabold text-black font-mono">
              {weightedTotal} <span className="text-xs text-neutral-500 font-normal">/ {totalWeight} pts</span>
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
                className={`border p-4 transition-all ${
                  isLocked
                    ? "border-neutral-200 bg-neutral-50/50"
                    : "border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
                }`}
              >
                <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 pb-2 border-b border-neutral-200">
                  <div className="flex items-center gap-2">
                    <span className="w-5 h-5 bg-black text-white text-[10px] font-bold flex items-center justify-center shrink-0">
                      {idx + 1}
                    </span>
                    <span className="font-bold text-sm uppercase text-black">
                      {c.criterion_name}
                    </span>
                  </div>

                  <div className="flex items-center gap-2 text-xs font-mono">
                    <Badge variant="outline" className="border-black text-[10px]">
                      Weight: {c.weight}%
                    </Badge>
                    <span className="text-neutral-500">
                      Contribution: <strong>{weightedPoints} pts</strong>
                    </span>
                  </div>
                </div>

                {/* Score Controls */}
                <div className="grid grid-cols-1 sm:grid-cols-12 gap-4 mt-3">
                  <div className="sm:col-span-5 space-y-2">
                    <div className="flex items-center justify-between">
                      <label className="text-[11px] uppercase font-bold text-neutral-600">
                        Score (0 to {c.max_score})
                      </label>
                      <span className="text-sm font-bold font-mono text-black">
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
                          className="w-full accent-black cursor-pointer"
                        />

                        {/* Quick Stepper Chips */}
                        <div className="flex items-center gap-1 flex-wrap">
                          {[0, Math.round(c.max_score * 0.25), Math.round(c.max_score * 0.5), Math.round(c.max_score * 0.75), c.max_score].map(
                            (presetVal) => (
                              <button
                                key={presetVal}
                                type="button"
                                onClick={() => handleScoreChange(c.criterion_id, presetVal)}
                                className={`px-2 py-0.5 text-[10px] font-mono border ${
                                  currentScore === presetVal
                                    ? "bg-black text-white border-black font-bold"
                                    : "bg-white text-neutral-700 border-neutral-300 hover:border-black"
                                }`}
                              >
                                {presetVal}
                              </button>
                            )
                          )}
                        </div>
                      </div>
                    ) : (
                      <div className="p-2 border border-neutral-300 bg-neutral-100 font-mono text-xs font-bold">
                        {currentScore} pts (Locked)
                      </div>
                    )}
                  </div>

                  <div className="sm:col-span-7 space-y-1">
                    <label className="text-[11px] uppercase font-bold text-neutral-600">
                      Evaluator Notes & Observations
                    </label>
                    <textarea
                      rows={2}
                      disabled={isLocked}
                      placeholder="Enter feedback or observations on this rubric..."
                      value={currentNotes}
                      onChange={(e) => handleNoteChange(c.criterion_id, e.target.value)}
                      className="w-full p-2 border border-black font-mono text-xs resize-none focus:outline-none focus:ring-1 focus:ring-black disabled:bg-neutral-100 disabled:text-neutral-500"
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Action Controls Footer: Save Draft / Lock / Call Next */}
        <div className="border-t-2 border-black pt-4 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div>
            {isLocked ? (
              <div className="flex items-center gap-2 text-xs font-bold text-black">
                <CheckCircle2 className="h-4 w-4 text-black" />
                <span>YOUR SCORECARD IS LOCKED</span>
                <span className="text-neutral-500 font-normal">
                  {slot.all_panel_judges_locked
                    ? "(All judges locked)"
                    : "(Waiting for other panel judges)"}
                </span>
              </div>
            ) : (
              <div className="text-xs text-neutral-500 font-mono">
                Draft ratings are visible only to you until locked.
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
                  className="border-black text-black hover:bg-neutral-100 text-xs flex items-center gap-1"
                >
                  <Save className="h-3.5 w-3.5" />
                  <span>SAVE DRAFT</span>
                </Button>

                <Button
                  size="sm"
                  onClick={() => setIsLockConfirmOpen(true)}
                  disabled={lockScoresMutation.isPending}
                  className="bg-black text-white hover:bg-neutral-800 border border-black text-xs flex items-center gap-1 shadow-sm"
                >
                  <Lock className="h-3.5 w-3.5" />
                  <span>LOCK IN SCORES</span>
                </Button>
              </>
            )}

            {/* CALL NEXT TEAM BUTTON */}
            {isLocked && !isCompleted && (
              <Button
                variant="primary"
                size="sm"
                onClick={handleCompleteAndNext}
                disabled={completeSlotMutation.isPending}
                className="bg-black text-white hover:bg-neutral-800 border border-black text-xs flex items-center gap-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
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
        title="LOCK IN SCORECARD RATINGS"
        description="Finalize this evaluation and submit ratings to the leaderboard."
      >
        <div className="space-y-4 pt-2 font-mono">
          <div className="border border-black p-3 bg-neutral-50 space-y-2">
            <div className="text-sm font-bold uppercase">{slot.team_name}</div>
            <div className="text-xs text-neutral-600">
              Total Score: <strong>{weightedTotal} / {totalWeight} pts</strong>
            </div>
          </div>

          <div className="flex items-start gap-2.5 p-3 border border-black bg-white text-xs">
            <AlertTriangle className="h-4 w-4 text-black shrink-0 mt-0.5" />
            <p className="text-neutral-700 leading-relaxed">
              Once locked, your scores cannot be modified. They will immediately contribute to the final leaderboard calculations.
            </p>
          </div>

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-neutral-200">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsLockConfirmOpen(false)}
              disabled={lockScoresMutation.isPending}
              className="border-black text-black"
            >
              CANCEL
            </Button>
            <Button
              size="sm"
              onClick={handleConfirmLock}
              disabled={lockScoresMutation.isPending}
              className="bg-black text-white hover:bg-neutral-800 border border-black flex items-center gap-1"
            >
              {lockScoresMutation.isPending ? "LOCKING..." : "CONFIRM & LOCK SCORES"}
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
      <div className="border border-neutral-200 p-3 bg-neutral-50 flex items-center justify-between text-xs text-neutral-600 font-mono">
        <span className="flex items-center gap-1.5">
          <Clock className="h-4 w-4 text-neutral-400" />
          Allocated Slot Duration: <strong>{durationMins} minutes</strong>
        </span>
        <span className="text-neutral-400 uppercase">
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
      className={`border-2 p-3 font-mono transition-all ${
        isOvertime
          ? "border-black bg-neutral-100 text-black"
          : secondsRemaining <= 120
          ? "border-black bg-white text-black ring-1 ring-black"
          : "border-black bg-white text-black"
      }`}
    >
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
        <div className="flex items-center gap-3">
          <Clock className={`h-6 w-6 text-black ${isOvertime ? "animate-bounce" : ""}`} />
          <div>
            <div className="text-[10px] uppercase font-bold text-neutral-500 tracking-wider">
              {isOvertime ? "OVERTIME WARNING" : "LIVE PITCH COUNTDOWN"}
            </div>
            <div className="text-2xl font-extrabold tracking-tight">
              {isOvertime ? `+${formattedTime} OVERTIME` : `${formattedTime} REMAINING`}
            </div>
          </div>
        </div>

        <div className="text-right text-xs text-neutral-600">
          <div>Elapsed: <strong>{formattedElapsed}</strong> / {durationMins}:00</div>
          <div className="w-32 bg-neutral-200 h-1.5 border border-black mt-1 ml-auto overflow-hidden">
            <div
              className="bg-black h-full transition-all"
              style={{ width: `${100 - percentLeft}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
