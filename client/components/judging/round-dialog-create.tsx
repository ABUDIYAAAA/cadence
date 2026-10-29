"use client";

import React, { useState, useMemo } from "react";
import {
  Award,
  Plus,
  Trash2,
  Layers,
  DoorOpen,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
  Sparkles,
  Search,
  Clock,
  ArrowUp,
  ArrowDown,
  AlertCircle,
  HelpCircle,
  Check,
} from "lucide-react";
import { useUsers } from "@/hooks/use-users";
import { useTeams } from "@/hooks/use-teams";
import {
  CreateCriteriaRequest,
  CreateRoundRequest,
  SlotAssignment,
} from "@/types/judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";

interface PanelDraft {
  id: string;
  name: string;
  room: string;
  judge_ids: string[];
  assigned_team_ids: string[];
}

interface RoundCreateDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmitFullWorkflow: (params: {
    round: CreateRoundRequest;
    panels: Array<{
      name: string;
      room: string;
      judge_ids: string[];
      slots: SlotAssignment[];
    }>;
  }) => Promise<void>;
  isLoading: boolean;
}

export interface RubricPreset {
  id: string;
  name: string;
  tagline: string;
  criteria: CreateCriteriaRequest[];
}

const RUBRIC_PRESETS: RubricPreset[] = [
  {
    id: "standard",
    name: "Standard Hackathon",
    tagline: "Architecture, Innovation, UX & Pitch",
    criteria: [
      {
        name: "Technical Execution & Architecture",
        description: "Code craft, system scalability, robust architecture, and code hygiene",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 1,
      },
      {
        name: "Innovation & Problem Solving",
        description: "Originality of concept, problem relevance, and creative approach",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 2,
      },
      {
        name: "UI / UX & Prototype Polish",
        description: "Visual hierarchy, usability, responsive interaction, and aesthetic craft",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 3,
      },
      {
        name: "Presentation & Feasibility",
        description: "Clarity of pitch, market viability, roadmap, and live Q&A defense",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 4,
      },
    ],
  },
  {
    id: "ai_tech",
    name: "AI & Deep Tech",
    tagline: "Algorithms, Reliability & Live Demos",
    criteria: [
      {
        name: "Algorithmic Novelty & Complexity",
        description: "Model selection, pipeline sophistication, and technical depth",
        min_score: 0,
        max_score: 10,
        weight: 1.5,
        order_index: 1,
      },
      {
        name: "System Reliability & Latency",
        description: "Throughput, error handling, edge cases, and inference efficiency",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 2,
      },
      {
        name: "Working Demonstration",
        description: "Live functional prototype tested with real, unpredictable data",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 3,
      },
      {
        name: "Practical Value & Viability",
        description: "Addresses a genuine enterprise or consumer bottleneck effectively",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 4,
      },
    ],
  },
  {
    id: "design_product",
    name: "Design & Product Jam",
    tagline: "User Delight, Visual Craft & Value",
    criteria: [
      {
        name: "User Experience & Task Flow",
        description: "Frictionless journeys, cognitive clarity, and accessible design",
        min_score: 0,
        max_score: 10,
        weight: 1.2,
        order_index: 1,
      },
      {
        name: "Visual Polish & Craftsmanship",
        description: "Typography rhythm, color harmony, micro-interactions, and visual identity",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 2,
      },
      {
        name: "Problem-Solution Validation",
        description: "Authentic user research empathy and validated product-market fit",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: 3,
      },
      {
        name: "Storytelling & Demo Delivery",
        description: "Compelling narrative structure and articulate vision communication",
        min_score: 0,
        max_score: 10,
        weight: 0.8,
        order_index: 4,
      },
    ],
  },
];

