"use client";

import React from "react";
import {
  Edit2,
  Trash2,
  Eye,
  Crown,
  RotateCcw,
  Ban,
  ShieldCheck,
} from "lucide-react";
import { UserDetailResponse } from "@/types/users";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";

interface UserTableProps {
  users: UserDetailResponse[];
  isLoading: boolean;
  mode?: "event" | "platform";
  onView: (user: UserDetailResponse) => void;
  onEdit: (user: UserDetailResponse) => void;
  onDelete: (user: UserDetailResponse) => void;
  /** Event mode: ban/unban from active event. Platform mode: blacklist/unblacklist globally. */
  onToggleRestriction?: (user: UserDetailResponse) => void;
  /** Reactivates a removed user; shown only for rows with deleted_at in platform mode. */
  onReactivate?: (user: UserDetailResponse) => void;
  /** Admins/owners/devs get management actions; volunteers can only view. */
  canManage?: boolean;
  actionInProgressId?: string | null;
}

export function UserTable({
  users,
  isLoading,
  mode = "event",
  onView,
  onEdit,
  onDelete,
  onToggleRestriction,
  onReactivate,
  canManage = true,
  actionInProgressId,
}: UserTableProps) {
  if (isLoading) {
    return (
      <div className="space-y-3 font-sans">
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center space-x-4 py-3 px-4 border-b border-[var(--card-line)]"
            >
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/4" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (users.length === 0) {
    return (
      <EmptyState
        title={mode === "event" ? "No event members found" : "No users found"}
        description="No records matched your current filter criteria. Try adjusting your filters or adding a member."
      />
    );
  }

  const renderRoleBadge = (role: string) => {
    switch (role) {
      case "dev":
        return (
          <Badge variant="amber" size="sm" className="uppercase">
            {role}
          </Badge>
        );
      case "owner":
        return (
          <Badge variant="blue" size="sm" className="capitalize gap-1">
            <Crown className="h-2.5 w-2.5" />
            {role}
          </Badge>
        );
      case "admin":
        return (
          <Badge variant="solid" size="sm" className="capitalize">
            {role}
          </Badge>
        );
      case "judge":
        return (
          <Badge variant="amber" size="sm" className="capitalize">
            {role}
          </Badge>
        );
      case "volunteer":
        return (
          <Badge variant="emerald" size="sm" className="capitalize">
            {role}
          </Badge>
        );
      default:
        return (
          <Badge variant="subtle" size="sm" className="capitalize">
            {role}
          </Badge>
        );
    }
  };

  return (
    <div className="w-full font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{mode === "event" ? "Member" : "User"}</TableHead>
            <TableHead>Role</TableHead>
            <TableHead>Team</TableHead>
            <TableHead>Check-in</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => {
            const isProcessing = actionInProgressId === user.id;
            const isRestricted =
              mode === "event" ? Boolean(user.is_banned) : !user.is_active;
            const isOwnerRow = mode === "event" && user.role === "owner";

            return (
              <TableRow key={user.id} className="group">
                {/* User Name & Email */}
                <TableCell>
                  <div className="flex flex-col">
                    <div className="flex items-center gap-1.5 font-semibold text-[var(--ink-primary)]">
                      <span>{user.name}</span>
                      {user.role === "dev" && (
                        <Badge variant="amber" size="sm">
                          dev
                        </Badge>
                      )}
                      {user.is_leader && (
                        <Badge variant="amber" size="sm" className="gap-1">
                          <Crown className="h-2.5 w-2.5" />
                          Lead
                        </Badge>
                      )}
                    </div>
                    <span className="text-xs text-[var(--ink-secondary)] font-mono">
                      {user.email}
                    </span>
                    {user.phone_number && (
                      <span className="text-[11px] text-[var(--ink-tertiary)]">
                        {user.phone_number}
                      </span>
                    )}
                  </div>
                </TableCell>

                {/* Role */}
                <TableCell>
                  <div className="flex flex-wrap items-center gap-1">
                    {renderRoleBadge(user.role)}
                  </div>
                </TableCell>

                {/* Team */}
                <TableCell className="text-xs">
                  {user.team_name ? (
                    <span className="font-medium text-[var(--ink-primary)]">
                      {user.team_name}
                    </span>
                  ) : (
                    <span className="text-[var(--ink-tertiary)]">
                      Unassigned
                    </span>
                  )}
                </TableCell>

                {/* Check-in status */}
                <TableCell className="text-xs">
                  {user.checkin_status === "checked_in" ? (
                    <Badge variant="emerald" size="sm">
                      Checked in
                    </Badge>
                  ) : user.checkin_status === "not_checked_in" ? (
                    <Badge variant="subtle" size="sm">
                      Ticket issued
                    </Badge>
                  ) : (
                    <span className="text-[var(--ink-tertiary)]">No ticket</span>
                  )}
                </TableCell>

                {/* Status */}
                <TableCell>
                  <Badge
                    variant={
                      user.deleted_at || isRestricted ? "coral" : "emerald"
                    }
                    dot
                    size="sm"
                  >
                    {user.deleted_at
                      ? "Removed"
                      : isRestricted
                      ? mode === "event"
                        ? "Banned"
                        : "Blacklisted"
                      : "Active"}
                  </Badge>
                </TableCell>

                {/* Created */}
                <TableCell className="text-xs text-[var(--ink-secondary)] font-mono whitespace-nowrap">
                  {new Date(user.created_at).toLocaleDateString()}
                </TableCell>

                {/* Actions */}
                <TableCell className="text-right whitespace-nowrap">
                  <div className="flex items-center justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onView(user)}
                      title="Inspect details"
                      className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                    >
                      <Eye className="h-3.5 w-3.5" />
                    </Button>

                    {canManage && user.deleted_at && onReactivate && (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={isProcessing}
                        onClick={() => onReactivate(user)}
                        title="Reactivate user"
                        className="h-7 px-2 text-xs gap-1"
                      >
                        <RotateCcw className="h-3 w-3" />
                        Reactivate
                      </Button>
                    )}

                    {canManage && !user.deleted_at && !isOwnerRow && (
                      <>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => onEdit(user)}
                          title={
                            mode === "event"
                              ? "Change member role"
                              : "Edit user"
                          }
                          className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                        >
                          <Edit2 className="h-3.5 w-3.5" />
                        </Button>

                        {onToggleRestriction && (
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={isProcessing}
                            onClick={() => onToggleRestriction(user)}
                            title={
                              isRestricted
                                ? mode === "event"
                                  ? "Unban member from event"
                                  : "Unblacklist user"
                                : mode === "event"
                                ? "Ban member from event"
                                : "Blacklist user globally"
                            }
                            className={`h-7 w-7 p-0 ${
                              isRestricted
                                ? "text-[var(--color-emerald)]"
                                : "text-[var(--ink-secondary)] hover:text-[var(--color-coral)]"
                            }`}
                          >
                            {isRestricted ? (
                              <ShieldCheck className="h-3.5 w-3.5" />
                            ) : (
                              <Ban className="h-3.5 w-3.5" />
                            )}
                          </Button>
                        )}

                        <Button
                          variant="ghost"
                          size="sm"
                          disabled={isProcessing}
                          onClick={() => onDelete(user)}
                          title={
                            mode === "event"
                              ? "Remove from event"
                              : "Delete user"
                          }
                          className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--color-coral)]"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
