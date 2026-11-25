"use client";

import React, { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  UserPlus,
  Upload,
  RefreshCw,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useBlacklistUser,
  useBulkCreatePlatformUsers,
  useBulkCreateUsers,
  useCreatePlatformUser,
  useCreateUser,
  useDeleteUser,
  usePlatformUsers,
  useReactivateUser,
  useUnblacklistUser,
  useUpdateUser,
  useUsers,
} from "@/hooks/use-users";
import {
  useBanEventMember,
  useRemoveEventMember,
  useUnbanEventMember,
  useUpdateEventMemberRole,
} from "@/hooks/use-events";
import {
  BulkCreateUsersRequest,
  CreateUserRequest,
  UpdateUserRequest,
  UserDetailResponse,
  UserFilter,
} from "@/types/users";
import { AssignableEventRole } from "@/types/auth";
import { apiErrorMessage } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/pagination";
import { Alert } from "@/components/ui/alert";
import { AdminTablePageSkeleton, Skeleton } from "@/components/ui/skeleton";
import {
  UserFilters,
  UserTable,
  UserCreateDialog,
  UserBulkCreateDialog,
  UserEditDialog,
  UserDeleteDialog,
  UserDetailDialog,
} from "@/components/users";

export default function UsersPage() {
  const router = useRouter();
  const toast = useToast();
  const {
    user: currentUser,
    isDev,
    isOwner,
    isAdmin,
    isVolunteer,
    isLoading: authLoading,
  } = useAuth();
  const { eventSlug, competitions } = useTenant();

  const [selectedMode, setMode] = useState<"event" | "platform">(() =>
    !eventSlug ? "platform" : "event"
  );
  const mode: "event" | "platform" =
    isDev && !eventSlug ? "platform" : selectedMode;

  const [filters, setFilters] = useState<UserFilter>({
    page: 1,
    limit: 25,
    search: "",
    role: isVolunteer && !isAdmin ? "participant" : undefined,
    is_active: undefined,
    is_banned: undefined,
  });

  // Event-scoped query vs platform-wide query (for dev)
  const eventUsersQuery = useUsers(filters, {
    enabled: mode === "event" && Boolean(eventSlug),
  });
  const platformUsersQuery = usePlatformUsers(filters, {
    enabled: mode === "platform" && isDev,
  });

  const activeQuery = mode === "platform" ? platformUsersQuery : eventUsersQuery;
  const usersResponse = activeQuery.data;
  const usersLoading = activeQuery.isLoading;
  const isFetching = activeQuery.isFetching;
  const usersError = activeQuery.isError;
  const usersErrorObj = activeQuery.error;
  const refetch = activeQuery.refetch;

  // Event mutations
  const createEventMember = useCreateUser();
  const bulkCreateEventMembers = useBulkCreateUsers();
  const updateEventMemberRole = useUpdateEventMemberRole(eventSlug || "");
  const removeEventMember = useRemoveEventMember(eventSlug || "");
  const banEventMember = useBanEventMember(eventSlug || "");
  const unbanEventMember = useUnbanEventMember(eventSlug || "");

  // Platform dev mutations
  const createPlatformUser = useCreatePlatformUser();
  const bulkCreatePlatformUsers = useBulkCreatePlatformUsers();
  const updatePlatformUser = useUpdateUser();
  const deletePlatformUser = useDeleteUser();
  const reactivatePlatformUser = useReactivateUser();
  const blacklistUser = useBlacklistUser();
  const unblacklistUser = useUnblacklistUser();

  // Dialog states
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isBulkCreateOpen, setIsBulkCreateOpen] = useState(false);
  const [viewingUser, setViewingUser] = useState<UserDetailResponse | null>(
    null
  );
  const [editingUser, setEditingUser] = useState<UserDetailResponse | null>(
    null
  );
  const [deletingUser, setDeletingUser] = useState<UserDetailResponse | null>(
    null
  );
  const [actionInProgressId, setActionInProgressId] = useState<string | null>(
    null
  );

  const metrics = useMemo(() => {
    const data = usersResponse?.data || [];
    const total = usersResponse?.total_count ?? 0;
    const restricted =
      mode === "event"
        ? data.filter((u) => u.is_banned).length
        : data.filter((u) => !u.is_active).length;
    const active = data.length - restricted;
    const staff = data.filter((u) => u.role !== "participant").length;
    return { total, active, restricted, staff };
  }, [usersResponse, mode]);

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
          <div>You do not have permission to manage event members.</div>
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push("/dashboard")}
            >
              Return to dashboard
            </Button>
          </div>
        </Alert>
      </div>
    );
  }

  const handleModeChange = (nextMode: "event" | "platform") => {
    setMode(nextMode);
    setFilters({
      page: 1,
      limit: 25,
      search: "",
      role: undefined,
      no_team: undefined,
      competition_id: undefined,
      is_active: undefined,
      is_banned: undefined,
      deleted: undefined,
    });
  };

  const handleCreateUser = async (data: CreateUserRequest) => {
    try {
      const created =
        mode === "platform"
          ? await createPlatformUser.mutateAsync(data)
          : await createEventMember.mutateAsync(data);
      toast.success(
        mode === "platform"
          ? `Platform user ${created.email} created.`
          : `${created.email} added to the event.`,
        mode === "platform" ? "User created" : "Member added"
      );
    } catch (err: unknown) {
      const errorMsg = apiErrorMessage(err, "Failed to add user");
      toast.error(errorMsg, "Could not add user");
      throw err;
    }
  };

  const handleBulkCreate = async (payload: BulkCreateUsersRequest) => {
    try {
      const res =
        mode === "platform"
          ? await bulkCreatePlatformUsers.mutateAsync(payload)
          : await bulkCreateEventMembers.mutateAsync(payload);
      const teamPart =
        res.teams_assigned && res.teams_assigned > 0
          ? `, ${res.teams_assigned} team assignment${res.teams_assigned === 1 ? "" : "s"}`
          : "";
      if (res.skipped_count > 0) {
        toast.warning(
          `Processed ${res.total_processed} rows (${res.total_created} accounts${teamPart}, ${res.skipped_count} skipped).`,
          "Import completed with warnings"
        );
      } else {
        toast.success(
          `Imported ${res.total_created} account${res.total_created === 1 ? "" : "s"}${teamPart}.`,
          "Import complete"
        );
      }
      return res;
    } catch (err: unknown) {
      const errorMsg = apiErrorMessage(err, "Bulk import failed");
      toast.error(errorMsg, "Import error");
      throw err;
    }
  };

  const handleUpdateUser = async (id: string, payload: UpdateUserRequest) => {
    try {
      if (mode === "event" && eventSlug) {
        const nextRole: AssignableEventRole =
          payload.role === "admin" ||
          payload.role === "judge" ||
          payload.role === "volunteer" ||
          payload.role === "participant"
            ? payload.role
            : "participant";
        if (!editingUser || editingUser.role !== nextRole) {
          await updateEventMemberRole.mutateAsync({
            userId: id,
            payload: { role: nextRole },
          });
        }
        if (payload.is_active !== undefined && editingUser) {
          const wasBanned = Boolean(editingUser.is_banned);
          const wantBanned = !payload.is_active;
          if (wantBanned && !wasBanned) {
            await banEventMember.mutateAsync({ userId: id });
          } else if (!wantBanned && wasBanned) {
            await unbanEventMember.mutateAsync(id);
          }
        }
        toast.success("Event member updated.", "Member updated");
      } else {
        await updatePlatformUser.mutateAsync({ id, payload });
        toast.success("Platform user details updated.", "User updated");
      }
    } catch (err: unknown) {
      const errorMsg = apiErrorMessage(err, "Update failed");
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  const handleDeleteUser = async (id: string) => {
    try {
      if (mode === "event" && eventSlug) {
        await removeEventMember.mutateAsync(id);
        toast.info("Member removed from this event.", "Member removed");
      } else {
        await deletePlatformUser.mutateAsync(id);
        toast.info("User removed and session invalidated.", "User removed");
      }
      setDeletingUser(null);
    } catch (err: unknown) {
      const errorMsg = apiErrorMessage(err, "Removal failed");
      toast.error(errorMsg, "Error");
    }
  };

  const handleToggleRestriction = async (targetUser: UserDetailResponse) => {
    setActionInProgressId(targetUser.id);
    try {
      if (mode === "event" && eventSlug) {
        if (targetUser.is_banned) {
          await unbanEventMember.mutateAsync(targetUser.id);
          toast.success(
            `${targetUser.email} unbanned in this event.`,
            "Member unbanned"
          );
        } else {
          await banEventMember.mutateAsync({ userId: targetUser.id });
          toast.info(
            `${targetUser.email} banned from this event.`,
            "Member banned"
          );
        }
      } else {
        if (!targetUser.is_active) {
          await unblacklistUser.mutateAsync(targetUser.id);
          toast.success(
            `${targetUser.email} unblacklisted globally.`,
            "User unblacklisted"
          );
        } else {
          await blacklistUser.mutateAsync(targetUser.id);
          toast.info(
            `${targetUser.email} blacklisted across Cadence.`,
            "User blacklisted"
          );
        }
      }
    } catch (err: unknown) {
      const errorMsg = apiErrorMessage(err, "Action failed");
      toast.error(errorMsg, "Error");
    } finally {
      setActionInProgressId(null);
    }
  };

  const handleReactivateUser = async (targetUser: UserDetailResponse) => {
    setActionInProgressId(targetUser.id);
    try {
      await reactivatePlatformUser.mutateAsync(targetUser.id);
      toast.success(`${targetUser.email} reactivated.`, "User reactivated");
    } catch (err: unknown) {
      const errorMsg = apiErrorMessage(err, "Reactivation failed");
      toast.error(errorMsg, "Error");
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
              {mode === "event" ? "Event members" : "Platform users"}
            </h1>
            {usersResponse?.total_count !== undefined && (
              <Badge variant="subtle" size="sm">
                {usersResponse.total_count}{" "}
                {usersResponse.total_count === 1 ? "record" : "records"}
              </Badge>
            )}
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            {mode === "platform"
              ? "Global developer directory across all events with platform-wide blacklist controls."
              : isAdmin
              ? "Manage participants, judges, volunteers, and admins enrolled in this event."
              : "Add participants who registered with a different email address."}
          </p>
        </div>

        {/* Action Toolbar + Dev Mode Switcher */}
        <div className="flex flex-wrap items-center gap-2">
          {isDev && (
            <div className="inline-flex rounded-lg border border-[var(--card-line)] p-1 bg-[var(--card-solid-subtle)] gap-1">
              <button
                type="button"
                onClick={() => handleModeChange("event")}
                disabled={!eventSlug}
                className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed ${
                  mode === "event"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                }`}
              >
                Event members
              </button>
              <button
                type="button"
                onClick={() => handleModeChange("platform")}
                className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
                  mode === "platform"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                }`}
              >
                Platform (Dev)
              </button>
            </div>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh directory"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          {isAdmin && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setIsBulkCreateOpen(true)}
                className="h-9 px-3 text-xs gap-1.5"
              >
                <Upload className="h-3.5 w-3.5" />
                <span>Import CSV</span>
              </Button>

              <Button
                variant="primary"
                size="sm"
                onClick={() => setIsCreateOpen(true)}
                className="h-9 px-4 text-xs gap-1.5 font-semibold"
              >
                <UserPlus className="h-3.5 w-3.5" />
                <span>
                  {mode === "platform" ? "Create user" : "Add member"}
                </span>
              </Button>
            </>
          )}
        </div>
      </div>

      {usersError && (
        <Alert variant="error" title="Could not load users">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <span>{apiErrorMessage(usersErrorObj, "Failed to fetch users from the server.")}</span>
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
            {mode === "event" ? "Total event members" : "Total platform users"}
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
              {mode === "event" ? "Banned in event" : "Blacklisted"}
            </span>
            <span className="h-2 w-2 rounded-full bg-[var(--color-coral)]" />
          </div>
          <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {usersLoading ? <Skeleton className="h-7 w-12" /> : metrics.restricted}
          </div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">
              Staff & evaluators
            </span>
            <span className="h-2 w-2 rounded-full bg-[var(--color-blue)]" />
          </div>
          <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {usersLoading ? <Skeleton className="h-7 w-12" /> : metrics.staff}
          </div>
        </div>
      </div>

      {/* Primary Surface: Filters + Table + Pagination */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        <UserFilters
          filters={filters}
          mode={mode}
          competitions={competitions}
          canViewRemoved={mode === "platform" && isDev}
          onChange={(newFilters) => setFilters(newFilters)}
          onReset={() =>
            setFilters({
              page: 1,
              limit: filters.limit,
              search: "",
              role: undefined,
              no_team: undefined,
              competition_id: undefined,
              is_active: undefined,
              is_banned: undefined,
              deleted: undefined,
            })
          }
        />

        <UserTable
          users={usersResponse?.data ?? []}
          isLoading={usersLoading}
          mode={mode}
          onView={(user) => setViewingUser(user)}
          onEdit={(user) => setEditingUser(user)}
          onDelete={(user) => setDeletingUser(user)}
          onToggleRestriction={handleToggleRestriction}
          onReactivate={mode === "platform" ? handleReactivateUser : undefined}
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
        isLoading={
          createEventMember.isPending || createPlatformUser.isPending
        }
        participantOnly={!isAdmin}
        canAssignAdmin={isOwner || mode === "platform"}
      />

      <UserBulkCreateDialog
        open={isBulkCreateOpen}
        onClose={() => setIsBulkCreateOpen(false)}
        onSubmit={handleBulkCreate}
        competitions={mode === "event" ? competitions : []}
        isLoading={
          bulkCreateEventMembers.isPending || bulkCreatePlatformUsers.isPending
        }
      />

      <UserEditDialog
        user={editingUser}
        open={Boolean(editingUser)}
        mode={mode}
        canAssignAdmin={isOwner || mode === "platform"}
        onClose={() => setEditingUser(null)}
        onSubmit={handleUpdateUser}
        isLoading={
          updateEventMemberRole.isPending || updatePlatformUser.isPending
        }
      />

      <UserDeleteDialog
        user={deletingUser}
        open={Boolean(deletingUser)}
        mode={mode}
        onClose={() => setDeletingUser(null)}
        onConfirm={handleDeleteUser}
        isLoading={
          removeEventMember.isPending || deletePlatformUser.isPending
        }
      />

      <UserDetailDialog
        user={viewingUser}
        open={Boolean(viewingUser)}
        mode={mode}
        onClose={() => setViewingUser(null)}
        onEdit={(user) => {
          setViewingUser(null);
          setEditingUser(user);
        }}
        onDelete={(user) => {
          setViewingUser(null);
          setDeletingUser(user);
        }}
        onToggleRestriction={handleToggleRestriction}
        onReactivate={mode === "platform" ? handleReactivateUser : undefined}
        actionInProgress={Boolean(actionInProgressId)}
        canManage={isAdmin}
      />
    </div>
  );
}
