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
      title="Activate judging round"
      description="Transition this evaluation round from draft to active."
    >
      <div className="space-y-4 pt-2 font-sans">
        <div className="rounded-[16px] border border-[var(--card-line)] p-4 bg-[var(--card-solid-subtle)] space-y-2.5">
          <div className="flex items-center gap-2">
            <Award className="h-4 w-4 text-[var(--color-blue)] shrink-0" />
            <div className="font-bold text-sm text-[var(--ink-primary)] truncate">{round.name}</div>
          </div>
          <div className="text-xs text-[var(--ink-tertiary)] font-mono">
            ID: {round.id}
          </div>

          <div className="grid grid-cols-2 gap-2 pt-2.5 border-t border-[var(--card-line)] text-xs">
            <div className="flex items-center gap-1.5 text-[var(--ink-secondary)]">
              <Users className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Queue Buffer: <strong className="text-[var(--ink-primary)] font-mono">{round.buffer_size} teams</strong></span>
            </div>
            <div className="flex items-center gap-1.5 text-[var(--ink-secondary)]">
              <Clock className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Duration: <strong className="text-[var(--ink-primary)] font-mono">{round.slot_duration_mins}m</strong></span>
            </div>
          </div>
        </div>

        <div className="flex items-start gap-3 p-3.5 rounded-[16px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] text-xs">
          <AlertTriangle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-bold text-[var(--color-coral)] tracking-tight">System Activation Impact</p>
            <p className="text-[var(--color-coral)] leading-relaxed">
              Activating this round lets judges start pitches and score. The first {round.buffer_size} team(s) in each room are marked on-deck so volunteers and participants know who to call up. Teams and rooms can no longer be reassigned from the draft view.
            </p>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
          <Button
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={onConfirm}
            disabled={isLoading}
            className="flex items-center gap-1.5"
          >
            {isLoading ? "Activating..." : "Confirm Activation"}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
