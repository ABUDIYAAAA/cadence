"use client";

import React, { useState } from "react";
import { BulkInviteRequest } from "@/types/users";
import { useAllUsers } from "@/hooks/use-users";
import { isEmailExcluded, parseEmailDomains } from "@/lib/utils";
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
  const { data: allUsers, isLoading: usersLoading } = useAllUsers({ enabled: open });
  const [excludeDomain, setExcludeDomain] = useState("");
  const [skipInvited, setSkipInvited] = useState(false);
  const [dispatchedCount, setDispatchedCount] = useState<number | null>(null);

  const resetDialog = () => {
    setExcludeDomain("");
    setSkipInvited(false);
    setDispatchedCount(null);
  };

  const handleClose = () => {
    resetDialog();
    onClose();
  };

  // Filter unconfigured users
  const unconfiguredUsers = (allUsers || []).filter(
    (u) => !u.is_password_configured && u.is_active
  );

  const alreadyInvitedCount = unconfiguredUsers.filter((u) => u.invite_count > 0).length;
  const neverInvitedCount = unconfiguredUsers.length - alreadyInvitedCount;

  const excludedDomains = parseEmailDomains(excludeDomain);

  const eligibleUserIds = unconfiguredUsers
    .filter((u) => !(skipInvited && u.invite_count > 0))
    .filter((u) => excludedDomains.length === 0 || !isEmailExcluded(u.email, excludedDomains))
    .map((u) => u.id);

  const excludedCount =
    excludedDomains.length > 0
      ? unconfiguredUsers.filter((u) => isEmailExcluded(u.email, excludedDomains)).length
      : 0;

  const handleSend = async () => {
    try {
      // The server applies the exclusion too, so it holds beyond the loaded list.
      const payload: BulkInviteRequest = {
        all_uninvited: true,
        ...(skipInvited && { skip_already_invited: true }),
        ...(excludedDomains.length > 0 && { exclude_domains: excludedDomains }),
      };

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
      <div className="space-y-4 font-sans text-xs">
        {dispatchedCount !== null ? (
          <div className="rounded-[16px] border border-[var(--color-emerald-border)] bg-[var(--color-emerald-dim)] p-4 flex items-center gap-3">
            <CheckCircle2 className="h-5 w-5 text-[#2EB88A] shrink-0" />
            <div>
              <p className="font-bold text-[#2EB88A]">
                {dispatchedCount} Invitation{dispatchedCount === 1 ? "" : "s"} Dispatched
              </p>
              <p className="text-[var(--color-emerald)] mt-0.5">
                Onboarding links with secure 48-hour password setup tokens have been dispatched.
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] space-y-2">
              <p className="font-bold text-[var(--ink-primary)]">Target Resolution:</p>
              <p className="text-[var(--ink-secondary)] leading-relaxed">
                Scans all active accounts where{" "}
                <code className="bg-[var(--card-line)] px-1.5 py-0.5 rounded text-[var(--ink-primary)] font-mono text-[11px]">
                  is_password_configured = false
                </code>.
              </p>
            </div>

            {/* Invite history */}
            <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid)] space-y-2">
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-[var(--ink-secondary)]">
                  Never invited:{" "}
                  <strong className="text-[var(--ink-primary)] font-mono">{neverInvitedCount}</strong>
                </span>
                <span className="text-[var(--ink-secondary)]">
                  Invited at least once:{" "}
                  <strong className="text-[var(--ink-primary)] font-mono">{alreadyInvitedCount}</strong>
                </span>
              </div>
              <label className="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={skipInvited}
                  onChange={(e) => setSkipInvited(e.target.checked)}
                  disabled={isLoading}
                  className="accent-[#83A2DB] h-4 w-4 rounded"
                />
                <span className="font-semibold text-[var(--ink-primary)]">
                  Skip people who&apos;ve already been invited
                </span>
              </label>
            </div>

            {/* Excluded Domain Filter */}
            <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid)] space-y-2">
              <div className="flex items-center gap-1.5 text-[var(--ink-primary)] font-semibold uppercase text-[11px]">
                <Filter className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Exclude Email Domains (Optional)</span>
              </div>
              <Input
                placeholder="e.g. test.com, company.internal"
                value={excludeDomain}
                onChange={(e) => setExcludeDomain(e.target.value)}
                disabled={isLoading}
                className="text-xs"
              />
              <p className="text-[10px] text-[var(--ink-secondary)]">
                Separate multiple domains with commas or spaces. Any unconfigured accounts ending with these domains will be excluded from the mailout.
              </p>
            </div>

            {/* Summary statistics bar */}
            <div className="rounded-[12px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)] flex items-center justify-between text-[11px]">
              <div>
                <span className="text-[var(--ink-secondary)]">Unconfigured Total: </span>
                <strong className="text-[var(--ink-primary)] font-mono">{unconfiguredUsers.length}</strong>
              </div>
              {excludedDomains.length > 0 && (
                <div>
                  <span className="text-[var(--ink-secondary)]">
                    Excluded ({excludedDomains.join(", ")}):{" "}
                  </span>
                  <strong className="text-[var(--ink-primary)] font-mono">{excludedCount}</strong>
                </div>
              )}
              <div>
                <span className="text-[var(--ink-secondary)]">Net To Send: </span>
                <strong className="text-[var(--color-blue)] font-mono">{eligibleUserIds.length}</strong>
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
