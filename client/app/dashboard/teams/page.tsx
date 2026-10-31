"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  RefreshCw,
  Users,
  ShieldCheck,
  UserCheck,
  Hash,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useAddTeamMember,
  useDeleteTeam,
  useRemoveTeamMember,
  useSetTeamFrozen,
  useTeams,
  useUpdateTeam,
} from "@/hooks/use-teams";
import {
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
  TeamSettingsCard,
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
    limit: 20,
    search: "",
  });

  // Query & Mutations
  const {
    data: teamsResponse,
    isLoading: teamsLoading,
    isFetching,
    refetch,
  } = useTeams(filters);

  const updateTeamMutation = useUpdateTeam();
  const deleteTeamMutation = useDeleteTeam();
  const addMemberMutation = useAddTeamMember();
  const removeMemberMutation = useRemoveTeamMember();
  const setFrozenMutation = useSetTeamFrozen();

  // Dialog states
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
      <div className="font-mono">
        <SkeletonCard />
      </div>
    );
  }

  // Participants role guard (the directory endpoints require admin, volunteer, or judge)
  if (isParticipant && !isAdmin && !isVolunteer && !isJudge) {
    return (
      <div className="font-sans space-y-4">
        <Alert variant="info" title="Team directory">
          <div>
            The full event squad directory is managed by organizers, volunteers, and judges. You can create, join and manage your own team in the participant portal.
          </div>
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push("/dashboard/participant")}
            >
              Go to participant portal
            </Button>
          </div>
        </Alert>
      </div>
    );
  }

  // Handlers
  const handleToggleFreeze = async (team: TeamResponse) => {
    const frozen = !team.is_frozen;
    try {
      await setFrozenMutation.mutateAsync({ id: team.id, frozen });
      toast[frozen ? "warning" : "success"](
        frozen
          ? `"${team.name}" is frozen. Members can't join, leave or be removed.`
          : `"${team.name}" is open again.`,
        frozen ? "Team frozen" : "Team unfrozen"
      );
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to update freeze state";
      toast.error(errorMsg, "Error");
    }
  };

  const handleUpdateTeam = async (id: string, payload: UpdateTeamRequest) => {
    try {
      await updateTeamMutation.mutateAsync({ id, payload });
      toast.success("Team settings updated.", "Team updated");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Update failed";
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  const handleDeleteTeam = async (id: string) => {
    try {
      await deleteTeamMutation.mutateAsync(id);
      toast.info("Team removed and members safely unlinked.", "Team removed");
      setDeletingTeam(null);
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Deletion failed";
      toast.error(errorMsg, "Error");
    }
  };

  const handleAddMember = async (teamId: string, userId: string) => {
    try {
      await addMemberMutation.mutateAsync({ teamId, payload: { user_id: userId } });
      toast.success("Member added to team.", "Member added");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to add member";
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  const handleRemoveMember = async (teamId: string, userId: string) => {
    try {
      await removeMemberMutation.mutateAsync({ teamId, userId });
      toast.info("Member detached from team.", "Member removed");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to remove member";
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  const handleSetLeader = async (teamId: string, leaderId: string) => {
    try {
      await updateTeamMutation.mutateAsync({
        id: teamId,
        payload: { leader_id: leaderId },
      });
      toast.success("Team lead assigned.", "Lead assigned");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to update leader";
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  // Admins and volunteers manage teams; only admins delete them. Judges are read-only.
  const canManage = isAdmin || isVolunteer;

  const totalTeams = teamsResponse?.total_count ?? 0;
  const teamsInView = teamsResponse?.data ?? [];
  const teamsWithLead = teamsInView.filter((t) => Boolean(t.leader_id)).length;
  const totalMembersInView = teamsInView.reduce((acc, t) => acc + (t.member_count || 0), 0);
  const avgMembers = teamsInView.length ? (totalMembersInView / teamsInView.length).toFixed(1) : "0";

  return (
    <div className="space-y-6 font-sans">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              {isJudge && !isAdmin ? "My Teams" : "Teams"}
            </h1>
            {teamsResponse?.total_count !== undefined && (
              <Badge variant="subtle" size="sm">
                {teamsResponse.total_count} {teamsResponse.total_count === 1 ? "team" : "teams"}
              </Badge>
            )}
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            {isJudge && !isAdmin
              ? "Teams assigned to the panels you evaluate."
              : "Participants create their own teams and share a join code. Freeze, edit or review them here."}
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
            title="Refresh teams"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

        </div>
      </div>

      {canManage && <TeamSettingsCard canEditSize={isAdmin} />}

      {/* Metrics Row (Canvas Level) */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[12px] p-4">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Total squads</span>
            <Users className="h-4 w-4 text-[var(--ink-tertiary)]" />
          </div>
          <div className="text-2xl font-bold text-[var(--ink-primary)] mt-2">
            {totalTeams}
          </div>
          <p className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            Registered event groups
          </p>
        </div>

        <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[12px] p-4">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Squads with lead</span>
            <UserCheck className="h-4 w-4 text-[var(--ink-tertiary)]" />
          </div>
          <div className="text-2xl font-bold text-[var(--ink-primary)] mt-2">
            {teamsWithLead}
          </div>
          <p className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            On this page
          </p>
        </div>

        <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[12px] p-4">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Rostered members</span>
            <ShieldCheck className="h-4 w-4 text-[var(--ink-tertiary)]" />
          </div>
          <div className="text-2xl font-bold text-[var(--ink-primary)] mt-2">
            {totalMembersInView}
          </div>
          <p className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            On this page
          </p>
        </div>

        <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[12px] p-4">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Avg squad size</span>
            <Hash className="h-4 w-4 text-[var(--ink-tertiary)]" />
          </div>
          <div className="text-2xl font-bold text-[var(--ink-primary)] mt-2">
            {avgMembers}
          </div>
          <p className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            Members per squad
          </p>
        </div>
      </div>

      {/* Primary Surface: Table + Controls */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
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
          teams={teamsInView}
          isLoading={teamsLoading}
          onView={(team) => setSelectedTeamId(team.id)}
          onEdit={(team) => setEditingTeam(team)}
          onDelete={(team) => setDeletingTeam(team)}
          onToggleFreeze={handleToggleFreeze}
          canManage={canManage}
          canFreeze={canManage}
          canDelete={isAdmin}
          showCodes={canManage}
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
        key={selectedTeamId ?? "none"}
        teamId={selectedTeamId}
        open={Boolean(selectedTeamId)}
        onClose={() => setSelectedTeamId(null)}
        onAddMember={handleAddMember}
        onRemoveMember={handleRemoveMember}
        onSetLeader={handleSetLeader}
        canManage={canManage}
      />
    </div>
  );
}
