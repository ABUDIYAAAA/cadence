"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { RefreshCw, Upload, UserPlus } from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useAddCompetitionUser,
  useBanCompetitionMember,
  useBulkAddCompetitionUsers,
  useCompetitionUsers,
  useRemoveCompetitionMember,
  useUnbanCompetitionMember,
  useUpdateCompetitionMemberRole,
} from "@/hooks/use-competitions";
import {
  BulkCreateUsersRequest,
  CreateUserRequest,
  UpdateUserRequest,
  UserDetailResponse,
  UserFilter,
} from "@/types/users";
import { AssignableEventRole } from "@/types/auth";
import { apiErrorMessage } from "@/lib/utils";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import { AdminTablePageSkeleton, Skeleton } from "@/components/ui/skeleton";
import {
  UserBulkCreateDialog,
  UserCreateDialog,
  UserDeleteDialog,
  UserDetailDialog,
  UserEditDialog,
  UserFilters,
  UserTable,
} from "@/components/users";

export default function CompetitionUsersClient() {
  const router = useRouter();
  const params = useParams<{ competitionSlug: string }>();
  const toast = useToast();
  const {
    user: currentUser,
    isDev,
    isOwner,
    isAdmin,
    isVolunteer,
    isLoading: authLoading,
  } = useAuth();
  const { eventSlug, activeCompetition, competitions } = useTenant();

  const rawSlug = params?.competitionSlug ?? "";
  let compSlug = rawSlug;
  try {
    compSlug = decodeURIComponent(rawSlug);
  } catch {
    // keep raw segment
  }

  const currentComp =
    competitions.find((c) => c.slug === compSlug) ??
    (activeCompetition?.slug === compSlug ? activeCompetition : null);
  const compName = currentComp?.name ?? compSlug;

  const [filters, setFilters] = useState<UserFilter>({
    page: 1,
    limit: 25,
    search: "",
    role: isVolunteer && !isAdmin ? "participant" : undefined,
    is_banned: undefined,
  });

  const {
    data: usersResponse,
    isLoading: usersLoading,
    isFetching,
    isError: usersError,
    error: usersErrorObj,
    refetch,
  } = useCompetitionUsers(eventSlug, compSlug, filters, {
    enabled: Boolean(eventSlug && compSlug),
  });

  const addMember = useAddCompetitionUser(eventSlug, compSlug);
  const bulkAddMembers = useBulkAddCompetitionUsers(eventSlug, compSlug);
  const updateMemberRole = useUpdateCompetitionMemberRole(eventSlug, compSlug);
  const removeMember = useRemoveCompetitionMember(eventSlug, compSlug);
  const banMember = useBanCompetitionMember(eventSlug, compSlug);
  const unbanMember = useUnbanCompetitionMember(eventSlug, compSlug);

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isBulkCreateOpen, setIsBulkCreateOpen] = useState(false);
  const [viewingUser, setViewingUser] = useState<UserDetailResponse | null>(null);
  const [editingUser, setEditingUser] = useState<UserDetailResponse | null>(null);
  const [deletingUser, setDeletingUser] = useState<UserDetailResponse | null>(null);
  const [actionInProgressId, setActionInProgressId] = useState<string | null>(null);

  const metrics = useMemo(() => {
    const data = usersResponse?.data || [];
    const total = usersResponse?.total_count ?? 0;
    const banned = data.filter((u) => u.is_banned).length;
    const active = data.length - banned;
    const staff = data.filter((u) => u.role !== "participant").length;
    return { total, active, banned, staff };
  }, [usersResponse]);

  useEffect(() => {
    if (!authLoading && !currentUser) {
      router.replace("/login");
    }
  }, [authLoading, currentUser, router]);

  if (authLoading || !currentUser) {
    return <AdminTablePageSkeleton showMetrics />;
  }

  if (!isAdmin && !isVolunteer && !isDev) {
    return (
      <div className="font-sans space-y-4">
        <Alert variant="error" title="Access restricted">
          <div>You do not have permission to manage competition members.</div>
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push("/dashboard/competitions")}
            >
              Back to competitions
            </Button>
          </div>
        </Alert>
      </div>
    );
  }

  const handleCreateUser = async (data: CreateUserRequest) => {
    try {
      const created = await addMember.mutateAsync({
        email: data.email,
        name: data.name,
        phone_number: data.phone_number,
        role:
          data.role === "admin" ||
          data.role === "judge" ||
          data.role === "volunteer" ||
          data.role === "participant"
            ? data.role
            : "participant",
      });
      toast.success(`${created.email} added to ${compName}.`, "Member added");
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to add member"), "Could not add member");
      throw err;
    }
  };

  const handleBulkCreate = async (payload: BulkCreateUsersRequest) => {
    try {
      const res = await bulkAddMembers.mutateAsync({
        users: payload.users.map((u) => ({
          email: u.email,
          name: u.name,
          phone_number: u.phone_number,
          role:
            u.role === "admin" ||
            u.role === "judge" ||
            u.role === "volunteer" ||
            u.role === "participant"
              ? u.role
              : "participant",
          team_name: u.team_name,
        })),
      });
      toast.success(
        `Imported ${res.total_created} of ${res.total_processed} members into ${compName}.`,
        "Import complete"
      );
      return res;
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Bulk import failed"), "Import error");
      throw err;
    }
  };

  const handleUpdateUser = async (id: string, payload: UpdateUserRequest) => {
    try {
      const nextRole: AssignableEventRole =
        payload.role === "admin" ||
        payload.role === "judge" ||
        payload.role === "volunteer" ||
        payload.role === "participant"
          ? payload.role
          : "participant";
      if (!editingUser || editingUser.role !== nextRole) {
        await updateMemberRole.mutateAsync({
          userId: id,
          payload: { role: nextRole },
        });
      }
      if (payload.is_active !== undefined && editingUser) {
        const wasBanned = Boolean(editingUser.is_banned);
        const wantBanned = !payload.is_active;
        if (wantBanned && !wasBanned) {
          await banMember.mutateAsync({ userId: id });
        } else if (!wantBanned && wasBanned) {
          await unbanMember.mutateAsync(id);
        }
      }
      toast.success("Competition member updated.", "Member updated");
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Update failed"), "Error");
      throw err;
    }
  };

  const handleDeleteUser = async (id: string) => {
    try {
      await removeMember.mutateAsync(id);
      toast.info(`Member removed from ${compName}.`, "Member removed");
      setDeletingUser(null);
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Removal failed"), "Error");
    }
  };

  const handleToggleRestriction = async (targetUser: UserDetailResponse) => {
    setActionInProgressId(targetUser.id);
    try {
      if (targetUser.is_banned) {
        await unbanMember.mutateAsync(targetUser.id);
        toast.success(`${targetUser.email} unbanned in ${compName}.`, "Member unbanned");
      } else {
        await banMember.mutateAsync({ userId: targetUser.id });
        toast.info(`${targetUser.email} banned from ${compName}.`, "Member banned");
      }
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Action failed"), "Error");
    } finally {
      setActionInProgressId(null);
    }
  };

  return (
    <div className="space-y-6 font-sans">
      {/* Page Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Competition members
            </h1>
            {usersResponse?.total_count !== undefined && (
              <Badge variant="subtle" size="sm">
                {usersResponse.total_count}{" "}
                {usersResponse.total_count === 1 ? "member" : "members"}
              </Badge>
            )}
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            {isAdmin
              ? `Manage participants, judges, volunteers, and admins enrolled in ${compName}.`
              : `Add participants to ${compName}.`}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh directory"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`} />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          {isAdmin && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsBulkCreateOpen(true)}
              className="h-9 px-3 text-xs gap-1.5"
            >
              <Upload className="h-3.5 w-3.5" />
              <span>Import CSV</span>
            </Button>
          )}

          {(isAdmin || isVolunteer) && (
            <Button
              variant="primary"
              size="sm"
              onClick={() => setIsCreateOpen(true)}
              className="h-9 px-4 text-xs gap-1.5 font-semibold"
            >
              <UserPlus className="h-3.5 w-3.5" />
              <span>Add member</span>
            </Button>
          )}
        </div>
      </div>

      {usersError && (
        <Alert variant="error" title="Could not load competition members">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <span>{apiErrorMessage(usersErrorObj, "Failed to fetch competition members.")}</span>
            <Button variant="outline" size="sm" onClick={() => refetch()} className="h-8 px-3 text-xs shrink-0">
              Try again
            </Button>
          </div>
        </Alert>
      )}

      {/* Operational Metrics Strip */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3.5">
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <span className="text-[11px] font-medium text-[var(--ink-secondary)]">
            Total competition members
          </span>
          <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {usersLoading ? <Skeleton className="h-7 w-12" /> : metrics.total}
          </div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">
              Active on page
            </span>
            <span className="h-2 w-2 rounded-full bg-[var(--color-emerald)]" />
          </div>
          <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {usersLoading ? <Skeleton className="h-7 w-12" /> : metrics.active}
          </div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">
              Banned in competition
            </span>
            <span className="h-2 w-2 rounded-full bg-[var(--color-coral)]" />
          </div>
          <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {usersLoading ? <Skeleton className="h-7 w-12" /> : metrics.banned}
          </div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">
              Staff &amp; evaluators
            </span>
            <span className="h-2 w-2 rounded-full bg-[var(--color-blue)]" />
          </div>
          <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {usersLoading ? <Skeleton className="h-7 w-12" /> : metrics.staff}
          </div>
        </div>
      </div>

      {/* Filters & Table */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        <UserFilters
          filters={filters}
          mode="event"
          canViewRemoved={false}
          onChange={(newFilters) => setFilters(newFilters)}
          onReset={() =>
            setFilters({
              page: 1,
              limit: filters.limit,
              search: "",
              role: isVolunteer && !isAdmin ? "participant" : undefined,
              no_team: undefined,
              is_banned: undefined,
            })
          }
        />

        <UserTable
          users={usersResponse?.data ?? []}
          isLoading={usersLoading}
          mode="event"
          onView={(u) => setViewingUser(u)}
          onEdit={(u) => setEditingUser(u)}
          onDelete={(u) => setDeletingUser(u)}
          onToggleRestriction={handleToggleRestriction}
          actionInProgressId={actionInProgressId}
          canManage={isAdmin}
        />

        {usersResponse && (
          <Pagination
            page={usersResponse.page}
            totalPages={usersResponse.total_pages}
            totalCount={usersResponse.total_count}
            limit={usersResponse.limit}
            onPageChange={(page) => setFilters((prev) => ({ ...prev, page }))}
            onLimitChange={(limit) =>
              setFilters((prev) => ({ ...prev, limit, page: 1 }))
            }
          />
        )}
      </div>

      {/* Dialogs */}
      <UserCreateDialog
        open={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSubmit={handleCreateUser}
        isLoading={addMember.isPending}
        participantOnly={!isAdmin}
        canAssignAdmin={isOwner}
      />

      <UserBulkCreateDialog
        open={isBulkCreateOpen}
        onClose={() => setIsBulkCreateOpen(false)}
        onSubmit={handleBulkCreate}
        competitions={currentComp ? [currentComp] : []}
        isLoading={bulkAddMembers.isPending}
      />

      <UserEditDialog
        user={editingUser}
        open={Boolean(editingUser)}
        mode="event"
        canAssignAdmin={isOwner}
        onClose={() => setEditingUser(null)}
        onSubmit={handleUpdateUser}
        isLoading={
          updateMemberRole.isPending ||
          banMember.isPending ||
          unbanMember.isPending
        }
      />

      <UserDeleteDialog
        user={deletingUser}
        open={Boolean(deletingUser)}
        mode="event"
        onClose={() => setDeletingUser(null)}
        onConfirm={handleDeleteUser}
        isLoading={removeMember.isPending}
      />

      <UserDetailDialog
        user={viewingUser}
        open={Boolean(viewingUser)}
        mode="event"
        onClose={() => setViewingUser(null)}
        onEdit={(u) => {
          setViewingUser(null);
          setEditingUser(u);
        }}
        onDelete={(u) => {
          setViewingUser(null);
          setDeletingUser(u);
        }}
        onToggleRestriction={handleToggleRestriction}
        actionInProgress={Boolean(actionInProgressId)}
        canManage={isAdmin}
      />
    </div>
  );
}
