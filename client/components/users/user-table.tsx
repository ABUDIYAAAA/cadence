"use client";

import React from "react";
import {
  KeyRound,
  Edit2,
  Trash2,
  Eye,
  Crown,
  Check,
  Clock,
  RotateCcw,
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
  onView: (user: UserDetailResponse) => void;
  onEdit: (user: UserDetailResponse) => void;
  onDelete: (user: UserDetailResponse) => void;
  onResetPassword: (user: UserDetailResponse) => void;
  /** Reactivates a removed user; shown only for rows with deleted_at. */
  onReactivate?: (user: UserDetailResponse) => void;
  /** Admins get every action; volunteers can only view. */
  canManage?: boolean;
  actionInProgressId?: string | null;
}

export function UserTable({
  users,
  isLoading,
  onView,
  onEdit,
  onDelete,
  onResetPassword,
  onReactivate,
  canManage = true,
  actionInProgressId,
}: UserTableProps) {
  if (isLoading) {
    return (
      <div className="space-y-3 font-sans">
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="flex items-center space-x-4 py-3 px-4 border-b border-[var(--card-line)]">
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
        title="No users found"
        description="No users matched your current query or filter criteria. Try adjusting your filters or create a new user account."
      />
    );
  }

  const renderRoleBadge = (role: string) => {
    switch (role) {
      case "admin":
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-[4px] text-[11px] font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-900/60 capitalize">
            {role}
          </span>
        );
      case "judge":
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-[4px] text-[11px] font-semibold bg-amber-50 text-amber-700 border border-amber-200 dark:bg-amber-950/50 dark:text-amber-300 dark:border-amber-900/60 capitalize">
            {role}
          </span>
        );
      case "volunteer":
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-[4px] text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-emerald-950/50 dark:text-emerald-300 dark:border-emerald-900/60 capitalize">
            {role}
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-[4px] text-[11px] font-medium bg-slate-100 text-slate-700 border border-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700 capitalize">
            {role}
          </span>
        );
    }
  };

  return (
    <div className="w-full font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>User</TableHead>
            <TableHead>Role</TableHead>
            <TableHead>Team</TableHead>
            <TableHead>Password setup</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => {
            const isProcessing = actionInProgressId === user.id;

            return (
              <TableRow key={user.id} className="group">
                {/* User Name & Email */}
                <TableCell>
                  <div className="flex flex-col">
                    <div className="flex items-center gap-1.5 font-semibold text-[var(--ink-primary)]">
                      <span>{user.name}</span>
                      {user.is_leader && (
                        <span
                          title="Team Lead"
                          className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.2 rounded-[4px] font-semibold bg-amber-50 text-amber-700 border border-amber-200 dark:bg-amber-950/50 dark:text-amber-300 dark:border-amber-900/60"
                        >
                          <Crown className="h-2.5 w-2.5" />
                          Lead
                        </span>
                      )}
                    </div>
                    <span className="text-xs text-[var(--ink-secondary)] font-mono">
                      {user.email}
                    </span>
                    {user.phone_number && (
                      <span className="text-[10px] text-[var(--ink-tertiary)]">
                        {user.phone_number}
                      </span>
                    )}
                  </div>
                </TableCell>

                {/* Role with distinctive styling */}
                <TableCell>
                  {renderRoleBadge(user.role)}
                </TableCell>

                {/* Team */}
                <TableCell className="text-xs">
                  {user.team_name ? (
                    <span className="font-medium text-[var(--ink-primary)]">
                      {user.team_name}
                    </span>
                  ) : (
                    <span className="text-[var(--ink-tertiary)]">Unassigned</span>
                  )}
                </TableCell>

                {/* Password / Onboarding */}
                <TableCell>
                  {user.is_password_configured ? (
                    <span className="inline-flex items-center gap-1 text-xs text-[var(--ink-secondary)] font-medium">
                      <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                      Configured
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400 font-medium">
                      <Clock className="h-3 w-3" />
                      Password not set
                    </span>
                  )}
                </TableCell>

                {/* Status */}
                <TableCell>
                  <Badge
                    variant={user.is_active ? "emerald" : "subtle"}
                    dot
                    size="sm"
                  >
                    {user.deleted_at ? "Removed" : user.is_active ? "Active" : "Inactive"}
                  </Badge>
                </TableCell>

                {/* Created */}
                <TableCell className="text-xs text-[var(--ink-secondary)] font-mono whitespace-nowrap">
                  {new Date(user.created_at).toLocaleDateString()}
                </TableCell>

                {/* Actions */}
                <TableCell className="text-right whitespace-nowrap">
                  <div className="flex items-center justify-end gap-1">
                    {/* View Details */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onView(user)}
                      title="Inspect user details"
                      className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                    >
                      <Eye className="h-3.5 w-3.5" />
                    </Button>

                    {/* Removed users can only be reactivated */}
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

                    {/* Edit User */}
                    {canManage && !user.deleted_at && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onEdit(user)}
                      title="Edit user"
                      className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                    >
                      <Edit2 className="h-3.5 w-3.5" />
                    </Button>
                    )}

                    {/* Trigger Password Reset */}
                    {canManage && !user.deleted_at && (
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={isProcessing}
                      onClick={() => onResetPassword(user)}
                      title="Send password reset link"
                      className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                    >
                      <KeyRound className="h-3.5 w-3.5" />
                    </Button>
                    )}

                    {/* Delete User */}
                    {canManage && !user.deleted_at && (
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={isProcessing}
                      onClick={() => onDelete(user)}
                      title="Delete user"
                      className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--color-coral)]"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
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
