"use client";

import React from "react";
import { useMyTicket } from "@/hooks/use-tickets";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CheckCircle2, Clock } from "lucide-react";

export function MyTicketView() {
  const { data: ticket, isLoading, error } = useMyTicket();

  if (isLoading) {
    return (
      <div className="space-y-4 font-mono max-w-md mx-auto">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-64 w-full" />
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

  return (
    <div className="max-w-md mx-auto font-mono">
      <Card className="border-2 border-black text-center shadow-xl">
        <CardHeader className="border-b border-black pb-3">
          <div className="flex items-center justify-between">
            <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
              OFFICIAL ENTRY PASS
            </span>
            <Badge
              variant={ticket.is_checked_in ? "solid" : "outline"}
              size="sm"
            >
              {ticket.is_checked_in ? "CHECKED IN" : "VALID PASS"}
            </Badge>
          </div>
          <CardTitle className="text-lg uppercase tracking-wider mt-2">
            Cadence Hackathon Pass
          </CardTitle>
          <CardDescription className="text-xs">
            Present this QR code at the registration gate for admission.
          </CardDescription>
        </CardHeader>

        <CardContent className="pt-6 pb-6 space-y-4 flex flex-col items-center">
          {/* QR Code Image */}
          <div className="p-3 border-2 border-black bg-white inline-block">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={`data:image/png;base64,${ticket.qr_code_base64}`}
              alt="Cadence Entry QR Pass"
              className="h-56 w-56 object-contain"
            />
          </div>

          {/* Ticket Code */}
          <div className="w-full border border-neutral-300 p-2 bg-neutral-50 text-xs font-mono break-all text-neutral-800">
            {ticket.ticket_code}
          </div>

          {/* Check-In Status Detail */}
          {ticket.is_checked_in ? (
            <div className="flex items-center gap-1.5 text-xs text-black font-semibold border border-black p-2 bg-neutral-100 w-full justify-center">
              <CheckCircle2 className="h-4 w-4 text-black" />
              <span>
                Checked In{" "}
                {ticket.checked_in_at
                  ? `on ${new Date(ticket.checked_in_at).toLocaleString()}`
                  : ""}
              </span>
            </div>
          ) : (
            <div className="flex items-center gap-1.5 text-xs text-neutral-600 border border-neutral-200 p-2 bg-white w-full justify-center">
              <Clock className="h-4 w-4 text-neutral-400" />
              <span>Awaiting Gate Check-In</span>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
