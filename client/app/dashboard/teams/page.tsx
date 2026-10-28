"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Layers,
  Plus,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useAddTeamMember,
  useCreateTeam,
  useDeleteTeam,
  useRemoveTeamMember,
  useTeams,
  useUpdateTeam,
} from "@/hooks/use-teams";
import {
  CreateTeamRequest,
  TeamFilter,
  TeamResponse,
  UpdateTeamRequest,
} from "@/types/teams";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/pagination";
import { Alert } from "@/components/ui/alert";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  TeamFilters,
  TeamTable,
  TeamCreateDialog,
  TeamEditDialog,
  TeamDetailDialog,
  TeamDeleteDialog,
} from "@/components/teams";

export default function TeamsPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAdmin, isVolunteer, isJudge, isParticipant, isLoading: authLoading } = useAuth();

  // Filters state
  const [filters, setFilters] = useState<TeamFilter>({
    page: 1,
    limit: 10,
    search: "",
  });

  // Query & Mutations
  const {
    data: teamsResponse,
    isLoading: teamsLoading,
    isFetching,
    refetch,
  } = useTeams(filters);

  const createTeamMutation = useCreateTeam();
  const updateTeamMutation = useUpdateTeam();
  const deleteTeamMutation = useDeleteTeam();
  const addMemberMutation = useAddTeamMember();
  const removeMemberMutation = useRemoveTeamMember();

  // Dialog states
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [editingTeam, setEditingTeam] = useState<TeamResponse | null>(null);
  const [deletingTeam, setDeletingTeam] = useState<TeamResponse | null>(null);
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null);

  // Guards
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace("/login");
    }
  }, [authLoading, user, router]);

  if (authLoading || !user) {
    return (
      <div className="p-6 font-mono">
        <SkeletonCard />
      </div>
    );
  }

  // Participants role guard (backend /teams endpoint requires admin, volunteer, or judge)
  if (isParticipant && !isAdmin && !isVolunteer && !isJudge) {
    return (
      <div className="p-6 font-mono max-w-4xl mx-auto space-y-4">
        <Alert variant="info" title="SQUAD DIRECTORY ACCESS">
          <div className="flex items-center gap-2">
            <ShieldAlert className="h-5 w-5 text-black" />
            <span>
              The full event squad directory is managed by organizers, volunteers, and judges. You can check your squad roster directly in the Participant Portal.
            </span>
          </div>
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push("/dashboard/participant")}
            >
              Go to Participant Portal
            </Button>
          </div>
        </Alert>
      </div>
    );
  }

  // Handlers
  const handleCreateTeam = async (payload: CreateTeamRequest) => {
    try {
      const created = await createTeamMutation.mutateAsync(payload);
      toast.success(`Team "${created.name}" created successfully.`, "TEAM CREATED");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to create team";
      toast.error(errorMsg, "CREATION FAILED");
      throw err;
    }
  };

  const handleUpdateTeam = async (id: string, payload: UpdateTeamRequest) => {
    try {
      await updateTeamMutation.mutateAsync({ id, payload });
      toast.success("Team settings updated successfully.", "UPDATED");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Update failed";
      toast.error(errorMsg, "ERROR");
      throw err;
    }
  };

  const handleDeleteTeam = async (id: string) => {
    try {
      await deleteTeamMutation.mutateAsync(id);
      toast.info("Team disbanded and members safely unlinked.", "TEAM DELETED");
      setDeletingTeam(null);
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Deletion failed";
      toast.error(errorMsg, "ERROR");
    }
  };

  const handleAddMember = async (teamId: string, userId: string) => {
    try {
      await addMemberMutation.mutateAsync({ teamId, payload: { user_id: userId } });
      toast.success("Participant added to team squad.", "MEMBER ENROLLED");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to add member";
      toast.error(errorMsg, "ERROR");
      throw err;
    }
  };

  const handleRemoveMember = async (teamId: string, userId: string) => {
    try {
      await removeMemberMutation.mutateAsync({ teamId, userId });
      toast.info("Member detached from team.", "MEMBER REMOVED");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to remove member";
      toast.error(errorMsg, "ERROR");
      throw err;
    }
  };

  const handleSetLeader = async (teamId: string, leaderId: string) => {
    try {
      await updateTeamMutation.mutateAsync({
        id: teamId,
        payload: { leader_id: leaderId },
      });
      toast.success("Team leader assigned successfully.", "LEADER DESIGNATED");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to update leader";
      toast.error(errorMsg, "ERROR");
      throw err;
    }
  };

  return (
    <div className="translucent-container space-y-6 font-sans">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-5 border-b border-[var(--card-line)]">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Badge variant="blue" className="text-[10px] tracking-wider uppercase font-semibold">
              {isAdmin ? "ADMIN CONTROL" : isVolunteer ? "VOLUNTEER ACCESS" : "EVENT OPERATIONS"}
            </Badge>
            <span className="text-xs text-[var(--ink-tertiary)] font-mono">
              {teamsResponse?.total_count ?? 0} SQUADS REGISTERED
            </span>
          </div>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)] flex items-center gap-2">
            <Layers className="h-6 w-6 text-[var(--color-blue)]" />
            <span>Teams & Squads</span>
          </h1>
          <p className="text-xs text-[var(--ink-secondary)] mt-1">
            Directory of event teams, participant allocations, and designated leaders.
          </p>
        </div>

        {/* Action Toolbar */}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh Teams"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          {isAdmin && (
            <Button
              variant="primary"
              size="sm"
              onClick={() => setIsCreateOpen(true)}
              className="h-9 px-4 text-xs gap-1.5"
            >
              <Plus className="h-3.5 w-3.5" />
              <span>Create Team</span>
            </Button>
          )}
        </div>
      </div>

      {/* Teams Directory */}
      <div className="space-y-4">
        {/* Filters */}
        <TeamFilters
          filters={filters}
          onChange={(newFilters) => setFilters(newFilters)}
          onReset={() =>
            setFilters({
              page: 1,
              limit: filters.limit,
              search: "",
            })
          }
        />

        {/* Table */}
        <TeamTable
          teams={teamsResponse?.data ?? []}
          isLoading={teamsLoading}
          onView={(team) => setSelectedTeamId(team.id)}
          onEdit={(team) => setEditingTeam(team)}
          onDelete={(team) => setDeletingTeam(team)}
          canManage={isAdmin}
        />

        {/* Pagination */}
        {teamsResponse && (
          <Pagination
            page={teamsResponse.page}
            totalPages={teamsResponse.total_pages}
            totalCount={teamsResponse.total_count}
            limit={teamsResponse.limit}
            onPageChange={(page) => setFilters((prev) => ({ ...prev, page }))}
            onLimitChange={(limit) =>
              setFilters((prev) => ({ ...prev, limit, page: 1 }))
            }
          />
        )}
      </div>

      {/* Dialogs */}
      <TeamCreateDialog
        open={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSubmit={handleCreateTeam}
        isLoading={createTeamMutation.isPending}
      />

      <TeamEditDialog
        team={editingTeam}
        open={Boolean(editingTeam)}
        onClose={() => setEditingTeam(null)}
        onSubmit={handleUpdateTeam}
        isLoading={updateTeamMutation.isPending}
      />

      <TeamDeleteDialog
        team={deletingTeam}
        open={Boolean(deletingTeam)}
        onClose={() => setDeletingTeam(null)}
        onConfirm={handleDeleteTeam}
        isLoading={deleteTeamMutation.isPending}
      />

      <TeamDetailDialog
        teamId={selectedTeamId}
        open={Boolean(selectedTeamId)}
        onClose={() => setSelectedTeamId(null)}
        onAddMember={handleAddMember}
        onRemoveMember={handleRemoveMember}
        onSetLeader={handleSetLeader}
        canManage={isAdmin}
      />
    </div>
  );
}
