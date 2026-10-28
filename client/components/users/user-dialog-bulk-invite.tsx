"use client";

import React, { useState } from "react";
import { BulkInviteRequest } from "@/types/users";
import { useUsers } from "@/hooks/use-users";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Mail, CheckCircle2, Filter } from "lucide-react";

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
  const { data: usersData, isLoading: usersLoading } = useUsers({ limit: 1000 });
  const [excludeDomain, setExcludeDomain] = useState("");
  const [dispatchedCount, setDispatchedCount] = useState<number | null>(null);

  const resetDialog = () => {
    setExcludeDomain("");
    setDispatchedCount(null);
  };

  const handleClose = () => {
    resetDialog();
    onClose();
  };

  // Filter unconfigured users
  const unconfiguredUsers = (usersData?.data || []).filter(
    (u) => !u.is_password_configured && u.is_active
  );

  const normalizedDomain = excludeDomain.trim().toLowerCase().replace(/^@/, "");

  const excludedCount = normalizedDomain
    ? unconfiguredUsers.filter((u) =>
        u.email.toLowerCase().endsWith("@" + normalizedDomain) ||
        u.email.toLowerCase().endsWith("." + normalizedDomain)
      ).length
    : 0;

  const eligibleUserIds = normalizedDomain
    ? unconfiguredUsers
        .filter(
          (u) =>
            !u.email.toLowerCase().endsWith("@" + normalizedDomain) &&
            !u.email.toLowerCase().endsWith("." + normalizedDomain)
        )
        .map((u) => u.id)
    : unconfiguredUsers.map((u) => u.id);

  const handleSend = async () => {
    try {
      const payload: BulkInviteRequest = normalizedDomain
        ? { user_ids: eligibleUserIds }
        : { all_uninvited: true };

      const count = await onSubmit(payload);
      setDispatchedCount(count);
    } catch {
      // handled by toast in parent
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
              onClick={handleSend}
              isLoading={isLoading || usersLoading}
              disabled={eligibleUserIds.length === 0}
              className="gap-1.5"
            >
              <Mail className="h-3.5 w-3.5" />
              <span>
                Dispatch Setup Emails ({eligibleUserIds.length})
              </span>
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
                Onboarding links with secure 48-hour password setup tokens have been dispatched.
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="border border-neutral-300 p-3 bg-neutral-50 space-y-2">
              <p className="font-bold text-black">Target Resolution:</p>
              <p className="text-neutral-600 leading-relaxed">
                Scans all active accounts where{" "}
                <code className="bg-neutral-200 px-1 py-0.5 text-black">
                  is_password_configured = false
                </code>.
              </p>
            </div>

            {/* Excluded Domain Filter */}
            <div className="border border-neutral-300 p-3 bg-white space-y-2">
              <div className="flex items-center gap-1.5 text-black font-bold uppercase text-[11px]">
                <Filter className="h-3.5 w-3.5" />
                <span>Exclude Specific Email Domain (Optional)</span>
              </div>
              <Input
                placeholder="e.g. test.com or company.internal (no emails sent to this domain)"
                value={excludeDomain}
                onChange={(e) => setExcludeDomain(e.target.value)}
                disabled={isLoading}
                className="text-xs"
              />
              <p className="text-[10px] text-neutral-500">
                Any unconfigured accounts ending with this domain will be excluded from the mailout.
              </p>
            </div>

            {/* Summary statistics bar */}
            <div className="border border-neutral-200 p-2.5 bg-neutral-50 flex items-center justify-between text-[11px]">
              <div>
                <span className="text-neutral-500">Unconfigured Total: </span>
                <strong className="text-black">{unconfiguredUsers.length}</strong>
              </div>
              {normalizedDomain && (
                <div>
                  <span className="text-neutral-500">Excluded ({normalizedDomain}): </span>
                  <strong className="text-neutral-800">{excludedCount}</strong>
                </div>
              )}
              <div>
                <span className="text-neutral-500">Net To Send: </span>
                <strong className="text-black">{eligibleUserIds.length}</strong>
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
