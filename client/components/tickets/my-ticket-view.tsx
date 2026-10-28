"use client";

import React from "react";
import { useMyTicket } from "@/hooks/use-tickets";
import { useAuth } from "@/providers/auth-provider";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CheckCircle2, Clock } from "lucide-react";

export function MyTicketView() {
  const { user } = useAuth();
  const { data: ticket, isLoading, error } = useMyTicket();

  if (isLoading) {
    return (
      <div className="space-y-4 font-sans max-w-md mx-auto">
        <Skeleton className="h-8 w-1/3 rounded-xl" />
        <Skeleton className="h-96 w-full rounded-[24px]" />
      </div>
    );
  }

  if (error || !ticket || !ticket.qr_code_base64) {
    return (
      <EmptyState
        title="No Entry Pass Issued Yet"
        description="Your digital QR entry ticket has not been issued yet. Event administrators will generate and email your entry pass prior to hackathon check-in."
      />
    );
  }

  const qrImageSrc = ticket.qr_code_base64.startsWith("data:")
    ? ticket.qr_code_base64
    : `data:image/png;base64,${ticket.qr_code_base64}`;

  return (
    <div className="max-w-md mx-auto font-sans">
      <div className="rounded-[24px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] overflow-hidden">
        {/* Pass Top Bar */}
        <div className="p-5 sm:p-6 bg-[var(--card-solid-subtle)] border-b border-[var(--card-line)] flex items-center justify-between">
          <div>
            <div className="font-bold text-sm text-[var(--ink-primary)] tracking-tight">
              Cadence Hackathon
            </div>
            <div className="text-xs text-[var(--ink-secondary)]">
              Digital Admission Pass
            </div>
          </div>
          <Badge
            variant={ticket.is_checked_in ? "blue" : "outline"}
            className="text-[10px] uppercase font-semibold tracking-wider"
          >
            {ticket.is_checked_in ? "CHECKED IN" : "VALID PASS"}
          </Badge>
        </div>

        {/* Pass QR Stage */}
        <div className="p-6 sm:p-8 flex flex-col items-center bg-[var(--card-solid)]">
          <div className="p-3 bg-white rounded-[20px] border border-[var(--card-line)] shadow-[0_4px_16px_rgba(24,32,48,0.06)]  inline-block">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={qrImageSrc}
              alt="Cadence Entry QR Pass"
              className="h-52 w-52 object-contain rounded-xl"
            />
          </div>

          <div className="text-center mt-5">
            <div className="font-bold text-lg text-[var(--ink-primary)]">
              {user?.name || "Participant"}
            </div>
            <div className="font-mono text-xs font-bold text-[var(--color-blue)] mt-0.5 tracking-wider">
              {ticket.ticket_code}
            </div>
          </div>
        </div>

        {/* Pass Details Strip */}
        <div className="border-t-2 border-dashed border-[var(--card-line)] p-5 sm:p-6 bg-[var(--card-solid-subtle)] flex flex-col gap-3 text-xs">
          <div className="flex items-center justify-between">
            <span className="text-[var(--ink-secondary)] font-medium">Gate Admission</span>
            <span className="font-semibold text-[var(--ink-primary)]">Main Hall Entrance</span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-[var(--ink-secondary)] font-medium">Status</span>
            {ticket.is_checked_in ? (
              <span className="flex items-center gap-1.5 font-semibold text-[var(--ink-primary)]">
                <CheckCircle2 className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>
                  Checked In{" "}
                  {ticket.checked_in_at
                    ? `(${new Date(ticket.checked_in_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })})`
                    : ""}
                </span>
              </span>
            ) : (
              <span className="flex items-center gap-1.5 font-semibold text-[var(--ink-secondary)]">
                <Clock className="h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
                <span>Awaiting Gate Check-In</span>
              </span>
            )}
          </div>

          {ticket.checked_in_at && (
            <div className="flex items-center justify-between">
              <span className="text-[var(--ink-secondary)] font-medium">Timestamp</span>
              <span className="font-mono text-xs font-semibold text-[var(--ink-primary)]">
                {new Date(ticket.checked_in_at).toLocaleString()}
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
