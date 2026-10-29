"use client";

import React, { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  UserPlus,
  Upload,
  Mail,
  RefreshCw,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useBulkCreateUsers,
  useBulkSendInvite,
  useCreateUser,
  useDeleteUser,
  useSendInvite,
  useTriggerResetPassword,
  useUpdateUser,
  useUsers,
} from "@/hooks/use-users";
import {
  BulkCreateUsersRequest,
  BulkInviteRequest,
  CreateUserRequest,
  UpdateUserRequest,
  UserDetailResponse,
  UserFilter,
} from "@/types/users";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/pagination";
import { Alert } from "@/components/ui/alert";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  UserFilters,
  UserTable,
  UserCreateDialog,
  UserBulkCreateDialog,
  UserBulkInviteDialog,
  UserEditDialog,
  UserDeleteDialog,
  UserDetailDialog,
} from "@/components/users";

export default function UsersPage() {
  const router = useRouter();
  const toast = useToast();
  const { user: currentUser, isAdmin, isLoading: authLoading } = useAuth();

  // Filters state with dense page size for sensible viewport usage
  const [filters, setFilters] = useState<UserFilter>({
    page: 1,
    limit: 25,
    search: "",
    role: undefined,
    is_active: undefined,
  });

  // Query & Mutations
  const {
    data: usersResponse,
    isLoading: usersLoading,
    isFetching,
    refetch,
  } = useUsers(filters);

  const createUserMutation = useCreateUser();
  const bulkCreateMutation = useBulkCreateUsers();
  const updateUserMutation = useUpdateUser();
  const deleteUserMutation = useDeleteUser();
  const sendInviteMutation = useSendInvite();
  const bulkInviteMutation = useBulkSendInvite();
  const resetPasswordMutation = useTriggerResetPassword();

  // Dialog states
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isBulkCreateOpen, setIsBulkCreateOpen] = useState(false);
  const [isBulkInviteOpen, setIsBulkInviteOpen] = useState(false);
  const [viewingUser, setViewingUser] = useState<UserDetailResponse | null>(null);
  const [editingUser, setEditingUser] = useState<UserDetailResponse | null>(null);
  const [deletingUser, setDeletingUser] = useState<UserDetailResponse | null>(null);
  const [actionInProgressId, setActionInProgressId] = useState<string | null>(null);

  // Derive summary metrics from user records
  const metrics = useMemo(() => {
    const data = usersResponse?.data || [];
    const total = usersResponse?.total_count ?? 0;
    const active = data.filter((u) => u.is_active).length;
    const pendingSetup = data.filter((u) => !u.is_password_configured).length;
    const staff = data.filter((u) => u.role !== "participant").length;
    return { total, active, pendingSetup, staff };
  }, [usersResponse]);

  // Guards
  useEffect(() => {
    if (!authLoading && !currentUser) {
      router.replace("/login");
    }
  }, [authLoading, currentUser, router]);

  if (authLoading || !currentUser) {
    return (
      <div className="font-mono">
        <SkeletonCard />
      </div>
    );
  }

  if (!isAdmin) {
    return (
      <div className="font-sans space-y-4">
        <Alert variant="error" title="Access restricted">
          <div>
            You do not have administrative privileges to manage user accounts.
          </div>
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

  // Handlers
  const handleCreateUser = async (data: CreateUserRequest) => {
    try {
      const created = await createUserMutation.mutateAsync(data);
      toast.success(
        `User ${created.email} provisioned successfully.`,
        "User created"
      );
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to create user";
      toast.error(errorMsg, "Could not create user");
      throw err;
    }
  };

  const handleBulkCreate = async (payload: BulkCreateUsersRequest) => {
    try {
      const res = await bulkCreateMutation.mutateAsync(payload);
      toast.success(
        `Successfully imported ${res.total_created} of ${res.total_processed} accounts.`,
        "Import complete"
      );
      return res;
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Bulk import failed";
      toast.error(errorMsg, "Import error");
      throw err;
    }
  };

  const handleUpdateUser = async (id: string, payload: UpdateUserRequest) => {
    try {
      await updateUserMutation.mutateAsync({ id, payload });
      toast.success("User account details updated.", "Account updated");
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Update failed";
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  const handleDeleteUser = async (id: string) => {
    try {
      await deleteUserMutation.mutateAsync(id);
      toast.info(
        "User removed and session invalidated.",
        "Account removed"
      );
      setDeletingUser(null);
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Deletion failed";
      toast.error(errorMsg, "Error");
    }
  };

  const handleSendInvite = async (targetUser: UserDetailResponse) => {
    setActionInProgressId(targetUser.id);
    try {
      await sendInviteMutation.mutateAsync(targetUser.id);
      toast.success(
        `Onboarding invite email dispatched to ${targetUser.email}`,
        "Invite sent"
      );
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to send invite";
      toast.error(errorMsg, "Invite failed");
    } finally {
      setActionInProgressId(null);
    }
  };

  const handleBulkInvite = async (payload: BulkInviteRequest) => {
    try {
      const res = await bulkInviteMutation.mutateAsync(payload);
      const count =
        typeof res === "object" && res && "invites_dispatched" in res
          ? (res as { invites_dispatched: number }).invites_dispatched
          : 0;
      toast.success(
        `Dispatched setup invites to unconfigured users.`,
        "Invites queued"
      );
      return count;
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Bulk invite dispatch failed";
      toast.error(errorMsg, "Error");
      throw err;
    }
  };

  const handleTriggerResetPassword = async (targetUser: UserDetailResponse) => {
    setActionInProgressId(targetUser.id);
    try {
      await resetPasswordMutation.mutateAsync(targetUser.id);
      toast.success(
        `Password recovery link emailed to ${targetUser.email}`,
        "Password reset sent"
      );
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Password reset trigger failed";
      toast.error(errorMsg, "Error");
    } finally {
      setActionInProgressId(null);
    }
  };

  return (
    <div className="space-y-6 font-sans">
      {/* Level 1: Page Header directly on canvas */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-4 border-b border-[var(--card-line)]">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Users
            </h1>
            {usersResponse?.total_count !== undefined && (
              <Badge variant="subtle" size="sm">
                {usersResponse.total_count} {usersResponse.total_count === 1 ? "user" : "users"}
              </Badge>
            )}
          </div>
          <p className="text-xs text-[var(--ink-secondary)] mt-1">
            Directory of participants, judges, volunteers, and organizers.
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
            title="Refresh User Data"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsBulkInviteOpen(true)}
            className="h-9 px-3 text-xs gap-1.5"
          >
            <Mail className="h-3.5 w-3.5" />
            <span>Send invites</span>
          </Button>

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
            <span>Add user</span>
          </Button>
        </div>
      </div>

      {/* Purposeful Operational Metrics Strip */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3.5">
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <span className="text-[11px] font-medium text-[var(--ink-secondary)]">Total accounts</span>
          <div className="text-xl font-bold text-[var(--ink-primary)] mt-1">{metrics.total}</div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">Active accounts</span>
            <span className="h-2 w-2 rounded-full bg-emerald-500" />
          </div>
          <div className="text-xl font-bold text-[var(--ink-primary)] mt-1">{metrics.active}</div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">Pending setup</span>
            <span className="h-2 w-2 rounded-full bg-amber-500" />
          </div>
          <div className="text-xl font-bold text-[var(--ink-primary)] mt-1">{metrics.pendingSetup}</div>
        </div>
        <div className="p-3.5 rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-xs">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-[var(--ink-secondary)]">Staff & evaluators</span>
            <span className="h-2 w-2 rounded-full bg-indigo-500" />
          </div>
          <div className="text-xl font-bold text-[var(--ink-primary)] mt-1">{metrics.staff}</div>
        </div>
      </div>

      {/* Inline Filters */}
      <UserFilters
        filters={filters}
        onChange={(newFilters) => setFilters(newFilters)}
        onReset={() =>
          setFilters({
            page: 1,
            limit: filters.limit,
            search: "",
            role: undefined,
            is_active: undefined,
          })
        }
      />

      {/* Level 2: Primary Surface (The Table) */}
      <UserTable
        users={usersResponse?.data ?? []}
        isLoading={usersLoading}
        onView={(user) => setViewingUser(user)}
        onEdit={(user) => setEditingUser(user)}
        onDelete={(user) => setDeletingUser(user)}
        onSendInvite={handleSendInvite}
        onResetPassword={handleTriggerResetPassword}
        actionInProgressId={actionInProgressId}
      />

      {/* Level 3: Pagination Controls */}
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

      {/* Dialogs */}
      <UserCreateDialog
        open={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSubmit={handleCreateUser}
        isLoading={createUserMutation.isPending}
      />

      <UserBulkCreateDialog
        open={isBulkCreateOpen}
        onClose={() => setIsBulkCreateOpen(false)}
        onSubmit={handleBulkCreate}
        isLoading={bulkCreateMutation.isPending}
      />

      <UserBulkInviteDialog
        open={isBulkInviteOpen}
        onClose={() => setIsBulkInviteOpen(false)}
        onSubmit={handleBulkInvite}
        isLoading={bulkInviteMutation.isPending}
      />

      <UserEditDialog
        user={editingUser}
        open={Boolean(editingUser)}
        onClose={() => setEditingUser(null)}
        onSubmit={handleUpdateUser}
        isLoading={updateUserMutation.isPending}
      />

      <UserDeleteDialog
        user={deletingUser}
        open={Boolean(deletingUser)}
        onClose={() => setDeletingUser(null)}
        onConfirm={handleDeleteUser}
        isLoading={deleteUserMutation.isPending}
      />

      <UserDetailDialog
        user={viewingUser}
        open={Boolean(viewingUser)}
        onClose={() => setViewingUser(null)}
        onEdit={(user) => {
          setViewingUser(null);
          setEditingUser(user);
        }}
        onDelete={(user) => {
          setViewingUser(null);
          setDeletingUser(user);
        }}
        onSendInvite={handleSendInvite}
        onResetPassword={handleTriggerResetPassword}
        actionInProgress={Boolean(actionInProgressId)}
      />
    </div>
  );
}
