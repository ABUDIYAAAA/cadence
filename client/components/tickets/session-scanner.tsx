"use client";

import React, { useState } from "react";
import { useScanSessions } from "@/hooks/use-tickets";
import { Select } from "@/components/ui/select";
import { TicketScanner } from "./ticket-scanner";

const ENTRY = "entry";

/** Scanner with a picker for what is being scanned: entry check-in, or an open attendance/exit session. */
export function SessionScanner() {
  const { data: sessions = [] } = useScanSessions();
  const [picked, setPicked] = useState(ENTRY);

  const open = sessions.filter((s) => s.is_open);
  // A session that was closed (or removed) while selected falls back to entry rather than failing every scan.
  const session = open.find((s) => s.id === picked) ?? null;

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center gap-2">
        <label htmlFor="scan-target" className="text-xs font-semibold text-[var(--ink-secondary)]">
          Scanning for
        </label>
        <div className="w-full sm:w-72">
          <Select
            id="scan-target"
            value={session?.id ?? ENTRY}
            onChange={(e) => setPicked(e.target.value)}
            className="text-xs h-9"
          >
            <option value={ENTRY}>Entry check-in</option>
            {open.map((s) => (
              <option key={s.id} value={s.id}>
                {s.kind === "exit" ? "Exit" : "Attendance"}: {s.name}
              </option>
            ))}
          </Select>
        </div>
      </div>
      {/* Keyed so switching session starts clean: a stale approval never reads as belonging to the new one. */}
      <TicketScanner key={session?.id ?? ENTRY} session={session} />
    </div>
  );
}
