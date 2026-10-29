"use client";

import React from "react";
import { RoundSummary } from "@/types/judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export type RoundConfirmMode = "end" | "delete";

const COPY: Record<RoundConfirmMode, { title: string; action: string; body: (name: string) => string }> = {
  end: {
    title: "End this round?",
    action: "End round",
    body: (name) =>
      `"${name}" will be closed. Judges can no longer start or score pitches. Rooms, teams and scores are kept and the leaderboard stays available. This can't be undone.`,
  },
  delete: {
    title: "Delete this round?",
    action: "Delete round",
    body: (name) =>
      `"${name}" and its rooms, team schedule and every score will be permanently deleted. This can't be undone.`,
  },
};

interface RoundConfirmDialogProps {
  round: RoundSummary | null;
  mode: RoundConfirmMode;
  open: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  isLoading?: boolean;
}

export function RoundConfirmDialog({
  round,
  mode,
  open,
  onClose,
  onConfirm,
  isLoading = false,
}: RoundConfirmDialogProps) {
  if (!round) return null;
  const copy = COPY[mode];

  return (
    <Dialog
      open={open}
      onClose={isLoading ? () => {} : onClose}
      title={copy.title}
      description={round.name}
    >
      <div className="space-y-4 pt-2 font-sans">
        <div className="flex items-start gap-3 p-3.5 rounded-[16px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] text-xs">
          <AlertTriangle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />
          <p className="text-[var(--color-coral)] leading-relaxed">{copy.body(round.name)}</p>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
          <Button variant="outline" size="sm" onClick={onClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={onConfirm}
            isLoading={isLoading}
            disabled={isLoading}
          >
            {copy.action}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
