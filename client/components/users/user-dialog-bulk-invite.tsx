"use client";

import React, { useState } from "react";
import { BulkInviteRequest } from "@/types/users";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Mail, CheckCircle2 } from "lucide-react";

interface UserBulkInviteDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (payload: BulkInviteRequest) => Promise<number>;
  isLoading: boolean;
}

export function UserBulkInviteDialog({
  open,
  onClose,
  onSubmit,
  isLoading,
}: UserBulkInviteDialogProps) {
  const [dispatchedCount, setDispatchedCount] = useState<number | null>(null);

  const handleClose = () => {
    setDispatchedCount(null);
    onClose();
  };

  const handleSendAll = async () => {
    try {
      const count = await onSubmit({ all_uninvited: true });
      setDispatchedCount(count);
    } catch {
      // error handled by toast in parent
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Bulk Dispatch Setup Invites"
      description="Send email invitations to users who have not yet configured their passwords."
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isLoading}
          >
            {dispatchedCount !== null ? "Done" : "Cancel"}
          </Button>

          {dispatchedCount === null && (
            <Button
              variant="primary"
              size="sm"
              onClick={handleSendAll}
              isLoading={isLoading}
              className="gap-1.5"
            >
              <Mail className="h-3.5 w-3.5" />
              Dispatch to All Unconfigured
            </Button>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs">
        {dispatchedCount !== null ? (
          <div className="border border-black p-4 bg-neutral-100 flex items-center gap-3">
            <CheckCircle2 className="h-5 w-5 text-black shrink-0" />
            <div>
              <p className="font-bold text-black">
                {dispatchedCount} Invitation{dispatchedCount === 1 ? "" : "s"} Dispatched
              </p>
              <p className="text-neutral-600 mt-0.5">
                Onboarding links with secure password configuration tokens have been queued and sent.
              </p>
            </div>
          </div>
        ) : (
          <div className="border border-neutral-300 p-3 bg-neutral-50 space-y-2">
            <p className="font-bold text-black">Automatic Target Resolution:</p>
            <p className="text-neutral-600 leading-relaxed">
              This action scans all user accounts in the system where{" "}
              <code className="bg-neutral-200 px-1 py-0.5 text-black">
                is_password_configured = false
              </code>{" "}
              and sends each an onboarding email containing a unique 24-hour setup token.
            </p>
          </div>
        )}
      </div>
    </Dialog>
  );
}
