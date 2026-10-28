"use client";

import React from "react";
import { UserDetailResponse } from "@/types/users";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

interface UserDeleteDialogProps {
  user: UserDetailResponse | null;
  open: boolean;
  onClose: () => void;
  onConfirm: (id: string) => Promise<void>;
  isLoading: boolean;
}

export function UserDeleteDialog({
  user,
  open,
  onClose,
  onConfirm,
  isLoading,
}: UserDeleteDialogProps) {
  if (!user) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Confirm User Deletion"
      description="Irreversible administrative action"
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onConfirm(user.id)}
            isLoading={isLoading}
          >
            Permanently Remove User
          </Button>
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs">
        <div className="rounded-[16px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] p-3.5 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-[var(--color-coral)] shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-bold text-[var(--color-coral)]">
              Immediate Session Eviction & Soft Deletion
            </p>
            <p className="text-[var(--color-coral)] leading-relaxed">
              Deleting this account will immediately revoke all active JWT tokens
              via the distributed Redis blocklist (&lt;1ms). The user will be
              instantly disconnected.
            </p>
          </div>
        </div>

        <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] space-y-1.5">
          <div>
            <span className="text-[var(--ink-secondary)]">Target Name: </span>
            <strong className="text-[var(--ink-primary)]">{user.name}</strong>
          </div>
          <div>
            <span className="text-[var(--ink-secondary)]">Target Email: </span>
            <strong className="text-[var(--ink-primary)] font-mono">{user.email}</strong>
          </div>
          <div>
            <span className="text-[var(--ink-secondary)]">Role: </span>
            <span className="uppercase font-semibold text-[var(--ink-primary)]">{user.role}</span>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
