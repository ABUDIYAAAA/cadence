"use client";

import React, { useState } from "react";
import {
  Award,
  Plus,
  Trash2,
  Layers,
  DoorOpen,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
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

interface PanelDraft {
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

const DEFAULT_CRITERIA: CreateCriteriaRequest[] = [
  {
    name: "Technical Execution & Architecture",
    description: "Code quality, technical complexity, robust implementation",
    max_score: 10,
    weight: 1.0,
    order_index: 1,
  },
  {
    name: "Innovation & Problem Solving",
    description: "Originality of idea and impact of the solution",
    max_score: 10,
    weight: 1.0,
    order_index: 2,
  },
  {
    name: "UI / UX & Prototype Polish",
    description: "Visual design, responsiveness, and usability",
    max_score: 10,
    weight: 1.0,
    order_index: 3,
  },
  {
    name: "Presentation & Feasibility",
    description: "Pitch clarity and roadmap viability",
    max_score: 10,
    weight: 1.0,
    order_index: 4,
  },
];

export function RoundCreateDialog({
  open,
  onClose,
  onSubmitFullWorkflow,
  isLoading,
}: RoundCreateDialogProps) {
  // Step navigation: 1 = Round & Criteria, 2 = Rooms & Judges, 3 = Teams & Slot Allocation, 4 = Review
  const [step, setStep] = useState<1 | 2 | 3 | 4>(1);

  // Queries for dropdowns
  const { data: usersData, isLoading: usersLoading } = useUsers({
    role: "judge",
    limit: 100,
  });
  const { data: teamsData, isLoading: teamsLoading } = useTeams({ limit: 100 });

  // Evaluators list (strictly users with role 'judge', excluding admins)
  const availableJudges = (usersData?.data || []).filter(
    (u) => u.role === "judge"
  );

  // Registered teams
  const availableTeams = teamsData?.data || [];

  // Form State - Step 1: Round & Criteria
  const [roundName, setRoundName] = useState("");
  const [bufferSize, setBufferSize] = useState(2);
  const [slotDurationMins, setSlotDurationMins] = useState(10);
  const [criteria, setCriteria] = useState<CreateCriteriaRequest[]>(DEFAULT_CRITERIA);

  // Form State - Step 2: Panels & Rooms
  const [panels, setPanels] = useState<PanelDraft[]>([
    { name: "Panel Alpha", room: "Room 101 - Main Lab", judge_ids: [], assigned_team_ids: [] },
  ]);

  // Form State - Step 3: Selected Teams to put in round
  const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([]);

  // Validation errors
  const [stepErrors, setStepErrors] = useState<string | null>(null);

  const resetWizard = () => {
    setStep(1);
    setRoundName("");
    setBufferSize(2);
    setSlotDurationMins(10);
    setCriteria(DEFAULT_CRITERIA);
    setPanels([
      { name: "Panel Alpha", room: "Room 101 - Main Lab", judge_ids: [], assigned_team_ids: [] },
    ]);
    setSelectedTeamIds([]);
    setStepErrors(null);
  };

  const handleClose = () => {
    resetWizard();
    onClose();
  };

  // Step 1: Add/Remove criterion
  const handleAddCriterion = () => {
    setCriteria((prev) => [
      ...prev,
      {
        name: `Criterion ${prev.length + 1}`,
        description: "",
        max_score: 10,
        weight: 1.0,
        order_index: prev.length + 1,
      },
    ]);
  };

  const handleRemoveCriterion = (index: number) => {
    setCriteria((prev) => prev.filter((_, i) => i !== index));
  };

  const handleCriterionChange = (
    index: number,
    field: keyof CreateCriteriaRequest,
    val: string | number
  ) => {
    setCriteria((prev) =>
      prev.map((c, i) => (i === index ? { ...c, [field]: val } : c))
    );
  };

  // Step 2: Add/Remove panel
  const handleAddPanel = () => {
    setPanels((prev) => [
      ...prev,
      {
        name: `Panel ${String.fromCharCode(65 + prev.length)}`,
        room: `Room ${101 + prev.length}`,
        judge_ids: [],
        assigned_team_ids: [],
      },
    ]);
  };

  const handleRemovePanel = (index: number) => {
    setPanels((prev) => prev.filter((_, i) => i !== index));
  };

  const handleToggleJudgeForPanel = (panelIndex: number, judgeId: string) => {
    setPanels((prev) =>
      prev.map((p, i) => {
        if (i !== panelIndex) return p;
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

  // Step 3: Team selection & distribution
  const handleToggleTeam = (teamId: string) => {
    setSelectedTeamIds((prev) =>
      prev.includes(teamId)
        ? prev.filter((id) => id !== teamId)
        : [...prev, teamId]
    );
  };

  const handleSelectAllTeams = () => {
    setSelectedTeamIds(availableTeams.map((t) => t.id));
  };

  const handleClearTeams = () => {
    setSelectedTeamIds([]);
  };

  // Auto-distribute selected teams across created panels
  const autoDistributeTeams = () => {
    if (panels.length === 0 || selectedTeamIds.length === 0) return;

    const newPanels = panels.map((p) => ({ ...p, assigned_team_ids: [] as string[] }));

    selectedTeamIds.forEach((teamId, index) => {
      const panelIndex = index % panels.length;
      newPanels[panelIndex].assigned_team_ids.push(teamId);
    });

    setPanels(newPanels);
  };

  // Validation for moving forward
  const handleNext = () => {
    setStepErrors(null);

    if (step === 1) {
      if (!roundName.trim()) {
        setStepErrors("Round name is required.");
        return;
      }
      if (criteria.length === 0) {
        setStepErrors("Please specify at least one scoring criterion.");
        return;
      }
      for (const c of criteria) {
        if (!c.name.trim()) {
          setStepErrors("All criteria must have a title.");
          return;
        }
      }
      setStep(2);
      return;
    }

    if (step === 2) {
      if (panels.length === 0) {
        setStepErrors("At least one room/panel is required.");
        return;
      }
      for (let i = 0; i < panels.length; i++) {
        if (!panels[i].name.trim() || !panels[i].room.trim()) {
          setStepErrors(`Panel ${i + 1}: Name and Room are required.`);
          return;
        }
        if (panels[i].judge_ids.length === 0) {
          setStepErrors(`Panel "${panels[i].name}" must have at least one assigned judge.`);
          return;
        }
      }
      // If teams were not distributed yet, auto-distribute default
      if (selectedTeamIds.length > 0) {
        autoDistributeTeams();
      }
      setStep(3);
      return;
    }

    if (step === 3) {
      if (selectedTeamIds.length === 0) {
        setStepErrors("Please select at least one team to put into this judging round.");
        return;
      }
      // Ensure all selected teams are distributed into panels
      const allAssigned = panels.flatMap((p) => p.assigned_team_ids);
      const unassigned = selectedTeamIds.filter((id) => !allAssigned.includes(id));
      if (unassigned.length > 0) {
        autoDistributeTeams();
      }
      setStep(4);
    }
  };

  // Final submission of entire orchestrated workflow
  const handleFinalSubmit = async () => {
    try {
      const formattedPanels = panels.map((p) => ({
        name: p.name.trim(),
        room: p.room.trim(),
        judge_ids: p.judge_ids,
        slots: p.assigned_team_ids.map((teamId, seq) => ({
          team_id: teamId,
          sequence_order: seq + 1,
          submission_links: {},
        })),
      }));

      await onSubmitFullWorkflow({
        round: {
          name: roundName.trim(),
          buffer_size: Number(bufferSize) || 2,
          slot_duration_mins: Number(slotDurationMins) || 10,
          criteria: criteria.map((c, i) => ({
            name: c.name.trim(),
            description: c.description || "",
            max_score: Number(c.max_score) || 10,
            weight: Number(c.weight) || 1.0,
            order_index: i + 1,
          })),
        },
        panels: formattedPanels,
      });

      handleClose();
    } catch (err: unknown) {
      setStepErrors(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to provision judging round"
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Create Judging Round Workflow"
      description={`Step ${step} of 4: ${
        step === 1
          ? "Round Parameters & Scoring Criteria"
          : step === 2
          ? "Rooms, Panels & Judge Assignments"
          : step === 3
          ? "Choose Teams & Slot Sequences"
          : "Review & Provision Setup"
      }`}
      footer={
        <div className="flex items-center justify-between w-full">
          <div>
            {step > 1 && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setStep((prev) => (prev - 1) as 1 | 2 | 3 | 4)}
                disabled={isLoading}
                className="gap-1 text-xs"
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
              className="text-xs"
            >
              Cancel
            </Button>

            {step < 4 ? (
              <Button
                variant="primary"
                size="sm"
                onClick={handleNext}
                className="gap-1 text-xs"
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
                className="gap-1.5 text-xs"
              >
                <Award className="h-3.5 w-3.5" />
                <span>Provision Round & Rooms</span>
              </Button>
            )}
          </div>
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs max-h-[68vh] overflow-y-auto pr-1">
        {stepErrors && (
          <div className="border border-black p-2.5 bg-neutral-100 font-bold text-neutral-900">
            ! {stepErrors}
          </div>
        )}

        {/* STEP 1: Round Configuration & Scoring Criteria */}
        {step === 1 && (
          <div className="space-y-4">
            <Input
              label="Round Name *"
              placeholder="e.g. Round 1 - Preliminary Pitch & Technical Demo"
              value={roundName}
              onChange={(e) => setRoundName(e.target.value)}
              disabled={isLoading}
              required
            />

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Input
                label="Queue Buffer Size (Staging Depth) *"
                type="number"
                min={1}
                max={10}
                value={bufferSize}
                onChange={(e) => setBufferSize(Number(e.target.value))}
                disabled={isLoading}
                helperText="Number of teams queued and staged outside the room before presenting."
              />

              <Input
                label="Slot Duration (Minutes) *"
                type="number"
                min={1}
                max={120}
                value={slotDurationMins}
                onChange={(e) => setSlotDurationMins(Number(e.target.value))}
                disabled={isLoading}
                helperText="Estimated evaluation & pitch time per team."
              />
            </div>

            {/* Criteria List */}
            <div className="border border-neutral-300 p-3 bg-neutral-50 space-y-3">
              <div className="flex items-center justify-between border-b border-neutral-200 pb-2">
                <span className="font-bold text-black uppercase text-[11px] flex items-center gap-1.5">
                  <Award className="h-3.5 w-3.5" />
                  <span>Evaluation Criteria ({criteria.length})</span>
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleAddCriterion}
                  className="h-6 text-[10px] px-2 gap-1"
                >
                  <Plus className="h-3 w-3" />
                  <span>Add Criterion</span>
                </Button>
              </div>

              <div className="space-y-2">
                {criteria.map((c, i) => (
                  <div
                    key={i}
                    className="border border-neutral-300 bg-white p-2.5 space-y-2"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-bold text-[10px] uppercase text-neutral-500">
                        Criterion #{i + 1}
                      </span>
                      {criteria.length > 1 && (
                        <Button
                          variant="ghost"
                          size="sm"
                          type="button"
                          onClick={() => handleRemoveCriterion(i)}
                          className="h-5 w-5 p-0 text-neutral-400 hover:text-black"
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      )}
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                      <div className="sm:col-span-2">
                        <Input
                          placeholder="Criterion Name"
                          value={c.name}
                          onChange={(e) =>
                            handleCriterionChange(i, "name", e.target.value)
                          }
                          className="h-8 text-xs"
                        />
                      </div>
                      <div className="flex gap-2">
                        <Input
                          placeholder="Max Score"
                          type="number"
                          min={1}
                          value={c.max_score}
                          onChange={(e) =>
                            handleCriterionChange(i, "max_score", Number(e.target.value))
                          }
                          className="h-8 text-xs w-20"
                          title="Max Score"
                        />
                        <Input
                          placeholder="Weight"
                          type="number"
                          step="0.1"
                          min={0}
                          value={c.weight}
                          onChange={(e) =>
                            handleCriterionChange(i, "weight", Number(e.target.value))
                          }
                          className="h-8 text-xs w-20"
                          title="Weight Multiplier"
                        />
                      </div>
                    </div>

                    <Input
                      placeholder="Description & rubrics (optional)"
                      value={c.description || ""}
                      onChange={(e) =>
                        handleCriterionChange(i, "description", e.target.value)
                      }
                      className="h-7 text-[11px] text-neutral-600"
                    />
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* STEP 2: Rooms / Panels & Judges */}
        {step === 2 && (
          <div className="space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-neutral-200">
              <span className="font-bold text-black uppercase text-[11px] flex items-center gap-1.5">
                <DoorOpen className="h-3.5 w-3.5" />
                <span>Panels & Presentation Rooms ({panels.length})</span>
              </span>
              <Button
                variant="outline"
                size="sm"
                type="button"
                onClick={handleAddPanel}
                className="h-6 text-[10px] px-2 gap-1"
              >
                <Plus className="h-3 w-3" />
                <span>Add Panel Room</span>
              </Button>
            </div>

            <div className="space-y-3">
              {panels.map((p, pIdx) => (
                <div
                  key={pIdx}
                  className="border border-neutral-300 bg-white p-3 space-y-3"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-xs uppercase text-black">
                      Room #{pIdx + 1}
                    </span>
                    {panels.length > 1 && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleRemovePanel(pIdx)}
                        className="h-5 w-5 p-0 text-neutral-400 hover:text-black"
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    )}
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    <Input
                      label="Panel Name *"
                      placeholder="e.g. Panel Alpha"
                      value={p.name}
                      onChange={(e) => {
                        const val = e.target.value;
                        setPanels((prev) =>
                          prev.map((item, idx) =>
                            idx === pIdx ? { ...item, name: val } : item
                          )
                        );
                      }}
                      className="h-8 text-xs"
                    />

                    <Input
                      label="Physical Room Location *"
                      placeholder="e.g. Room 204 - Science Wing"
                      value={p.room}
                      onChange={(e) => {
                        const val = e.target.value;
                        setPanels((prev) =>
                          prev.map((item, idx) =>
                            idx === pIdx ? { ...item, room: val } : item
                          )
                        );
                      }}
                      className="h-8 text-xs"
                    />
                  </div>

                  {/* Assign Judges to this Panel */}
                  <div className="space-y-1 pt-1 border-t border-neutral-100">
                    <label className="block text-[10px] uppercase font-bold text-neutral-600">
                      Assign Evaluators to this Room ({p.judge_ids.length} Selected) *
                    </label>

                    {usersLoading ? (
                      <div className="text-neutral-400 text-xs">Loading registered judges...</div>
                    ) : availableJudges.length === 0 ? (
                      <div className="text-neutral-500 text-xs italic p-2 border border-neutral-200 bg-neutral-50">
                        No accounts with role &apos;judge&apos; found. You can promote users in the Users Directory.
                      </div>
                    ) : (
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-1.5 max-h-36 overflow-y-auto border border-neutral-200 p-2 bg-neutral-50">
                        {availableJudges.map((j) => {
                          const isAssigned = p.judge_ids.includes(j.id);
                          return (
                            <label
                              key={j.id}
                              className={`flex items-center gap-2 p-1.5 border cursor-pointer select-none text-xs ${
                                isAssigned
                                  ? "border-black bg-white font-bold"
                                  : "border-neutral-200 bg-white/70 text-neutral-700"
                              }`}
                            >
                              <input
                                type="checkbox"
                                checked={isAssigned}
                                onChange={() => handleToggleJudgeForPanel(pIdx, j.id)}
                                className="accent-black h-3.5 w-3.5 rounded-none"
                              />
                              <div className="truncate">
                                <div>{j.name}</div>
                                <div className="text-[10px] text-neutral-400 font-mono">
                                  {j.email}
                                </div>
                              </div>
                            </label>
                          );
                        })}
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* STEP 3: Choose Teams to put in round */}
        {step === 3 && (
          <div className="space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-2 border-b border-neutral-200">
              <div>
                <span className="font-bold text-black uppercase text-[11px] flex items-center gap-1.5">
                  <Layers className="h-3.5 w-3.5" />
                  <span>Participating Squads ({selectedTeamIds.length} Selected)</span>
                </span>
                <p className="text-[10px] text-neutral-500 mt-0.5">
                  Choose which teams will compete in this round.
                </p>
              </div>

              <div className="flex items-center gap-1.5">
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleSelectAllTeams}
                  className="h-6 text-[10px] px-2"
                >
                  Select All
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleClearTeams}
                  className="h-6 text-[10px] px-2"
                >
                  Clear
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  onClick={autoDistributeTeams}
                  className="h-6 text-[10px] px-2"
                >
                  Auto-Distribute to Rooms
                </Button>
              </div>
            </div>

            {teamsLoading ? (
              <div className="text-neutral-400 text-xs">Loading registered teams...</div>
            ) : availableTeams.length === 0 ? (
              <div className="text-neutral-500 text-xs italic p-4 border border-neutral-200 bg-neutral-50">
                No teams registered yet. Create teams first in the Teams tab.
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-52 overflow-y-auto border border-neutral-200 p-2 bg-neutral-50">
                {availableTeams.map((t) => {
                  const isSelected = selectedTeamIds.includes(t.id);
                  return (
                    <label
                      key={t.id}
                      className={`flex items-center gap-2 p-2 border cursor-pointer select-none text-xs ${
                        isSelected
                          ? "border-black bg-white font-bold"
                          : "border-neutral-200 bg-white/70 text-neutral-700"
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => handleToggleTeam(t.id)}
                        className="accent-black h-4 w-4 rounded-none"
                      />
                      <div className="truncate flex-1">
                        <div className="font-bold text-black">{t.name}</div>
                        <div className="text-[10px] text-neutral-500">
                          {t.member_count} Members • {t.leader_name ? `Lead: ${t.leader_name}` : "No Leader"}
                        </div>
                      </div>
                    </label>
                  );
                })}
              </div>
            )}

            {/* Room Queue Distribution Preview */}
            <div className="space-y-2 pt-2 border-t border-neutral-200">
              <span className="font-bold text-black uppercase text-[11px]">
                Sequential Slot Scheduling Across Rooms:
              </span>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {panels.map((p, idx) => (
                  <div
                    key={idx}
                    className="border border-neutral-300 p-2.5 bg-white space-y-1.5"
                  >
                    <div className="flex justify-between items-center text-[11px] font-bold">
                      <span className="text-black">{p.name} ({p.room})</span>
                      <span className="bg-neutral-100 px-1.5 py-0.2 border border-neutral-300">
                        {p.assigned_team_ids.length} Teams
                      </span>
                    </div>

                    <div className="text-[10px] text-neutral-600 divide-y divide-neutral-100 border border-neutral-100 max-h-24 overflow-y-auto">
                      {p.assigned_team_ids.length === 0 ? (
                        <div className="p-1 italic text-neutral-400">No teams assigned</div>
                      ) : (
                        p.assigned_team_ids.map((teamId, seq) => {
                          const teamObj = availableTeams.find((t) => t.id === teamId);
                          return (
                            <div key={teamId} className="p-1 flex justify-between">
                              <span>Slot #{seq + 1}: <strong>{teamObj?.name || teamId}</strong></span>
                            </div>
                          );
                        })
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* STEP 4: Review & Finalize */}
        {step === 4 && (
          <div className="space-y-3 font-mono text-xs">
            <div className="border border-black p-3 bg-neutral-100 flex items-start gap-2.5">
              <CheckCircle2 className="h-5 w-5 text-black shrink-0 mt-0.5" />
              <div>
                <span className="font-bold text-black uppercase">
                  Ready to Provision Judging Round
                </span>
                <p className="text-neutral-600 mt-0.5 text-[11px]">
                  Submitting will create the round, generate all rooms/panels, assign evaluators, and sequentialize team staging slots.
                </p>
              </div>
            </div>

            <div className="border border-neutral-300 divide-y divide-neutral-200 bg-neutral-50">
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Round Title</span>
                <strong className="text-black">{roundName}</strong>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Slot Duration</span>
                <span className="font-bold">{slotDurationMins} Minutes per team</span>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Queue Buffer Depth</span>
                <span className="font-bold">{bufferSize} Teams staged in queue</span>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Total Criteria Defined</span>
                <span className="font-bold">{criteria.length} Rubrics</span>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Rooms / Panels Configured</span>
                <span className="font-bold">{panels.length} Rooms</span>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Total Teams Slotted</span>
                <strong className="text-black">{selectedTeamIds.length} Teams</strong>
              </div>
            </div>

            {/* Room breakdown */}
            <div className="space-y-1.5 pt-1">
              <span className="text-[10px] uppercase font-bold text-neutral-500">
                Room Allocations Breakdown:
              </span>
              <div className="space-y-1">
                {panels.map((p, i) => (
                  <div key={i} className="border border-neutral-300 p-2 bg-white flex justify-between text-[11px]">
                    <div>
                      <strong>{p.name}</strong> ({p.room}) — {p.judge_ids.length} Evaluators
                    </div>
                    <div className="font-bold text-black">
                      {p.assigned_team_ids.length} Teams Slotted
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
