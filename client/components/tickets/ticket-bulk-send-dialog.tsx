"use client";

import React, { useState } from "react";
import { UserDetailResponse } from "@/types/users";
import { BulkSendTicketsRequest, BulkSendTicketsResponse } from "@/types/tickets";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { QrCode, Filter, CheckCircle2 } from "lucide-react";

interface TicketBulkSendDialogProps {
  open: boolean;
  onClose: () => void;
  selectedUsers: UserDetailResponse[];
  onSubmit: (req: BulkSendTicketsRequest) => Promise<BulkSendTicketsResponse>;
  isLoading: boolean;
}

export function TicketBulkSendDialog({
  open,
  onClose,
  selectedUsers,
  onSubmit,
  isLoading,
}: TicketBulkSendDialogProps) {
  const [excludeDomain, setExcludeDomain] = useState("");
  const [skipCheckedIn, setSkipCheckedIn] = useState(true);
  const [result, setResult] = useState<BulkSendTicketsResponse | null>(null);

  const resetDialog = () => {
    setExcludeDomain("");
    setSkipCheckedIn(true);
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
      const res = await onSubmit({
        user_ids: eligibleUsers.map((u) => u.id),
        skip_checked_in: skipCheckedIn,
      });
      setResult(res);
    } catch {
      // toast in parent
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Bulk Dispatch QR Entry Passes"
      description="Generate cryptographic QR codes and email tickets to attendees."
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
              isLoading={isLoading}
              disabled={eligibleUsers.length === 0}
              className="gap-1.5"
            >
              <QrCode className="h-3.5 w-3.5" />
              <span>
                Dispatch {eligibleUsers.length} QR Ticket{eligibleUsers.length === 1 ? "" : "s"}
              </span>
            </Button>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs">
        {result ? (
          <div className="border border-black p-4 bg-neutral-100 flex items-start gap-3">
            <CheckCircle2 className="h-5 w-5 text-black shrink-0 mt-0.5" />
            <div className="space-y-1">
              <p className="font-bold text-black text-sm">
                Ticket Generation & Email Queue Dispatched
              </p>
              <p className="text-neutral-600">
                {result.enqueued} ticket emails enqueued for delivery.
              </p>
              {result.skipped > 0 && (
                <p className="text-neutral-500 text-[11px]">
                  ({result.skipped} tickets skipped as already active or checked in)
                </p>
              )}
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="border border-neutral-300 p-3 bg-neutral-50 space-y-1">
              <div className="flex justify-between items-center">
                <span className="text-neutral-600">Selected Recipients:</span>
                <strong className="text-black">{selectedUsers.length} Users</strong>
              </div>
            </div>

            {/* Excluded Domain Filter */}
            <div className="border border-neutral-300 p-3 bg-white space-y-2">
              <div className="flex items-center gap-1.5 text-black font-bold uppercase text-[11px]">
                <Filter className="h-3.5 w-3.5" />
                <span>Exclude Specific Email Domain (Optional)</span>
              </div>
              <Input
                placeholder="e.g. test.com or company.internal"
                value={excludeDomain}
                onChange={(e) => setExcludeDomain(e.target.value)}
                disabled={isLoading}
                className="text-xs"
              />
              <p className="text-[10px] text-neutral-500">
                Emails ending with this domain will be omitted so test accounts do not receive messages.
              </p>
            </div>

            {/* Checkbox Options */}
            <label className="flex items-center gap-2 cursor-pointer select-none text-xs">
              <input
                type="checkbox"
                checked={skipCheckedIn}
                onChange={(e) => setSkipCheckedIn(e.target.checked)}
                disabled={isLoading}
                className="accent-black h-4 w-4 rounded-none"
              />
              <span className="text-neutral-800">
                Skip attendees who have already checked in
              </span>
            </label>

            {/* Calculation summary */}
            <div className="border border-neutral-200 p-2.5 bg-neutral-50 flex items-center justify-between text-[11px]">
              <div>
                <span className="text-neutral-500">Selected: </span>
                <strong className="text-black">{selectedUsers.length}</strong>
              </div>
              {normalizedDomain && (
                <div>
                  <span className="text-neutral-500">Excluded ({normalizedDomain}): </span>
                  <strong className="text-neutral-800">{excludedCount}</strong>
                </div>
              )}
              <div>
                <span className="text-neutral-500">Net To Send: </span>
                <strong className="text-black">{eligibleUsers.length}</strong>
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
