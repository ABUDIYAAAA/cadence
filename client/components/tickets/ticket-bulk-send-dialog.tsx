"use client";

import React, { useState } from "react";
import { UserDetailResponse } from "@/types/users";
import { BulkSendTicketsRequest, BulkSendTicketsResponse } from "@/types/tickets";
import { useAllUsers } from "@/hooks/use-users";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { QrCode, Filter, CheckCircle2 } from "lucide-react";

/** email: send the QR by mail. release: only show the pass on the participant dashboard. */
export type DeliveryMode = "email" | "release";

interface TicketBulkSendDialogProps {
  open: boolean;
  onClose: () => void;
  selectedUsers: UserDetailResponse[];
  onSubmit: (req: BulkSendTicketsRequest, mode: DeliveryMode) => Promise<BulkSendTicketsResponse>;
  isLoading: boolean;
}

export function TicketBulkSendDialog({
  open,
  onClose,
  selectedUsers: selectedProp,
  onSubmit,
  isLoading,
}: TicketBulkSendDialogProps) {
  // With no manual selection, target every attendee (exclusions still apply).
  const sendToAll = selectedProp.length === 0;
  const { data: allUsers, isLoading: allLoading } = useAllUsers({
    enabled: open && sendToAll,
  });
  const selectedUsers = sendToAll ? allUsers || [] : selectedProp;
  const [excludeDomain, setExcludeDomain] = useState("");
  const [skipCheckedIn, setSkipCheckedIn] = useState(true);
  const [mode, setMode] = useState<DeliveryMode>("email");
  const [result, setResult] = useState<BulkSendTicketsResponse | null>(null);

  const resetDialog = () => {
    setExcludeDomain("");
    setSkipCheckedIn(true);
    setMode("email");
    setResult(null);
  };

  const handleClose = () => {
    resetDialog();
    onClose();
  };

  const normalizedDomain = excludeDomain.trim().toLowerCase().replace(/^@/, "");

  // Exclude users whose email ends with the domain
  const eligibleUsers = normalizedDomain
    ? selectedUsers.filter(
        (u) =>
          !u.email.toLowerCase().endsWith("@" + normalizedDomain) &&
          !u.email.toLowerCase().endsWith("." + normalizedDomain)
      )
    : selectedUsers;

  const excludedCount = selectedUsers.length - eligibleUsers.length;

  const handleSend = async () => {
    try {
      const res = await onSubmit(
        {
          // Bulk mode lets the server resolve recipients; exclusion is enforced there too.
          ...(sendToAll ? {} : { user_ids: eligibleUsers.map((u) => u.id) }),
          ...(normalizedDomain ? { exclude_domains: [normalizedDomain] } : {}),
          skip_checked_in: skipCheckedIn,
        },
        mode
      );
      setResult(res);
    } catch {
      // toast in parent
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title={mode === "email" ? "Bulk Dispatch QR Entry Passes" : "Release QR Passes to Dashboard"}
      description={
        mode === "email"
          ? "Generate cryptographic QR codes and email tickets to attendees."
          : "Generate passes and show them on each attendee's dashboard. No email is sent."
      }
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isLoading}
          >
            {result ? "Close" : "Cancel"}
          </Button>

          {!result && (
            <Button
              variant="primary"
              size="sm"
              onClick={handleSend}
              isLoading={isLoading || (sendToAll && allLoading)}
              disabled={eligibleUsers.length === 0}
              className="gap-1.5"
            >
              <QrCode className="h-3.5 w-3.5" />
              <span>
                {mode === "email" ? "Dispatch" : "Release"} {eligibleUsers.length} QR Ticket
                {eligibleUsers.length === 1 ? "" : "s"}
              </span>
            </Button>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs">
        {result ? (
          <div className="rounded-[16px] border border-[var(--color-emerald-border)] bg-[var(--color-emerald-dim)] p-4 flex items-start gap-3">
            <CheckCircle2 className="h-5 w-5 text-[#2EB88A] shrink-0 mt-0.5" />
            <div className="space-y-1">
              <p className="font-bold text-[#2EB88A] text-sm">
                {mode === "email" ? "Ticket Generation & Email Queue Dispatched" : "Passes Released to Dashboards"}
              </p>
              <p className="text-[var(--color-emerald)]">
                {mode === "email"
                  ? `${result.enqueued} ticket emails enqueued for delivery.`
                  : `${result.enqueued} new passes released (${result.total_processed - result.enqueued - result.skipped} already had one). No emails were sent.`}
              </p>
              {result.skipped > 0 && (
                <p className="text-[var(--ink-tertiary)] text-[11px]">
                  ({result.skipped} tickets skipped as already active or checked in)
                </p>
              )}
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] space-y-1">
              <div className="flex justify-between items-center">
                <span className="text-[var(--ink-secondary)]">
                  {sendToAll ? "All Attendees:" : "Selected Recipients:"}
                </span>
                <strong className="text-[var(--ink-primary)] font-mono">{selectedUsers.length} Users</strong>
              </div>
            </div>

            {/* Delivery */}
            <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid)] space-y-2">
              <div className="text-[var(--ink-primary)] font-semibold uppercase text-[11px]">Delivery</div>
              {(
                [
                  { id: "email", label: "Email the QR pass", hint: "For external attendees without dashboard access." },
                  { id: "release", label: "Release to dashboard only", hint: "Attendees see their QR on their dashboard. No email." },
                ] as const
              ).map((o) => (
                <label key={o.id} className="flex items-start gap-2 cursor-pointer select-none">
                  <input
                    type="radio"
                    name="delivery"
                    checked={mode === o.id}
                    onChange={() => setMode(o.id)}
                    disabled={isLoading}
                    className="accent-[#83A2DB] mt-0.5"
                  />
                  <span>
                    <span className="text-[var(--ink-primary)] font-medium">{o.label}</span>
                    <span className="block text-[10px] text-[var(--ink-secondary)]">{o.hint}</span>
                  </span>
                </label>
              ))}
            </div>

            {/* Excluded Domain Filter */}
            <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid)] space-y-2">
              <div className="flex items-center gap-1.5 text-[var(--ink-primary)] font-semibold uppercase text-[11px]">
                <Filter className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Exclude Specific Email Domain (Optional)</span>
              </div>
              <Input
                placeholder="e.g. test.com or company.internal"
                value={excludeDomain}
                onChange={(e) => setExcludeDomain(e.target.value)}
                disabled={isLoading}
                className="text-xs"
              />
              <p className="text-[10px] text-[var(--ink-secondary)]">
                Attendees on this domain will be skipped (for example, test accounts).
              </p>
            </div>

            {/* Checkbox Options */}
            <label className="flex items-center gap-2 cursor-pointer select-none text-xs">
              <input
                type="checkbox"
                checked={skipCheckedIn}
                onChange={(e) => setSkipCheckedIn(e.target.checked)}
                disabled={isLoading}
                className="accent-[#83A2DB] h-4 w-4 rounded"
              />
              <span className="text-[var(--ink-primary)] font-medium">
                Skip attendees who have already checked in
              </span>
            </label>

            {/* Calculation summary */}
            <div className="rounded-[12px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)] flex items-center justify-between text-[11px]">
              <div>
                <span className="text-[var(--ink-secondary)]">Selected: </span>
                <strong className="text-[var(--ink-primary)] font-mono">{selectedUsers.length}</strong>
              </div>
              {normalizedDomain && (
                <div>
                  <span className="text-[var(--ink-secondary)]">Excluded ({normalizedDomain}): </span>
                  <strong className="text-[var(--ink-primary)] font-mono">{excludedCount}</strong>
                </div>
              )}
              <div>
                <span className="text-[var(--ink-secondary)]">Net To Send: </span>
                <strong className="text-[var(--color-blue)] font-mono">{eligibleUsers.length}</strong>
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
