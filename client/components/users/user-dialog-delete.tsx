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
      <div className="space-y-4 font-mono text-xs">
        <div className="border border-black p-3 bg-neutral-100 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-black shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-bold text-black">
              Immediate Session Eviction & Soft Deletion
            </p>
            <p className="text-neutral-600 leading-relaxed">
              Deleting this account will immediately revoke all active JWT tokens
              via the distributed Redis blocklist (&lt;1ms). The user will be
              instantly disconnected.
            </p>
          </div>
        </div>

        <div className="border border-neutral-300 p-3 bg-neutral-50 space-y-1">
          <div>
            <span className="text-neutral-500">Target Name: </span>
            <strong className="text-black">{user.name}</strong>
          </div>
          <div>
            <span className="text-neutral-500">Target Email: </span>
            <strong className="text-black">{user.email}</strong>
          </div>
          <div>
            <span className="text-neutral-500">Role: </span>
            <span className="uppercase font-semibold">{user.role}</span>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
