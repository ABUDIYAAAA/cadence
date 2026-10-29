"use client";

import React from "react";
import { useMyTicket } from "@/hooks/use-tickets";
import { useAuth } from "@/providers/auth-provider";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CheckCircle2, Clock } from "lucide-react";

export function MyTicketView({ compact = false }: { compact?: boolean } = {}) {
  const { user } = useAuth();
  const { data: ticket, isLoading, error } = useMyTicket();

  if (isLoading) {
    return <Skeleton className="h-96 w-full max-w-sm mx-auto rounded-[24px]" />;
  }

  if (error || !ticket || !ticket.qr_code_base64) {
    return (
      <EmptyState
        title="Your pass isn't ready yet"
        description="Organizers will email your QR entry pass before check-in. It will appear here as soon as it's issued."
      />
    );
  }

  const qrImageSrc = ticket.qr_code_base64.startsWith("data:")
    ? ticket.qr_code_base64
    : `data:image/png;base64,${ticket.qr_code_base64}`;

  const checkedInTime = ticket.checked_in_at
    ? new Date(ticket.checked_in_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : null;

  return (
    <div className="w-full max-w-sm mx-auto font-sans">
      <div className="rounded-[24px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] overflow-hidden">
        {/* Status first: the thing staff and the attendee check */}
        <div className="px-5 py-3.5 flex items-center justify-between border-b border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
          <span className="text-sm font-semibold text-[var(--ink-primary)]">Cadence Hackathon</span>
          <Badge variant={ticket.is_checked_in ? "emerald" : "blue"} dot size="sm">
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
              className={`${compact ? "h-44 w-44" : "h-56 w-56"} object-contain rounded-xl`}
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
    </div>
  );
}
