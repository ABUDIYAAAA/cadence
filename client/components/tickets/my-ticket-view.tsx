"use client";

import React, { useState } from "react";
import { CheckCircle2, Clock, KeyRound } from "lucide-react";
import { useCheckInByCode, useMyTicket } from "@/hooks/use-tickets";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

function SelfCheckInCodeCard() {
  const toast = useToast();
  const checkInByCode = useCheckInByCode();
  const [code, setCode] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = code.trim().toUpperCase();
    if (!trimmed) return;

    try {
      const res = await checkInByCode.mutateAsync(trimmed);
      setCode("");
      toast.success(
        res.message || `Recorded in "${res.session?.name || "session"}".`,
        "Check-in recorded"
      );
    } catch (err: unknown) {
      toast.error(
        apiErrorMessage(err, "Invalid or expired check-in code"),
        "Could not check in"
      );
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5 space-y-2"
    >
      <div className="flex items-center gap-1.5 text-xs font-semibold text-[var(--ink-primary)]">
        <KeyRound className="h-3.5 w-3.5 text-[var(--color-blue)]" />
        <span>Check in with session code</span>
      </div>
      <div className="flex items-center gap-2">
        <Input
          placeholder="Enter code (e.g. K9P4X2)"
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          disabled={checkInByCode.isPending}
          className="h-8 text-xs font-mono uppercase tracking-wider"
        />
        <Button
          type="submit"
          variant="primary"
          size="sm"
          disabled={code.trim().length < 2 || checkInByCode.isPending}
          isLoading={checkInByCode.isPending}
          className="h-8 px-3 text-xs shrink-0"
        >
          Submit
        </Button>
      </div>
    </form>
  );
}

export function MyTicketView({ compact = false }: { compact?: boolean } = {}) {
  const { user } = useAuth();
  const { activeEvent } = useTenant();
  const { data: ticket, isLoading, error } = useMyTicket();

  if (isLoading) {
    return <Skeleton className="h-96 w-full max-w-sm mx-auto rounded-[24px]" />;
  }

  if (error || !ticket || !ticket.qr_code_base64) {
    return (
      <div className="w-full max-w-sm mx-auto font-sans space-y-4">
        <EmptyState
          title="Your pass isn't ready yet"
          description="Organizers will release your QR entry pass before check-in, or you can check in directly with an active session code below."
        />
        <SelfCheckInCodeCard />
      </div>
    );
  }

  const qrImageSrc = ticket.qr_code_base64.startsWith("data:")
    ? ticket.qr_code_base64
    : `data:image/png;base64,${ticket.qr_code_base64}`;

  const checkedInTime = ticket.checked_in_at
    ? new Date(ticket.checked_in_at).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      })
    : null;

  const attendedSessions = ticket.attended_sessions ?? [];

  return (
    <div className="w-full max-w-sm mx-auto font-sans space-y-4">
      <div className="rounded-[24px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] overflow-hidden">
        {/* Status first: the thing staff and the attendee check */}
        <div className="px-5 py-3.5 flex items-center justify-between border-b border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
          <span className="text-sm font-semibold text-[var(--ink-primary)] truncate">
            {activeEvent?.event_name || "Cadence Event"}
          </span>
          <Badge
            variant={ticket.is_checked_in ? "emerald" : "blue"}
            dot
            size="sm"
          >
            {ticket.is_checked_in ? "Checked in" : "Valid"}
          </Badge>
        </div>

        {/* QR is the hero */}
        <div className="px-6 pt-6 pb-5 flex flex-col items-center">
          <div className="p-3 bg-white rounded-[20px] border border-[var(--card-line)] shadow-[0_4px_16px_rgba(24,32,48,0.06)]">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={qrImageSrc}
              alt="Entry QR code"
              className={`${
                compact ? "h-44 w-44" : "h-56 w-56"
              } object-contain rounded-xl`}
            />
          </div>

          <div className="text-center mt-5">
            <div className="font-semibold text-lg tracking-tight text-[var(--ink-primary)]">
              {user?.name || "Participant"}
            </div>
            <div className="font-mono text-xs font-semibold text-[var(--ink-secondary)] mt-1 tracking-widest tabular-nums">
              {ticket.ticket_code}
            </div>
          </div>
        </div>

        <div className="border-t border-dashed border-[var(--card-line)] px-5 py-3.5 bg-[var(--card-solid-subtle)] text-xs">
          {ticket.is_checked_in ? (
            <span className="flex items-center justify-center gap-1.5 font-medium text-[var(--ink-primary)]">
              <CheckCircle2 className="h-3.5 w-3.5 text-[var(--color-emerald)]" />
              Checked in{checkedInTime ? ` at ${checkedInTime}` : ""}
            </span>
          ) : (
            <span className="flex items-center justify-center gap-1.5 text-[var(--ink-secondary)]">
              <Clock className="h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
              Show this code at the entrance
            </span>
          )}
        </div>
      </div>

      {/* Self check-in or attendance code input */}
      <SelfCheckInCodeCard />

      {/* Attended sessions history */}
      {attendedSessions.length > 0 && (
        <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5 space-y-2">
          <div className="text-xs font-semibold text-[var(--ink-primary)]">
            Recorded sessions ({attendedSessions.length})
          </div>
          <ul className="divide-y divide-[var(--card-line)] -my-1">
            {attendedSessions.map((sess) => (
              <li
                key={sess.session_id}
                className="flex items-center justify-between gap-2 py-2 text-xs"
              >
                <div className="min-w-0">
                  <div className="font-medium text-[var(--ink-primary)] truncate">
                    {sess.session_name}
                  </div>
                  <div className="text-[11px] capitalize text-[var(--ink-tertiary)]">
                    {sess.kind === "check_in" ? "Entry check-in" : sess.kind}
                  </div>
                </div>
                <span className="font-mono text-[11px] tabular-nums text-[var(--ink-secondary)] shrink-0">
                  {new Date(sess.scanned_at).toLocaleTimeString([], {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