export function RoundCreateDialog({
  open,
  onClose,
  onSubmitFullWorkflow,
  isLoading,
}: RoundCreateDialogProps) {
  // Wizard steps: 1 = Round & Criteria, 2 = Rooms & Evaluators, 3 = Squad Scheduling, 4 = Review & Provision
  const [step, setStep] = useState<1 | 2 | 3 | 4>(1);

  // Queries for Evaluators (Judges + Admins) and Teams
  const { data: usersData, isLoading: usersLoading } = useUsers({ role: "judge", limit: 100 });
  const { data: teamsData, isLoading: teamsLoading } = useTeams({ limit: 100 });

  // Evaluators are judges only
  const allEvaluators = useMemo(() => {
    const list = usersData?.data || [];
    return list
      .filter((u) => u.role === "judge")
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [usersData]);

  // Available Teams
  const availableTeams = useMemo(() => teamsData?.data || [], [teamsData]);

  // Form State - Step 1: Round & Criteria
  const [roundName, setRoundName] = useState("");
  const [bufferSize, setBufferSize] = useState(2);
  const [slotDurationMins, setSlotDurationMins] = useState(10);
  const [criteria, setCriteria] = useState<CreateCriteriaRequest[]>(
    RUBRIC_PRESETS[0].criteria
  );
  const [activePresetId, setActivePresetId] = useState<string>("standard");

  // Form State - Step 2: Rooms & Evaluators
  const [enableRooms, setEnableRooms] = useState(true);
  const [panels, setPanels] = useState<PanelDraft[]>([
    {
      id: "panel-1",
      name: "Panel Alpha",
      room: "Room 101 - Main Lab",
      judge_ids: [],
      assigned_team_ids: [],
    },
  ]);
  const [evaluatorSearchQuery, setEvaluatorSearchQuery] = useState("");

  // Form State - Step 3: Squads & Staging Queues
  const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([]);
  const [teamSearchQuery, setTeamSearchQuery] = useState("");

  // Validation & Error Notice
  // The notice is tied to the form values it was raised for, so it clears itself as soon as
  // anything is edited or the step changes, instead of waiting to be dismissed.
  const formKey = JSON.stringify([
    step,
    roundName,
    bufferSize,
    slotDurationMins,
    criteria,
    enableRooms,
    panels,
    selectedTeamIds,
  ]);
  const [stepNotice, setStepNotice] = useState<{ message: string; key: string } | null>(null);
  const stepErrors = stepNotice && stepNotice.key === formKey ? stepNotice.message : null;
  const setStepErrors = (message: string | null) =>
    setStepNotice(message ? { message, key: formKey } : null);

  // Filtered Evaluators for Step 2
  const filteredEvaluators = useMemo(() => {
    return allEvaluators.filter((ev) => {
      const q = evaluatorSearchQuery.trim().toLowerCase();
      const matchesSearch =
        !q ||
        ev.name.toLowerCase().includes(q) ||
        ev.email.toLowerCase().includes(q);
      return matchesSearch;
    });
  }, [allEvaluators, evaluatorSearchQuery]);

  // Filtered Teams for Step 3
  const filteredTeams = useMemo(() => {
    const q = teamSearchQuery.trim().toLowerCase();
    if (!q) return availableTeams;
    return availableTeams.filter((t) => t.name.toLowerCase().includes(q));
  }, [availableTeams, teamSearchQuery]);

  // Reset helper
  const resetWizard = () => {
    setStep(1);
    setRoundName("");
    setBufferSize(2);
    setSlotDurationMins(10);
    setCriteria(RUBRIC_PRESETS[0].criteria);
    setActivePresetId("standard");
    setEnableRooms(true);
    setPanels([
      {
        id: "panel-1",
        name: "Panel Alpha",
        room: "Room 101 - Main Lab",
        judge_ids: [],
        assigned_team_ids: [],
      },
    ]);
    setSelectedTeamIds([]);
    setStepErrors(null);
    setEvaluatorSearchQuery("");
    setTeamSearchQuery("");
  };

  const handleClose = () => {
    if (!isLoading) {
      resetWizard();
      onClose();
    }
  };

  // --- Step 1 Handlers ---
  const handleApplyPreset = (preset: RubricPreset) => {
    setActivePresetId(preset.id);
    setCriteria(preset.criteria);
  };

  const handleAddCriterion = () => {
    setActivePresetId("custom");
    setCriteria((prev) => [
      ...prev,
      {
        name: `Criterion ${prev.length + 1}`,
        description: "",
        min_score: 0,
        max_score: 10,
        weight: 1.0,
        order_index: prev.length + 1,
      },
    ]);
  };

  const handleRemoveCriterion = (index: number) => {
    if (criteria.length <= 1) return;
    setActivePresetId("custom");
    setCriteria((prev) =>
      prev
        .filter((_, i) => i !== index)
        .map((c, i) => ({ ...c, order_index: i + 1 }))
    );
  };

  const handleCriterionChange = (
    index: number,
    field: keyof CreateCriteriaRequest,
    val: string | number
  ) => {
    setActivePresetId("custom");
    setCriteria((prev) =>
      prev.map((c, i) => (i === index ? { ...c, [field]: val } : c))
    );
  };

  // --- Step 2 Handlers ---
  const handleAddPanel = () => {
    const nextIdx = panels.length + 1;
    const letter = String.fromCharCode(65 + panels.length);
    setPanels((prev) => [
      ...prev,
      {
        id: `panel-${Date.now()}-${nextIdx}`,
        name: `Panel ${letter}`,
        room: `Room ${100 + nextIdx}`,
        judge_ids: [],
        assigned_team_ids: [],
      },
    ]);
  };

  const handleRemovePanel = (panelId: string) => {
    if (panels.length <= 1) return;
    setPanels((prev) => prev.filter((p) => p.id !== panelId));
  };

  const handleToggleJudgeForPanel = (panelId: string, judgeId: string) => {
    setPanels((prev) =>
      prev.map((p) => {
        if (p.id !== panelId) return p;
        const exists = p.judge_ids.includes(judgeId);
        return {
          ...p,
          judge_ids: exists
            ? p.judge_ids.filter((id) => id !== judgeId)
            : [...p.judge_ids, judgeId],
        };
      })
    );
  };

  // --- Step 3 Handlers ---
  const handleToggleTeam = (teamId: string) => {
    if (selectedTeamIds.includes(teamId)) {
      setSelectedTeamIds((prev) => prev.filter((id) => id !== teamId));
      // Deselected teams also leave their room queue
      setPanels((prev) =>
        prev.map((p) => ({
          ...p,
          assigned_team_ids: p.assigned_team_ids.filter((id) => id !== teamId),
        }))
      );
    } else {
      setSelectedTeamIds((prev) => [...prev, teamId]);
    }
  };

  const handleSelectAllTeams = () => {
    setSelectedTeamIds(availableTeams.map((t) => t.id));
  };

  // Clears everything chosen in this step: the selected teams, every room queue, and the search filter.
  const handleClearTeams = () => {
    setSelectedTeamIds([]);
    setPanels((prev) =>
      prev.map((p) => ({ ...p, assigned_team_ids: [] }))
    );
    setTeamSearchQuery("");
  };

  // Teams that are selected but not yet in any room queue
  const unassignedTeamIds = selectedTeamIds.filter(
    (id) => !panels.some((p) => p.assigned_team_ids.includes(id))
  );

  // Places only the unassigned teams (into the least-loaded rooms), keeping existing queues and their order
  const handleAutoDistributeTeams = () => {
    if (panels.length === 0 || unassignedTeamIds.length === 0) return;

    const newPanels = panels.map((p) => ({
      ...p,
      assigned_team_ids: [...p.assigned_team_ids],
    }));

    unassignedTeamIds.forEach((teamId) => {
      const target = newPanels.reduce((min, p) =>
        p.assigned_team_ids.length < min.assigned_team_ids.length ? p : min
      );
      target.assigned_team_ids.push(teamId);
    });

    setPanels(newPanels);
  };

  // Move slot order up/down within a panel
  const handleMoveSlotOrder = (panelId: string, fromIndex: number, direction: "up" | "down") => {
    setPanels((prev) =>
      prev.map((p) => {
        if (p.id !== panelId) return p;
        const toIndex = direction === "up" ? fromIndex - 1 : fromIndex + 1;
        if (toIndex < 0 || toIndex >= p.assigned_team_ids.length) return p;

        const copy = [...p.assigned_team_ids];
        const temp = copy[fromIndex];
        copy[fromIndex] = copy[toIndex];
        copy[toIndex] = temp;
        return { ...p, assigned_team_ids: copy };
      })
    );
  };

  // Reassign squad from one room to another
  const handleMoveSlotToPanel = (teamId: string, targetPanelId: string) => {
    setPanels((prev) =>
      prev.map((p) => {
        if (p.id === targetPanelId) {
          if (p.assigned_team_ids.includes(teamId)) return p;
          return { ...p, assigned_team_ids: [...p.assigned_team_ids, teamId] };
        }
        return {
          ...p,
          assigned_team_ids: p.assigned_team_ids.filter((id) => id !== teamId),
        };
      })
    );
  };

  // Remove squad from room queue
  const handleRemoveSlotFromPanel = (panelId: string, teamId: string) => {
    // Removing a squad from its queue also deselects it, so it isn't silently re-placed on the next step.
    setSelectedTeamIds((prev) => prev.filter((id) => id !== teamId));
    setPanels((prev) =>
      prev.map((p) =>
        p.id === panelId
          ? {
              ...p,
              assigned_team_ids: p.assigned_team_ids.filter((id) => id !== teamId),
            }
          : p
      )
    );
  };

  // --- Step Validation ---
  const validateStep1 = (): boolean => {
    if (!roundName.trim()) {
      setStepErrors("Please provide a name for this judging round.");
      return false;
    }
    if (bufferSize < 1) {
      setStepErrors("Staging buffer depth must be at least 1 team.");
      return false;
    }
    if (slotDurationMins < 1) {
      setStepErrors("Slot duration must be at least 1 minute.");
      return false;
    }
    if (criteria.length === 0) {
      setStepErrors("Please specify at least one scoring criterion.");
      return false;
    }
    for (let i = 0; i < criteria.length; i++) {
      const c = criteria[i];
      if (!c.name.trim()) {
        setStepErrors(`Criterion #${i + 1} requires a valid title.`);
        return false;
      }
      if (!c.max_score || c.max_score < 1) {
        setStepErrors(`Criterion "${c.name}" must have a maximum score of at least 1.`);
        return false;
      }
      if (c.weight === undefined || c.weight <= 0) {
        setStepErrors(`Criterion "${c.name}" must have a positive weight multiplier (> 0).`);
        return false;
      }
    }
    return true;
  };

  const validateStep2 = (): boolean => {
    if (!enableRooms) return true;

    if (panels.length === 0) {
      setStepErrors("Please add at least one presentation room, or disable room setup to configure later.");
      return false;
    }

    for (let i = 0; i < panels.length; i++) {
      const p = panels[i];
      if (!p.name.trim()) {
        setStepErrors(`Room #${i + 1}: Name is required.`);
        return false;
      }
      if (!p.room.trim()) {
        setStepErrors(`Room #${i + 1}: Physical location or room name is required.`);
        return false;
      }
      if (p.judge_ids.length === 0) {
        setStepErrors(
          `Room "${p.name}" requires at least one assigned evaluator. If evaluators are not confirmed yet, uncheck "Configure presentation rooms now" to provision the round first.`
        );
        return false;
      }
    }
    return true;
  };

  const handleNext = () => {
    setStepErrors(null);

    if (step === 1) {
      if (!validateStep1()) return;
      setStep(2);
      return;
    }

    if (step === 2) {
      if (!validateStep2()) return;
      // If rooms are disabled, skip team slotting and go directly to review
      if (!enableRooms) {
        setStep(4);
        return;
      }
      // If teams are already selected and not distributed, auto-distribute
      handleAutoDistributeTeams();
      setStep(3);
      return;
    }

    if (step === 3) {
      setStep(4);
    }
  };

  // Final Submission
  const handleFinalSubmit = async () => {
    setStepErrors(null);
    try {
      const effectivePanels = enableRooms
        ? panels.map((p) => ({
            name: p.name.trim(),
            room: p.room.trim(),
            judge_ids: p.judge_ids,
            slots: p.assigned_team_ids.map((teamId, seq) => ({
              team_id: teamId,
              sequence_order: seq + 1,
              submission_links: {},
            })),
          }))
        : [];

      await onSubmitFullWorkflow({
        round: {
          name: roundName.trim(),
          buffer_size: Number(bufferSize) || 2,
          slot_duration_mins: Number(slotDurationMins) || 10,
          criteria: criteria.map((c, i) => ({
            name: c.name.trim(),
            description: c.description?.trim() || "",
            min_score: 0,
            max_score: Number(c.max_score) || 10,
            weight: Number(c.weight) || 1.0,
            order_index: i + 1,
          })),
        },
        panels: effectivePanels,
      });

      // Reset directly: handleClose() is gated on this render's stale isLoading, so it would skip the reset.
      resetWizard();
      onClose();
    } catch (err: unknown) {
      const msg =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : err && typeof err === "object" && "error" in err
          ? String((err as { error: string }).error)
          : "Failed to provision judging round workflow";
      setStepErrors(msg);
    }
  };

  // Metrics summary
  const totalWeight = criteria.reduce((sum, c) => sum + (Number(c.weight) || 0), 0);
  const totalMaxRaw = criteria.reduce((sum, c) => sum + (Number(c.max_score) || 0), 0);
  const allSlottedTeamIds = enableRooms ? panels.flatMap((p) => p.assigned_team_ids) : [];
  const maxRoomRuntime = enableRooms && panels.length > 0
    ? Math.max(...panels.map((p) => p.assigned_team_ids.length * slotDurationMins), 0)
    : 0;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Create Judging Round"
      description="Configure scoring rubrics, assign room panels, and schedule team presentation staging queues."
      className="max-w-4xl max-h-[92vh] flex flex-col p-6 sm:p-7"
      footer={
        <div className="flex items-center justify-between w-full">
          <div>
            {step > 1 && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setStepErrors(null);
                  if (step === 4 && !enableRooms) {
                    setStep(2);
                  } else {
                    setStep((prev) => (prev - 1) as 1 | 2 | 3 | 4);
                  }
                }}
                disabled={isLoading}
                className="gap-1.5 text-xs tactile-press"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                <span>Back</span>
              </Button>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleClose}
              disabled={isLoading}
              className="text-xs tactile-press"
            >
              Cancel
            </Button>

            {step === 1 && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  if (!validateStep1()) return;
                  setEnableRooms(false);
                  setStep(4);
                }}
                disabled={isLoading}
                className="text-xs tactile-press hidden sm:inline-flex"
                title="Create round with rubrics now, configure rooms and teams later"
              >
                Skip Rooms & Review
              </Button>
            )}

            {step < 4 ? (
              <Button
                variant="primary"
                size="sm"
                onClick={handleNext}
                disabled={isLoading}
                className="gap-1.5 text-xs tactile-press"
              >
                <span>Continue</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            ) : (
              <Button
                variant="primary"
                size="sm"
                onClick={handleFinalSubmit}
                isLoading={isLoading}
                className="gap-1.5 text-xs tactile-press shadow-sm"
              >
                <Award className="h-3.5 w-3.5" />
                <span>
                  {enableRooms && panels.length > 0
                    ? `Provision Round (${panels.length} Rooms, ${allSlottedTeamIds.length} Squads)`
                    : "Create Judging Round (Rubrics Only)"}
                </span>
              </Button>
            )}
          </div>
        </div>
      }
    >
      <div className="space-y-5 font-sans text-xs max-h-[66vh] overflow-y-auto pr-1">
        {/* Step Progression Bar (Apple-style visual wayfinding) */}
        <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-2">
          <nav aria-label="Creation Steps" className="grid grid-cols-4 gap-1.5">
            {[
              { num: 1, label: "Round & Rubrics", icon: Award },
              { num: 2, label: "Rooms & Evaluators", icon: DoorOpen },
              { num: 3, label: "Squad Scheduling", icon: Layers },
              { num: 4, label: "Review & Provision", icon: CheckCircle2 },
            ].map((s) => {
              const isCurrent = step === s.num;
              const isDone = step > s.num;
              const StepIcon = s.icon;

              return (
                <button
                  key={s.num}
                  type="button"
                  onClick={() => {
                    if (isLoading) return;
                    if (s.num === 1) setStep(1);
                    else if (s.num === 2 && validateStep1()) setStep(2);
                    else if (s.num === 3 && validateStep1() && validateStep2() && enableRooms) setStep(3);
                    else if (s.num === 4 && validateStep1() && validateStep2()) setStep(4);
                  }}
                  className={`flex items-center gap-2 px-2.5 py-1.5 rounded-[10px] text-left transition-all ${
                    isCurrent
                      ? "bg-[var(--card-solid)] border border-[var(--card-line)] shadow-xs text-[var(--ink-primary)] font-semibold"
                      : isDone
                      ? "text-[var(--color-blue)] hover:bg-[var(--card-solid)]/60 cursor-pointer"
                      : "text-[var(--ink-tertiary)] opacity-60 cursor-not-allowed"
                  }`}
                >
                  <span
                    className={`flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold shrink-0 ${
                      isDone
                        ? "bg-[var(--color-blue-dim)] text-[var(--color-blue)] border border-[var(--color-blue-border)]"
                        : isCurrent
                        ? "bg-[var(--color-dark)] text-[var(--ink-inverse)]"
                        : "bg-[var(--card-solid)] border border-[var(--card-line)] text-[var(--ink-tertiary)]"
                    }`}
                  >
                    {isDone ? <Check className="h-3 w-3" /> : s.num}
                  </span>
                  <div className="hidden sm:block truncate">
                    <span className="text-[11px] block truncate">{s.label}</span>
                  </div>
                  <StepIcon className="h-3.5 w-3.5 sm:hidden shrink-0 ml-auto" />
                </button>
              );
            })}
          </nav>
        </div>

        {/* Global Error Banner */}
        {stepErrors && (
          <div className="rounded-[14px] border border-[var(--color-coral-border)] bg-[var(--color-coral-dim)] p-3.5 text-[var(--color-coral)] font-medium flex items-start gap-2.5 shadow-xs">
            <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            <div className="flex-1">
              <span className="font-semibold block text-xs">Action Required</span>
              <p className="text-[11px] mt-0.5 leading-relaxed">{stepErrors}</p>
            </div>
            <button
              type="button"
              onClick={() => setStepErrors(null)}
              className="text-[var(--color-coral)] hover:opacity-80 p-0.5"
              aria-label="Dismiss error"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        )}

        {/* ========================================================= */}
        {/* STEP 1: Round Configuration & Scoring Criteria            */}
        {/* ========================================================= */}
        {step === 1 && (
          <div className="space-y-5 animate-in fade-in-50 duration-150">
            {/* Primary Details Card */}
            <div className="rounded-[18px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 sm:p-5 space-y-4 shadow-xs">
              <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-3">
                <div>
                  <h3 className="text-sm font-bold text-[var(--ink-primary)]">
                    Round Parameters
                  </h3>
                  <p className="text-[11px] text-[var(--ink-secondary)]">
                    Establish evaluation timing, queue depth, and identifying title.
                  </p>
                </div>
                <Badge variant="blue" size="sm">
                  Required Setup
                </Badge>
              </div>

              <Input
                label="Round Title *"
                placeholder="e.g. Round 1 — Preliminary Pitch & Technical Demo"
                value={roundName}
                onChange={(e) => setRoundName(e.target.value)}
                disabled={isLoading}
                required
              />

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5 pt-1">
                <Input
                  label="Queue Buffer Depth (Staged Squads) *"
                  type="number"
                  min={1}
                  max={10}
                  value={bufferSize}
                  onChange={(e) => setBufferSize(Math.max(1, Number(e.target.value) || 1))}
                  disabled={isLoading}
                  helperText="Number of squads staged on-deck outside the room."
                />

                <Input
                  label="Slot Duration (Minutes per Squad) *"
                  type="number"
                  min={1}
                  max={180}
                  value={slotDurationMins}
                  onChange={(e) => setSlotDurationMins(Math.max(1, Number(e.target.value) || 1))}
                  disabled={isLoading}
                  helperText="Allocated pitch, presentation, and Q&A runtime."
                />
              </div>
            </div>

            {/* Rubric Presets Picker */}
            <div className="rounded-[18px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 sm:p-5 space-y-3.5 shadow-xs">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-[var(--card-line)] pb-3">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-4 w-4 text-[var(--color-blue)]" />
                  <div>
                    <h3 className="text-sm font-bold text-[var(--ink-primary)]">
                      Scoring Rubric Templates
                    </h3>
                    <p className="text-[11px] text-[var(--ink-secondary)]">
                      Pick a battle-tested preset or customize individual scoring parameters.
                    </p>
                  </div>
                </div>

                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleAddCriterion}
                  className="h-7 text-xs px-2.5 gap-1.5 tactile-press shrink-0"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>Add Criterion</span>
                </Button>
              </div>

              {/* Preset Selector Chips */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                {RUBRIC_PRESETS.map((preset) => {
                  const isSelected = activePresetId === preset.id;
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      onClick={() => handleApplyPreset(preset)}
                      className={`p-3 rounded-[12px] border text-left transition-all tactile-press cursor-pointer ${
                        isSelected
                          ? "border-[var(--color-blue)] bg-[var(--color-blue-dim)] text-[var(--ink-primary)] ring-1 ring-[var(--color-blue)]"
                          : "border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:border-[var(--ink-tertiary)]"
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-bold text-xs text-[var(--ink-primary)]">
                          {preset.name}
                        </span>
                        {isSelected && (
                          <CheckCircle2 className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                        )}
                      </div>
                      <p className="text-[10px] text-[var(--ink-tertiary)] mt-1 truncate">
                        {preset.tagline}
                      </p>
                    </button>
                  );
                })}
              </div>

              <p className="text-[11px] leading-relaxed text-[var(--ink-secondary)] pt-1">
                Judges score each criterion from 0 up to its <strong>max score</strong>. The <strong>weight</strong> sets
                how much that criterion counts toward a team&apos;s total (a weight of 2 counts double).
              </p>

              {/* Criteria List */}
              <div className="space-y-2.5 pt-2">
                {criteria.map((c, i) => (
                  <div
                    key={i}
                    className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5 space-y-2.5 transition-all hover:border-[var(--ink-tertiary)]"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-1.5">
                        <span className="flex items-center justify-center h-4 w-4 rounded-full bg-[var(--card-solid)] border border-[var(--card-line)] text-[10px] font-bold text-[var(--ink-secondary)]">
                          {i + 1}
                        </span>
                        <span className="font-bold text-[11px] uppercase tracking-wider text-[var(--ink-secondary)]">
                          Criterion #{i + 1}
                        </span>
                      </div>

                      {criteria.length > 1 && (
                        <Button
                          variant="ghost"
                          size="sm"
                          type="button"
                          onClick={() => handleRemoveCriterion(i)}
                          title="Remove criterion"
                          className="h-6 w-6 p-0 text-[var(--ink-tertiary)] hover:text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] rounded-[6px]"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-12 gap-2.5">
                      <div className="sm:col-span-8">
                        <Input
                          label="Criterion name"
                          id={`criterion-${i}-name`}
                          placeholder="e.g. Technical Architecture"
                          value={c.name}
                          onChange={(e) =>
                            handleCriterionChange(i, "name", e.target.value)
                          }
                          className="h-9 text-xs"
                        />
                      </div>

                      <div className="sm:col-span-2">
                        <Input
                          label="Max score"
                          id={`criterion-${i}-max`}
                          type="number"
                          min={1}
                          max={100}
                          value={c.max_score}
                          onChange={(e) =>
                            handleCriterionChange(
                              i,
                              "max_score",
                              Math.max(1, Number(e.target.value) || 1)
                            )
                          }
                          className="h-9 text-xs font-mono"
                                                    placeholder="Max"
                        />
                      </div>

                      <div className="sm:col-span-2">
                        <Input
                          label="Weight"
                          id={`criterion-${i}-weight`}
                          type="number"
                          step="0.1"
                          min={0.1}
                          max={10}
                          value={c.weight}
                          onChange={(e) =>
                            handleCriterionChange(
                              i,
                              "weight",
                              Math.max(0.1, Number(e.target.value) || 1.0)
                            )
                          }
                          className="h-9 text-xs font-mono"
                                                    placeholder="Weight"
                        />
                      </div>
                    </div>

                    <Input
                      label="Guidance for judges (optional)"
                      id={`criterion-${i}-guidance`}
                      placeholder="What should judges look for?"
                      value={c.description || ""}
                      onChange={(e) =>
                        handleCriterionChange(i, "description", e.target.value)
                      }
                      className="h-7 text-[11px] text-[var(--ink-secondary)] bg-[var(--card-solid)]"
                    />
                  </div>
                ))}
              </div>

              {/* Rubric Summary Footer */}
              <div className="flex flex-wrap items-center justify-between gap-2 p-2.5 rounded-[12px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[11px] text-[var(--ink-secondary)]">
                <span>
                  Total Criteria: <strong className="text-[var(--ink-primary)]">{criteria.length}</strong>
                </span>
                <span>
                  Max Raw Points: <strong className="text-[var(--ink-primary)]">{totalMaxRaw} pts</strong>
                </span>
                <span>
                  Aggregated Weight: <strong className="text-[var(--color-blue)]">{totalWeight.toFixed(1)}x</strong>
                </span>
              </div>
            </div>
          </div>
        )}

        {/* ========================================================= */}
        {/* STEP 2: Rooms / Panels & Evaluator Assignments             */}
        {/* ========================================================= */}
        {step === 2 && (
          <div className="space-y-4 animate-in fade-in-50 duration-150">
            {/* Quick Toggle to Skip Room Orchestration if desired */}
            <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 flex items-center justify-between shadow-xs">
              <div>
                <label className="font-bold text-xs text-[var(--ink-primary)] flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={enableRooms}
                    onChange={(e) => setEnableRooms(e.target.checked)}
                    className="accent-[var(--color-blue)] h-4 w-4 rounded cursor-pointer"
                  />
                  <span>Establish presentation rooms & assign evaluators now</span>
                </label>
                <p className="text-[11px] text-[var(--ink-secondary)] mt-0.5 ml-6">
                  Recommended. If evaluators are pending, you can uncheck this to create the round draft first and add rooms later.
                </p>
              </div>

              {enableRooms && (
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleAddPanel}
                  className="h-7 text-xs px-2.5 gap-1.5 tactile-press shrink-0"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>Add Room</span>
                </Button>
              )}
            </div>

            {enableRooms ? (
              <div className="space-y-4">
                {/* Search & Filter Evaluators Bar */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-2.5">
                  <div className="relative flex-1">
                    <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
                    <Input
                      placeholder="Search evaluators by name or email..."
                      value={evaluatorSearchQuery}
                      onChange={(e) => setEvaluatorSearchQuery(e.target.value)}
                      className="pl-8 h-8 text-xs bg-[var(--card-solid)]"
                    />
                  </div>
                </div>

                {/* Rooms List */}
                <div className="space-y-3.5">
                  {panels.map((p, pIdx) => (
                    <div
                      key={p.id}
                      className="rounded-[18px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 sm:p-5 space-y-3.5 shadow-xs"
                    >
                      <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-2.5">
                        <div className="flex items-center gap-2">
                          <DoorOpen className="h-4 w-4 text-[var(--color-blue)]" />
                          <span className="font-bold text-xs text-[var(--ink-primary)]">
                            Room #{pIdx + 1}: {p.name || "Untitled"}
                          </span>
                          <Badge
                            variant={p.judge_ids.length > 0 ? "blue" : "coral"}
                            size="sm"
                          >
                            {p.judge_ids.length} Evaluators
                          </Badge>
                        </div>

                        {panels.length > 1 && (
                          <Button
                            variant="ghost"
                            size="sm"
                            type="button"
                            onClick={() => handleRemovePanel(p.id)}
                            className="h-6 text-[11px] px-2 text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] rounded-[6px]"
                          >
                            <Trash2 className="h-3 w-3 mr-1" />
                            <span>Remove</span>
                          </Button>
                        )}
                      </div>

                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                        <Input
                          label="Panel Name *"
                          placeholder="e.g. Panel Alpha (or AI Track)"
                          value={p.name}
                          onChange={(e) => {
                            const val = e.target.value;
                            setPanels((prev) =>
                              prev.map((item) =>
                                item.id === p.id ? { ...item, name: val } : item
                              )
                            );
                          }}
                          className="h-8 text-xs"
                        />

                        <Input
                          label="Physical or Virtual Location *"
                          placeholder="e.g. Room 101 — Main Hall / Zoom A"
                          value={p.room}
                          onChange={(e) => {
                            const val = e.target.value;
                            setPanels((prev) =>
                              prev.map((item) =>
                                item.id === p.id ? { ...item, room: val } : item
                              )
                            );
                          }}
                          className="h-8 text-xs"
                        />
                      </div>

                      {/* Evaluators Checkable Grid */}
                      <div className="space-y-1.5 pt-1">
                        <div className="flex items-center justify-between text-[11px]">
                          <span className="font-semibold text-[var(--ink-secondary)]">
                            Assign Evaluators to this Room ({p.judge_ids.length} Selected) *
                          </span>
                          <span className="text-[10px] text-[var(--ink-tertiary)]">
                            Click to toggle judge assignment
                          </span>
                        </div>

                        {usersLoading ? (
                          <div className="p-3 text-center text-xs text-[var(--ink-tertiary)]">
                            Loading evaluators...
                          </div>
                        ) : filteredEvaluators.length === 0 ? (
                          <div className="p-3 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-center text-xs text-[var(--ink-secondary)]">
                            No evaluators match the filter. Create users with the judge role first.
                          </div>
                        ) : (
                          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 max-h-44 overflow-y-auto p-1.5 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
                            {filteredEvaluators
                              // A judge can only sit on one panel per round: hide those picked for other rooms.
                              .filter(
                                (ev) =>
                                  p.judge_ids.includes(ev.id) ||
                                  !panels.some((o) => o.id !== p.id && o.judge_ids.includes(ev.id))
                              )
                              .map((ev) => {
                              const isAssigned = p.judge_ids.includes(ev.id);
                              return (
                                <button
                                  key={ev.id}
                                  type="button"
                                  onClick={() => handleToggleJudgeForPanel(p.id, ev.id)}
                                  className={`flex items-center gap-2 p-2 rounded-[10px] border text-left transition-all tactile-press cursor-pointer select-none ${
                                    isAssigned
                                      ? "border-[var(--color-blue)] bg-[var(--card-solid)] shadow-xs ring-1 ring-[var(--color-blue)]/50"
                                      : "border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-secondary)] hover:border-[var(--ink-tertiary)]"
                                  }`}
                                >
                                  <div
                                    className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold shrink-0 ${
                                      isAssigned
                                        ? "bg-[var(--color-blue)] text-white"
                                        : "bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-secondary)]"
                                    }`}
                                  >
                                    {isAssigned ? (
                                      <Check className="h-3.5 w-3.5" />
                                    ) : (
                                      ev.name.slice(0, 1).toUpperCase()
                                    )}
                                  </div>

                                  <div className="truncate flex-1">
                                    <div className="text-[11px] font-bold text-[var(--ink-primary)] truncate">
                                      {ev.name}
                                    </div>
                                    <div className="text-[10px] text-[var(--ink-tertiary)] truncate">
                                      {ev.email}
                                    </div>
                                  </div>

                                  <Badge
                                    variant={ev.role === "judge" ? "blue" : "subtle"}
                                    size="sm"
                                    className="text-[9px] px-1 py-0 h-4"
                                  >
                                    {ev.role}
                                  </Badge>
                                </button>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-6 text-center space-y-2">
                <HelpCircle className="h-6 w-6 text-[var(--ink-tertiary)] mx-auto" />
                <h4 className="font-bold text-xs text-[var(--ink-primary)]">
                  Room Orchestration Deferred
                </h4>
                <p className="text-[11px] text-[var(--ink-secondary)] max-w-md mx-auto">
                  You can establish the judging round rubrics and parameters now. After creation, rooms and evaluator panels can be added at any time before activating the round.
                </p>
              </div>
            )}
          </div>
        )}

        {/* ========================================================= */}
        {/* STEP 3: Squad Selection & Staging Queue Scheduling        */}
        {/* ========================================================= */}
        {step === 3 && (
          <div className="space-y-4 animate-in fade-in-50 duration-150">
            {/* Squads Toolbar */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 pb-2 border-b border-[var(--card-line)]">
              <div>
                <h3 className="text-sm font-bold text-[var(--ink-primary)] flex items-center gap-1.5">
                  <Layers className="h-4 w-4 text-[var(--color-blue)]" />
                  <span>Participating Squads ({selectedTeamIds.length} Selected)</span>
                </h3>
                <p className="text-[11px] text-[var(--ink-secondary)]">
                  Pick the teams decided so far and arrange their room queues. You don&apos;t need to assign every team now; add the rest later from the round&apos;s actions.
                </p>
              </div>

              <div className="flex items-center gap-1.5 flex-wrap">
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleSelectAllTeams}
                  className="h-7 text-xs px-2.5 tactile-press"
                >
                  Select All
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleClearTeams}
                  disabled={selectedTeamIds.length === 0 && !teamSearchQuery}
                  className="h-7 text-xs px-2.5 tactile-press"
                >
                  Clear
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  onClick={handleAutoDistributeTeams}
                  disabled={unassignedTeamIds.length === 0 || panels.length === 0}
                  className="h-7 text-xs px-3 gap-1.5 tactile-press shadow-xs"
                >
                  <Sparkles className="h-3 w-3" />
                  <span>
                    Auto-Distribute{unassignedTeamIds.length > 0 ? ` ${unassignedTeamIds.length} Unassigned` : ""}
                  </span>
                </Button>
              </div>
            </div>

            {/* Team Picker Filter */}
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
              <Input
                placeholder="Search registered squads..."
                value={teamSearchQuery}
                onChange={(e) => setTeamSearchQuery(e.target.value)}
                className="pl-8 h-8 text-xs"
              />
            </div>

            {/* Team Picker Grid */}
            {teamsLoading ? (
              <div className="text-center p-4 text-xs text-[var(--ink-tertiary)]">
                Loading squads...
              </div>
            ) : availableTeams.length === 0 ? (
              <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-4 text-center text-xs text-[var(--ink-secondary)]">
                No squads registered yet. You can create squads in the Teams directory.
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 max-h-48 overflow-y-auto rounded-[14px] border border-[var(--card-line)] p-2.5 bg-[var(--card-solid-subtle)]">
                {filteredTeams.map((t) => {
                  const isSelected = selectedTeamIds.includes(t.id);
                  return (
                    <label
                      key={t.id}
                      className={`flex items-center gap-2.5 p-2 rounded-[10px] border cursor-pointer select-none text-xs transition-all tactile-press ${
                        isSelected
                          ? "border-[var(--color-blue)] bg-[var(--card-solid)] shadow-xs font-semibold ring-1 ring-[var(--color-blue)]/50"
                          : "border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-secondary)] hover:border-[var(--ink-tertiary)]"
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => handleToggleTeam(t.id)}
                        className="accent-[var(--color-blue)] h-4 w-4 rounded"
                      />
                      <div className="truncate flex-1">
                        <div className="font-bold text-[var(--ink-primary)] truncate">
                          {t.name}
                        </div>
                        <div className="text-[10px] text-[var(--ink-tertiary)] truncate">
                          {t.member_count} Members • {t.leader_name ? `Lead: ${t.leader_name}` : "No Leader"}
                        </div>
                      </div>
                    </label>
                  );
                })}
              </div>
            )}

            {/* Visual Staging Queues Per Room */}
            <div className="space-y-3 pt-2 border-t border-[var(--card-line)]">
              <div className="flex items-center justify-between">
                <span className="font-bold text-xs uppercase tracking-wider text-[var(--ink-primary)] flex items-center gap-1.5">
                  <Clock className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                  <span>Sequential Slot Scheduling Across Rooms</span>
                </span>
                <span className="text-[11px] text-[var(--ink-tertiary)]">
                  Est. runtime per room: ~{slotDurationMins}m / squad
                </span>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {panels.map((p) => {
                  const roomEstTime = p.assigned_team_ids.length * slotDurationMins;
                  return (
                    <div
                      key={p.id}
                      className="rounded-[16px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid)] space-y-2.5 shadow-xs"
                    >
                      <div className="flex justify-between items-center text-xs font-bold border-b border-[var(--card-line)] pb-2">
                        <div>
                          <span className="text-[var(--ink-primary)]">{p.name}</span>
                          <span className="text-[10px] text-[var(--ink-tertiary)] block">
                            {p.room} • {p.judge_ids.length} Evaluators
                          </span>
                        </div>
                        <div className="text-right">
                          <Badge variant="subtle" size="sm">
                            {p.assigned_team_ids.length} Squads
                          </Badge>
                          <span className="text-[10px] text-[var(--ink-tertiary)] block font-mono">
                            ~{roomEstTime} mins
                          </span>
                        </div>
                      </div>

                      <div className="space-y-1.5 max-h-40 overflow-y-auto pr-0.5">
                        {p.assigned_team_ids.length === 0 ? (
                          <div className="p-3 text-center text-xs italic text-[var(--ink-tertiary)] border border-dashed border-[var(--card-line)] rounded-[10px]">
                            No squads scheduled in this room. Click &quot;Auto-Distribute&quot; or assign squads.
                          </div>
                        ) : (
                          p.assigned_team_ids.map((teamId, seq) => {
                            const teamObj = availableTeams.find((t) => t.id === teamId);
                            const startTimeOffset = seq * slotDurationMins;
                            return (
                              <div
                                key={teamId}
                                className="flex items-center justify-between p-2 rounded-[8px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-xs group"
                              >
                                <div className="flex items-center gap-2 truncate">
                                  <span className="font-mono text-[10px] font-bold px-1.5 py-0.5 rounded bg-[var(--card-solid)] border border-[var(--card-line)] text-[var(--ink-secondary)]">
                                    #{seq + 1}
                                  </span>
                                  <div className="truncate">
                                    <span className="font-bold text-[var(--ink-primary)] block truncate">
                                      {teamObj?.name || teamId}
                                    </span>
                                    <span className="text-[10px] text-[var(--ink-tertiary)] font-mono">
                                      +{startTimeOffset}m from start
                                    </span>
                                  </div>
                                </div>

                                <div className="flex items-center gap-1 shrink-0">
                                  {/* Move Up */}
                                  <button
                                    type="button"
                                    onClick={() => handleMoveSlotOrder(p.id, seq, "up")}
                                    disabled={seq === 0}
                                    className="p-1 rounded text-[var(--ink-tertiary)] hover:text-[var(--ink-primary)] disabled:opacity-30 disabled:cursor-not-allowed"
                                    title="Move earlier"
                                  >
                                    <ArrowUp className="h-3 w-3" />
                                  </button>

                                  {/* Move Down */}
                                  <button
                                    type="button"
                                    onClick={() => handleMoveSlotOrder(p.id, seq, "down")}
                                    disabled={seq === p.assigned_team_ids.length - 1}
                                    className="p-1 rounded text-[var(--ink-tertiary)] hover:text-[var(--ink-primary)] disabled:opacity-30 disabled:cursor-not-allowed"
                                    title="Move later"
                                  >
                                    <ArrowDown className="h-3 w-3" />
                                  </button>

                                  {/* Move to other panel if multiple exist */}
                                  {panels.length > 1 && (
                                    <select
                                      value=""
                                      onChange={(e) => {
                                        if (e.target.value) {
                                          handleMoveSlotToPanel(teamId, e.target.value);
                                        }
                                      }}
                                      className="h-5 text-[10px] rounded border border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-secondary)]"
                                      title="Move to another room"
                                    >
                                      <option value="">Move...</option>
                                      {panels
                                        .filter((other) => other.id !== p.id)
                                        .map((other) => (
                                          <option key={other.id} value={other.id}>
                                            {other.name}
                                          </option>
                                        ))}
                                    </select>
                                  )}

                                  {/* Remove from panel */}
                                  <button
                                    type="button"
                                    onClick={() => handleRemoveSlotFromPanel(p.id, teamId)}
                                    className="p-1 rounded text-[var(--ink-tertiary)] hover:text-[var(--color-coral)]"
                                    title="Remove from room"
                                  >
                                    <Trash2 className="h-3 w-3" />
                                  </button>
                                </div>
                              </div>
                            );
                          })
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        )}

        {/* ========================================================= */}
        {/* STEP 4: Review & Finalize Provisioning                    */}
        {/* ========================================================= */}
        {step === 4 && (
          <div className="space-y-4 animate-in fade-in-50 duration-150">
            {/* Success Readiness Header */}
            <div className="rounded-[18px] border border-[var(--color-emerald-border)] bg-[var(--color-emerald-dim)] p-4 flex items-start gap-3 shadow-xs">
              <CheckCircle2 className="h-5 w-5 text-[var(--color-emerald)] shrink-0 mt-0.5" />
              <div>
                <h4 className="font-bold text-xs text-[var(--color-emerald)] uppercase tracking-wider">
                  Ready to Provision Judging Round
                </h4>
                <p className="text-[11px] text-[var(--color-emerald)] mt-0.5 leading-relaxed">
                  Submitting will create the judging round, register evaluation criteria, provision room panels, and schedule team staging sequences.
                </p>
              </div>
            </div>

            {/* Metrics Breakdown Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
              <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3">
                <span className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] block">
                  Slot Duration
                </span>
                <span className="text-base font-bold font-mono text-[var(--ink-primary)] mt-0.5 block">
                  {slotDurationMins} mins
                </span>
                <span className="text-[10px] text-[var(--ink-secondary)]">per team</span>
              </div>

              <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3">
                <span className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] block">
                  Queue Buffer
                </span>
                <span className="text-base font-bold font-mono text-[var(--ink-primary)] mt-0.5 block">
                  {bufferSize} teams
                </span>
                <span className="text-[10px] text-[var(--ink-secondary)]">on-deck staged</span>
              </div>

              <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3">
                <span className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] block">
                  Evaluation Rubrics
                </span>
                <span className="text-base font-bold font-mono text-[var(--color-blue)] mt-0.5 block">
                  {criteria.length} Criteria
                </span>
                <span className="text-[10px] text-[var(--ink-secondary)]">{totalWeight.toFixed(1)}x aggregated</span>
              </div>

              <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3">
                <span className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] block">
                  Rooms & Squads
                </span>
                <span className="text-base font-bold font-mono text-[var(--ink-primary)] mt-0.5 block">
                  {enableRooms ? `${panels.length} Rooms` : "Deferred"}
                </span>
                <span className="text-[10px] text-[var(--ink-secondary)]">
                  {enableRooms ? `${allSlottedTeamIds.length} squads slotted (~${maxRoomRuntime}m)` : "Configure later"}
                </span>
              </div>
            </div>

            {/* Structured Summary Table */}
            <div className="rounded-[16px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)] overflow-hidden text-xs shadow-xs">
              <div className="flex justify-between p-3.5 bg-[var(--card-solid-subtle)]">
                <span className="text-[var(--ink-secondary)] font-medium">Round Title</span>
                <strong className="text-[var(--ink-primary)] text-right">{roundName}</strong>
              </div>

              <div className="p-3.5 space-y-2">
                <span className="text-[11px] font-bold text-[var(--ink-secondary)] uppercase tracking-wider block">
                  Evaluation Rubrics & Multipliers:
                </span>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  {criteria.map((c, i) => (
                    <div
                      key={i}
                      className="p-2 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] flex items-center justify-between text-[11px]"
                    >
                      <span className="font-semibold text-[var(--ink-primary)] truncate mr-2">
                        {c.name}
                      </span>
                      <div className="shrink-0 flex items-center gap-1.5 font-mono text-[10px]">
                        <span className="text-[var(--ink-secondary)]">{c.max_score} pts</span>
                        <Badge variant="blue" size="sm" className="h-4 px-1">
                          {c.weight}x
                        </Badge>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {enableRooms && panels.length > 0 && (
                <div className="p-3.5 space-y-2">
                  <span className="text-[11px] font-bold text-[var(--ink-secondary)] uppercase tracking-wider block">
                    Room Schedule & Allocations:
                  </span>
                  <div className="space-y-1.5">
                    {panels.map((p, i) => (
                      <div
                        key={i}
                        className="rounded-[10px] border border-[var(--card-line)] p-2.5 bg-[var(--card-solid-subtle)] flex flex-col sm:flex-row sm:items-center justify-between gap-1.5 text-[11px]"
                      >
                        <div>
                          <strong className="text-[var(--ink-primary)]">{p.name}</strong>{" "}
                          <span className="text-[var(--ink-secondary)]">({p.room})</span> —{" "}
                          <span className="text-[var(--color-blue)] font-medium">
                            {p.judge_ids.length} Evaluators
                          </span>
                        </div>
                        <div className="font-mono text-[var(--ink-primary)] font-semibold">
                          {p.assigned_team_ids.length} Squads Slotted (~{p.assigned_team_ids.length * slotDurationMins}m)
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
