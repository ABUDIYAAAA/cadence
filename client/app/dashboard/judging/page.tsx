"use client";

import React, { useState, useMemo, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Award,
  Plus,
  RefreshCw,
  Search,
  Filter,
  CheckCircle2,
  Clock,
  Layers,
  DoorOpen,
  UserCheck,
  CheckSquare,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useJudgingRounds,
  useActivateRound,
  useCreateRound,
  useCreatePanel,
  useConfigureSlots,
} from "@/hooks/use-judging";
import {
  RoundSummary,
  CreateRoundRequest,
  CreatePanelRequest,
  ConfigureSlotsRequest,
  SlotAssignment,
} from "@/types/judging";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  RoundTable,
  RoundCreateDialog,
  RoundPanelCreateDialog,
  RoundSlotsConfigDialog,
  RoundDetailDialog,
  RoundActivateDialog,
  VolunteerScheduleView,
  ParticipantJudgingStatusView,
  JudgeEvaluationView,
} from "@/components/judging";

type JudgingTab = "evaluation" | "rounds" | "volunteer_monitor" | "participant_preview";

export default function JudgingPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAdmin, isJudge, isVolunteer, isParticipant, isLoading: authLoading } = useAuth();

  // Active view tab: judges default to evaluation scorecard, admins default to rounds
  const [selectedTab, setSelectedTab] = useState<JudgingTab>("rounds");
  const activeTab: JudgingTab = isJudge && !isAdmin ? "evaluation" : selectedTab;
  const setActiveTab = setSelectedTab;

  // Queries & Mutations
  const {
    data: rounds = [],
    isLoading: roundsLoading,
    isFetching,
    refetch,
  } = useJudgingRounds();

  const activateRoundMutation = useActivateRound();
  const createRoundMutation = useCreateRound();
  const createPanelMutation = useCreatePanel();
  const configureSlotsMutation = useConfigureSlots();

  // Dialog State
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCreatingWorkflow, setIsCreatingWorkflow] = useState(false);
  const [selectedRoundForDetails, setSelectedRoundForDetails] = useState<RoundSummary | null>(null);
  const [selectedRoundForPanel, setSelectedRoundForPanel] = useState<RoundSummary | null>(null);
  const [selectedRoundForSlots, setSelectedRoundForSlots] = useState<RoundSummary | null>(null);
  const [selectedRoundForActivation, setSelectedRoundForActivation] = useState<RoundSummary | null>(null);

  // Search & Filters
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  // Filtering rounds
  const filteredRounds = useMemo(() => {
    return (rounds || []).filter((round) => {
      const matchesSearch =
        searchQuery.trim() === "" ||
        round.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        round.id.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesStatus =
        statusFilter === "all" || round.status === statusFilter;

      return matchesSearch && matchesStatus;
    });
  }, [rounds, searchQuery, statusFilter]);

  // Metric stats
  const stats = useMemo(() => {
    const list = rounds || [];
    const total = list.length;
    const active = list.filter((r) => r.status === "active").length;
    const completed = list.filter((r) => r.status === "completed").length;
    const draft = list.filter((r) => r.status === "draft").length;
    const totalPanels = list.reduce((acc, r) => acc + (r.panel_count || 0), 0);
    const totalSlots = list.reduce((acc, r) => acc + (r.slot_count || 0), 0);

    return { total, active, completed, draft, totalPanels, totalSlots };
  }, [rounds]);

  // Auth Guards
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace("/login");
    }
  }, [authLoading, user, router]);

  if (authLoading || !user) {
    return (
      <div className="p-6 font-mono max-w-7xl mx-auto space-y-4">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  // 1. Participant dedicated judging view
  if (isParticipant && !isAdmin && !isJudge && !isVolunteer) {
    return (
      <div className="p-6 font-mono max-w-7xl mx-auto">
        <ParticipantJudgingStatusView />
      </div>
    );
  }

  // 2. Volunteer dedicated room & schedule coordination view
  if (isVolunteer && !isAdmin && !isJudge) {
    return (
      <div className="p-6 font-mono max-w-7xl mx-auto">
        <VolunteerScheduleView />
      </div>
    );
  }

  // 3. Admin & Judge View Handlers
  const handleOpenActivate = (roundId: string) => {
    if (!isAdmin) {
      toast.error("Only administrators can activate judging rounds.");
      return;
    }
    const r = rounds.find((item) => item.id === roundId);
    if (r) {
      setSelectedRoundForActivation(r);
    }
  };

  const handleConfirmActivation = async () => {
    if (!selectedRoundForActivation) return;
    try {
      await activateRoundMutation.mutateAsync(selectedRoundForActivation.id);
      toast.success(`Round "${selectedRoundForActivation.name}" is now ACTIVE!`);
      setSelectedRoundForActivation(null);
      refetch();
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to activate round";
      toast.error(message);
    }
  };

  const handleFullWorkflowSubmit = async (params: {
    round: CreateRoundRequest;
    panels: Array<{
      name: string;
      room: string;
      judge_ids: string[];
      slots: SlotAssignment[];
    }>;
  }) => {
    setIsCreatingWorkflow(true);
    try {
      // 1. Create Round
      const createdRound = await createRoundMutation.mutateAsync(params.round);
      const roundId = createdRound.round_id || createdRound.id;

      if (!roundId) {
        throw new Error("Round created but server did not return a round ID");
      }

      // 2. Create Panels and configure Slots
      for (const p of params.panels) {
        const panelRes = await createPanelMutation.mutateAsync({
          round_id: roundId,
          name: p.name,
          room: p.room,
          judge_ids: p.judge_ids,
        });

        const panelId = panelRes.panel_id || panelRes.id;

        if (panelId && p.slots && p.slots.length > 0) {
          await configureSlotsMutation.mutateAsync({
            panelId,
            payload: {
              slots: p.slots,
            },
          });
        }
      }

      toast.success(`Judging round "${params.round.name}" created with panels and team slots!`);
      setIsCreateOpen(false);
      refetch();
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to create round workflow";
      toast.error(message);
    } finally {
      setIsCreatingWorkflow(false);
    }
  };

  const handlePanelCreate = async (payload: CreatePanelRequest) => {
    try {
      await createPanelMutation.mutateAsync(payload);
      toast.success(`Panel "${payload.name}" added successfully.`);
      setSelectedRoundForPanel(null);
      refetch();
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to create panel";
      toast.error(message);
    }
  };

  const handleSlotsConfigure = async (panelId: string, payload: ConfigureSlotsRequest) => {
    try {
      await configureSlotsMutation.mutateAsync({ panelId, payload });
      toast.success(`Configured ${payload.slots.length} team slot(s) for the room.`);
      setSelectedRoundForSlots(null);
      refetch();
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to configure slots";
      toast.error(message);
    }
  };

  return (
    <div className="p-6 font-mono max-w-7xl mx-auto space-y-6">
      {/* View Switcher Tabs (For Admin and Judge) */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b-2 border-black pb-4">
        <div>
          <div className="flex items-center gap-2">
            <Award className="h-6 w-6 text-black" />
            <h1 className="text-2xl font-bold tracking-tight uppercase">
              Judging Operations Center
            </h1>
          </div>
          <p className="text-sm text-neutral-600 mt-1">
            Grade team pitches, monitor volunteer staging rooms, and orchestrate evaluation rounds.
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <div className="border border-black p-0.5 bg-neutral-100 flex items-center flex-wrap">
            {/* Judge Evaluation View */}
            <button
              onClick={() => setActiveTab("evaluation")}
              className={`px-3 py-1.5 text-xs uppercase font-bold flex items-center gap-1.5 transition-all ${
                activeTab === "evaluation"
                  ? "bg-black text-white"
                  : "text-neutral-700 hover:text-black"
              }`}
            >
              <CheckSquare className="h-3.5 w-3.5" />
              {isJudge && !isAdmin ? "Evaluation Scorecard" : "Judge Scorecards"}
            </button>

            {/* Rounds Registry / Configuration */}
            <button
              onClick={() => setActiveTab("rounds")}
              className={`px-3 py-1.5 text-xs uppercase font-bold flex items-center gap-1.5 transition-all ${
                activeTab === "rounds"
                  ? "bg-black text-white"
                  : "text-neutral-700 hover:text-black"
              }`}
            >
              <Award className="h-3.5 w-3.5" />
              {isAdmin ? "Rounds Configuration" : "Rounds Overview"}
            </button>

            {/* Room Staging Monitor */}
            <button
              onClick={() => setActiveTab("volunteer_monitor")}
              className={`px-3 py-1.5 text-xs uppercase font-bold flex items-center gap-1.5 transition-all ${
                activeTab === "volunteer_monitor"
                  ? "bg-black text-white"
                  : "text-neutral-700 hover:text-black"
              }`}
            >
              <DoorOpen className="h-3.5 w-3.5" />
              Room Staging (Volunteers)
            </button>

            {isAdmin && (
              <button
                onClick={() => setActiveTab("participant_preview")}
                className={`px-3 py-1.5 text-xs uppercase font-bold flex items-center gap-1.5 transition-all ${
                  activeTab === "participant_preview"
                    ? "bg-black text-white"
                    : "text-neutral-700 hover:text-black"
                }`}
              >
                <UserCheck className="h-3.5 w-3.5" />
                Participant Queue View
              </button>
            )}
          </div>
        </div>
      </div>

      {/* TAB 1: JUDGE EVALUATION SCORECARDS */}
      {activeTab === "evaluation" && (
        <JudgeEvaluationView />
      )}

      {/* TAB 2: VOLUNTEER STAGING SCHEDULE MONITOR */}
      {activeTab === "volunteer_monitor" && (
        <VolunteerScheduleView />
      )}

      {/* TAB 3: PARTICIPANT PREVIEW */}
      {activeTab === "participant_preview" && (
        <ParticipantJudgingStatusView />
      )}

      {/* TAB 4: ROUNDS MANAGEMENT & CONFIGURATION */}
      {activeTab === "rounds" && (
        <>
          <div className="flex items-center justify-between gap-4">
            <div className="text-sm uppercase font-bold text-neutral-600">
              Evaluation Rounds Registry
            </div>

            <div className="flex items-center gap-2 flex-wrap">
              <Button
                variant="outline"
                size="sm"
                onClick={() => refetch()}
                disabled={isFetching}
                className="flex items-center gap-1.5 border-black text-black hover:bg-neutral-100"
              >
                <RefreshCw
                  className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`}
                />
                REFRESH
              </Button>

              {isAdmin && (
                <Button
                  size="sm"
                  onClick={() => setIsCreateOpen(true)}
                  className="flex items-center gap-1.5 bg-black text-white hover:bg-neutral-800 border border-black shadow-sm"
                >
                  <Plus className="h-4 w-4" />
                  CREATE JUDGING ROUND
                </Button>
              )}
            </div>
          </div>

          {/* Metrics Banner */}
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
            <div className="border border-black p-3 bg-white">
              <div className="text-xs uppercase text-neutral-500 font-semibold">Total Rounds</div>
              <div className="text-2xl font-bold text-black mt-1">{stats.total}</div>
            </div>
            <div className="border border-black p-3 bg-white">
              <div className="text-xs uppercase text-neutral-500 font-semibold flex items-center gap-1">
                <CheckCircle2 className="h-3.5 w-3.5 text-black" />
                Active
              </div>
              <div className="text-2xl font-bold text-black mt-1">{stats.active}</div>
            </div>
            <div className="border border-black p-3 bg-white">
              <div className="text-xs uppercase text-neutral-500 font-semibold flex items-center gap-1">
                <Clock className="h-3.5 w-3.5 text-neutral-400" />
                Draft
              </div>
              <div className="text-2xl font-bold text-black mt-1">{stats.draft}</div>
            </div>
            <div className="border border-black p-3 bg-white">
              <div className="text-xs uppercase text-neutral-500 font-semibold">Completed</div>
              <div className="text-2xl font-bold text-black mt-1">{stats.completed}</div>
            </div>
            <div className="border border-black p-3 bg-white">
              <div className="text-xs uppercase text-neutral-500 font-semibold flex items-center gap-1">
                <Layers className="h-3.5 w-3.5 text-black" />
                Total Panels
              </div>
              <div className="text-2xl font-bold text-black mt-1">{stats.totalPanels}</div>
            </div>
            <div className="border border-black p-3 bg-white">
              <div className="text-xs uppercase text-neutral-500 font-semibold">Total Slots</div>
              <div className="text-2xl font-bold text-black mt-1">{stats.totalSlots}</div>
            </div>
          </div>

          {/* Search and Filters */}
          <div className="border border-black p-4 bg-white flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-neutral-400" />
              <Input
                placeholder="Search rounds by name or ID..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9 border-black font-mono text-sm"
              />
            </div>

            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1.5 text-xs uppercase font-semibold text-neutral-600 shrink-0">
                <Filter className="h-3.5 w-3.5" />
                Status:
              </div>
              <div className="w-40">
                <Select
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                  options={[
                    { value: "all", label: "ALL STATUSES" },
                    { value: "draft", label: "DRAFT" },
                    { value: "active", label: "ACTIVE" },
                    { value: "completed", label: "COMPLETED" },
                  ]}
                  className="border-black font-mono text-sm"
                />
              </div>
            </div>
          </div>

          {/* Rounds Listing Table */}
          <RoundTable
            rounds={filteredRounds}
            isLoading={roundsLoading}
            onView={(round: RoundSummary) => setSelectedRoundForDetails(round)}
            onAddPanel={(round: RoundSummary) => setSelectedRoundForPanel(round)}
            onConfigureSlots={(round: RoundSummary) => setSelectedRoundForSlots(round)}
            onActivate={(roundId: string) => handleOpenActivate(roundId)}
            isActivatingId={activateRoundMutation.isPending ? selectedRoundForActivation?.id : null}
            isAdmin={isAdmin}
          />
        </>
      )}

      {/* Modals & Dialogs */}
      {/* 1. Main Round Creation Wizard (Rounds -> Panels -> Teams -> Slots) */}
      <RoundCreateDialog
        open={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSubmitFullWorkflow={handleFullWorkflowSubmit}
        isLoading={isCreatingWorkflow}
      />

      {/* 2. Quick Panel Addition Modal */}
      <RoundPanelCreateDialog
        open={Boolean(selectedRoundForPanel)}
        onClose={() => setSelectedRoundForPanel(null)}
        round={selectedRoundForPanel}
        onSubmit={handlePanelCreate}
        isLoading={createPanelMutation.isPending}
      />

      {/* 3. Quick Slot Configuration Modal */}
      <RoundSlotsConfigDialog
        open={Boolean(selectedRoundForSlots)}
        onClose={() => setSelectedRoundForSlots(null)}
        round={selectedRoundForSlots}
        onSubmit={handleSlotsConfigure}
        isLoading={configureSlotsMutation.isPending}
      />

      {/* 4. Round Criteria & Details Modal */}
      <RoundDetailDialog
        open={Boolean(selectedRoundForDetails)}
        onClose={() => setSelectedRoundForDetails(null)}
        round={selectedRoundForDetails}
        onActivate={(roundId: string) => handleOpenActivate(roundId)}
        isActivating={activateRoundMutation.isPending}
      />

      {/* 5. Custom In-App Activation Confirmation Dialog (NO NATIVE ALERTS) */}
      <RoundActivateDialog
        open={Boolean(selectedRoundForActivation)}
        onClose={() => setSelectedRoundForActivation(null)}
        round={selectedRoundForActivation}
        onConfirm={handleConfirmActivation}
        isLoading={activateRoundMutation.isPending}
      />
    </div>
  );
}
