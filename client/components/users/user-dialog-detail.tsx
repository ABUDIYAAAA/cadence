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
            {!user.is_password_configured && (
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
          </div>

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
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs">
        <div className="border border-neutral-300 divide-y divide-neutral-200">
          <div className="flex justify-between p-2.5 bg-neutral-50">
            <span className="text-neutral-500">Name</span>
            <span className="font-bold text-black">{user.name}</span>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Email</span>
            <span className="font-semibold text-black">{user.email}</span>
          </div>

          <div className="flex justify-between p-2.5 bg-neutral-50">
            <span className="text-neutral-500">Role</span>
            <Badge variant="solid" size="sm">
              {user.role}
            </Badge>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Account Status</span>
            <span className="font-bold">
              {user.is_active ? "Active" : "Deactivated"}
            </span>
          </div>

          <div className="flex justify-between p-2.5 bg-neutral-50">
            <span className="text-neutral-500">Phone Number</span>
            <span>{user.phone_number || "— Not Provided"}</span>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Assigned Team</span>
            <span>
              {user.team_name ? (
                <>
                  <strong className="text-black">{user.team_name}</strong>
                  {user.is_leader && " (Leader)"}
                </>
              ) : (
                "— Unassigned"
              )}
            </span>
          </div>

          {user.team_id && (
            <div className="flex justify-between p-2.5 bg-neutral-50">
              <span className="text-neutral-500">Team UUID</span>
              <span className="text-[10px] text-neutral-600 break-all">
                {user.team_id}
              </span>
            </div>
          )}

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Password Configuration</span>
            <span>
              {user.is_password_configured ? "Configured" : "Pending Onboarding Setup"}
            </span>
          </div>

          <div className="flex justify-between p-2.5 bg-neutral-50">
            <span className="text-neutral-500">Created At</span>
            <span>{new Date(user.created_at).toLocaleString()}</span>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
