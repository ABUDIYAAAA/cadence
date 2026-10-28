"use client";

import React from "react";
import { RoundSummary } from "@/types/judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Award, AlertTriangle, Users, Clock } from "lucide-react";

interface RoundActivateDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  isLoading?: boolean;
}

export function RoundActivateDialog({
  round,
  open,
  onClose,
  onConfirm,
  isLoading = false,
}: RoundActivateDialogProps) {
  if (!round) return null;

  return (
    <Dialog
      open={open}
      onClose={isLoading ? () => {} : onClose}
      title="ACTIVATE JUDGING ROUND"
      description="Transition this evaluation round from DRAFT to ACTIVE."
    >
      <div className="space-y-4 pt-2 font-mono">
        <div className="border border-black p-3 bg-neutral-50 space-y-2">
          <div className="flex items-center gap-2">
            <Award className="h-4 w-4 text-black shrink-0" />
            <div className="font-bold text-sm uppercase truncate">{round.name}</div>
          </div>
          <div className="text-xs text-neutral-500 font-mono">
            ID: {round.id}
          </div>

          <div className="grid grid-cols-2 gap-2 pt-2 border-t border-neutral-200 text-xs">
            <div className="flex items-center gap-1.5 text-neutral-700">
              <Users className="h-3.5 w-3.5" />
              <span>Queue Buffer: <strong>{round.buffer_size} teams</strong></span>
            </div>
            <div className="flex items-center gap-1.5 text-neutral-700">
              <Clock className="h-3.5 w-3.5" />
              <span>Duration: <strong>{round.slot_duration_mins}m</strong></span>
            </div>
          </div>
        </div>

        <div className="flex items-start gap-2.5 p-3 border border-black bg-white text-xs">
          <AlertTriangle className="h-4 w-4 text-black shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-bold uppercase tracking-tight">System Activation Impact</p>
            <p className="text-neutral-600 leading-relaxed">
              Activating this round will immediately open queues, compute live estimated slot start times, notify scheduled judges, and stage the first {round.buffer_size} team(s) into volunteer call-up queue.
            </p>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 pt-2 border-t border-neutral-200">
          <Button
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={isLoading}
            className="border-black text-black"
          >
            CANCEL
          </Button>
          <Button
            size="sm"
            onClick={onConfirm}
            disabled={isLoading}
            className="bg-black text-white hover:bg-neutral-800 border border-black flex items-center gap-1.5"
          >
            {isLoading ? "ACTIVATING..." : "CONFIRM ACTIVATION"}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
