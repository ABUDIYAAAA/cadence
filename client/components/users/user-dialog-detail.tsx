"use client";

import React from "react";
import { UserDetailResponse } from "@/types/users";
import { getUserTableColumnVisibility } from "@/lib/competition-utils";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Edit2, Trash2, Ban, ShieldCheck, RotateCcw } from "lucide-react";

interface UserDetailDialogProps {
  user: UserDetailResponse | null;
  open: boolean;
  mode?: "event" | "platform";
  onClose: () => void;
  onEdit: (user: UserDetailResponse) => void;
  onDelete: (user: UserDetailResponse) => void;
  onToggleRestriction?: (user: UserDetailResponse) => void;
  onReactivate?: (user: UserDetailResponse) => void;
  /** Volunteers can only view. */
  canManage?: boolean;
  actionInProgress?: boolean;
}

export function UserDetailDialog({
  user,
  open,
  mode = "event",
  onClose,
  onEdit,
  onDelete,
  onToggleRestriction,
  onReactivate,
  canManage = true,
  actionInProgress,
}: UserDetailDialogProps) {
  if (!user) return null;

  const isDeleted = Boolean(user.deleted_at);
  const isRestricted =
    mode === "event" ? Boolean(user.is_banned) : !user.is_active;
  const isOwnerRow = mode === "event" && user.role === "owner";
  const { showTeam } = getUserTableColumnVisibility(mode);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={mode === "event" ? "Event Member Inspector" : "User Inspector"}
      description={`Record ID: ${user.id}`}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2 w-full">
          <div className="flex items-center gap-1.5">
            {canManage && isDeleted && onReactivate && (
              <Button
                variant="outline"
                size="sm"
                disabled={actionInProgress}
                onClick={() => {
                  onClose();
                  onReactivate(user);
                }}
                className="text-xs gap-1.5"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Reactivate User
              </Button>
            )}
            {canManage && !isDeleted && !isOwnerRow && onToggleRestriction && (
              <Button
                variant="outline"
                size="sm"
                disabled={actionInProgress}
                onClick={() => onToggleRestriction(user)}
                className="text-xs gap-1.5"
              >
                {isRestricted ? (
                  <>
                    <ShieldCheck className="h-3.5 w-3.5" />
                    {mode === "event" ? "Unban Member" : "Unblacklist User"}
                  </>
                ) : (
                  <>
                    <Ban className="h-3.5 w-3.5" />
                    {mode === "event" ? "Ban from Event" : "Blacklist User"}
                  </>
                )}
              </Button>
            )}
          </div>

          {canManage && !isDeleted && !isOwnerRow && (
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  onClose();
                  onEdit(user);
                }}
                className="text-xs gap-1"
              >
                <Edit2 className="h-3.5 w-3.5" />
                Edit
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => {
                  onClose();
                  onDelete(user);
                }}
                className="text-xs gap-1"
              >
                <Trash2 className="h-3.5 w-3.5" />
                {mode === "event" ? "Remove" : "Delete"}
              </Button>
            </div>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs">
        <div className="rounded-[16px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] overflow-hidden bg-[var(--card-solid)]">
          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Name</span>
            <span className="font-bold text-[var(--ink-primary)]">
              {user.name}
            </span>
          </div>

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Email</span>
            <span className="font-semibold text-[var(--ink-primary)] font-mono">
              {user.email}
            </span>
          </div>

          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">
              {mode === "event" ? "Event Role" : "Role"}
            </span>
            <Badge variant="solid" size="sm" className="capitalize">
              {user.role}
            </Badge>
          </div>

          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Status</span>
            <span className="font-bold text-[var(--ink-primary)]">
              {isDeleted
                ? "Removed"
                : isRestricted
                ? mode === "event"
                  ? "Banned from Event"
                  : "Blacklisted"
                : "Active"}
            </span>
          </div>

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Phone Number</span>
            <span className="text-[var(--ink-primary)]">
              {user.phone_number || "— Not Provided"}
            </span>
          </div>

          {showTeam && (
            <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)]">Assigned Team</span>
              <span className="text-[var(--ink-primary)]">
                {user.team_name ? (
                  <>
                    <strong className="text-[var(--ink-primary)]">
                      {user.team_name}
                    </strong>
                    {user.is_leader && " (Leader)"}
                  </>
                ) : (
                  <span className="text-[var(--ink-tertiary)]">
                    — Unassigned
                  </span>
                )}
              </span>
            </div>
          )}

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Created At</span>
            <span className="text-[var(--ink-primary)] font-mono">
              {new Date(user.created_at).toLocaleString()}
            </span>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
