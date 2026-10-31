"use client";

import React from "react";
import { UserDetailResponse } from "@/types/users";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Mail, KeyRound, Edit2, Trash2 } from "lucide-react";

interface UserDetailDialogProps {
  user: UserDetailResponse | null;
  open: boolean;
  onClose: () => void;
  onEdit: (user: UserDetailResponse) => void;
  onDelete: (user: UserDetailResponse) => void;
  onSendInvite: (user: UserDetailResponse) => void;
  onResetPassword: (user: UserDetailResponse) => void;
  /** Volunteers can only view and invite participants. */
  canManage?: boolean;
  actionInProgress?: boolean;
}

export function UserDetailDialog({
  user,
  open,
  onClose,
  onEdit,
  onDelete,
  onSendInvite,
  onResetPassword,
  canManage = true,
  actionInProgress,
}: UserDetailDialogProps) {
  if (!user) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="User Profile Inspector"
      description={`System record ID: ${user.id}`}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2 w-full">
          <div className="flex items-center gap-1.5">
            {!user.is_password_configured && (canManage || user.role === "participant") && (
              <Button
                variant="outline"
                size="sm"
                disabled={actionInProgress}
                onClick={() => onSendInvite(user)}
                className="text-xs gap-1.5"
              >
                <Mail className="h-3.5 w-3.5" />
                Send Setup Invite
              </Button>
            )}
            {canManage && (
              <Button
                variant="outline"
                size="sm"
                disabled={actionInProgress}
                onClick={() => onResetPassword(user)}
                className="text-xs gap-1.5"
              >
                <KeyRound className="h-3.5 w-3.5" />
                Reset Password
              </Button>
            )}
          </div>

          {canManage && (
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
              Delete
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
            <span className="font-bold text-[var(--ink-primary)]">{user.name}</span>
          </div>

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Email</span>
            <span className="font-semibold text-[var(--ink-primary)] font-mono">{user.email}</span>
          </div>

          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Role</span>
            <Badge variant="solid" size="sm">
              {user.role}
            </Badge>
          </div>

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Account Status</span>
            <span className="font-bold text-[var(--ink-primary)]">
              {user.is_active ? "Active" : "Deactivated"}
            </span>
          </div>

          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Phone Number</span>
            <span className="text-[var(--ink-primary)]">{user.phone_number || "— Not Provided"}</span>
          </div>

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Assigned Team</span>
            <span className="text-[var(--ink-primary)]">
              {user.team_name ? (
                <>
                  <strong className="text-[var(--ink-primary)]">{user.team_name}</strong>
                  {user.is_leader && " (Leader)"}
                </>
              ) : (
                <span className="text-[var(--ink-tertiary)] italic">— Unassigned</span>
              )}
            </span>
          </div>

          {user.team_id && (
            <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)]">Team UUID</span>
              <span className="text-[10px] text-[var(--ink-secondary)] font-mono break-all">
                {user.team_id}
              </span>
            </div>
          )}

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Password Configuration</span>
            <span className="font-semibold text-[var(--ink-primary)]">
              {user.is_password_configured ? "Configured" : "Pending Onboarding Setup"}
            </span>
          </div>

          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Created At</span>
            <span className="text-[var(--ink-primary)] font-mono">{new Date(user.created_at).toLocaleString()}</span>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
