"use client";

import React from "react";
import { RoundSummary } from "@/types/judging";
import { useRoundPanels } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export type RoundConfirmMode = "end" | "delete";

interface RoundConfirmDialogProps {
  round: RoundSummary | null;
  mode: RoundConfirmMode;
  open: boolean;
  onClose: () => void;
  /** `force` is true when the round is live or a pitch is running, and the admin confirmed anyway. */
  onConfirm: (force: boolean) => Promise<void>;
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
  // Which teams are presenting right now (only needed to warn before ending or deleting a live round)
  const { data: rooms } = useRoundPanels(open && round?.status === "active" ? round.id : undefined);
  if (!round) return null;

  const livePitches = (rooms ?? []).flatMap((r) => r.slots.filter((s) => s.status === "in_progress"));
  const isLive = round.status === "active";
  const hasLivePitch = livePitches.length > 0;
  const pitchNames = livePitches.map((s) => s.team_name).join(", ");

  const title = mode === "end" ? "End this round?" : "Delete this round?";
  const action =
    mode === "end"
      ? hasLivePitch ? "End anyway" : "End round"
      : isLive ? "Delete anyway" : "Delete round";

  const consequence =
    mode === "end"
      ? "Judges can no longer start or score pitches. Rooms, teams and scores are kept and the leaderboard stays available. This can't be undone."
      : `The round and its rooms, team schedule and every score will be permanently deleted. This can't be undone.`;

  return (
    <Dialog
      open={open}
      onClose={isLoading ? () => {} : onClose}
      title={title}
      description={round.name}
    >
      <div className="space-y-3 pt-2 font-sans">
        {hasLivePitch && (
          <div
            role="alert"
            className="flex items-start gap-3 p-3.5 rounded-[16px] border border-[var(--color-amber-border)] bg-[var(--color-amber-dim)] text-xs"
          >
            <AlertTriangle className="h-4 w-4 text-[var(--color-amber)] shrink-0 mt-0.5" />
            <p className="leading-relaxed text-[var(--ink-primary)]">
              <strong>
                {livePitches.length === 1 ? "A pitch is in progress" : `${livePitches.length} pitches are in progress`}
              </strong>
              {pitchNames && <> ({pitchNames})</>}.{" "}
              {mode === "end"
                ? "Ending now closes it as finished, and any scores judges haven't locked yet won't count."
                : "Deleting now discards it and all scores."}
            </p>
          </div>
        )}

        {mode === "delete" && isLive && !hasLivePitch && (
          <div
            role="alert"
            className="flex items-start gap-3 p-3.5 rounded-[16px] border border-[var(--color-amber-border)] bg-[var(--color-amber-dim)] text-xs"
          >
            <AlertTriangle className="h-4 w-4 text-[var(--color-amber)] shrink-0 mt-0.5" />
            <p className="leading-relaxed text-[var(--ink-primary)]">
              <strong>This round is live.</strong> Judges are currently able to score in it.
            </p>
          </div>
        )}

        <div className="flex items-start gap-3 p-3.5 rounded-[16px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] text-xs">
          <AlertTriangle className="h-4 w-4 text-[var(--color-coral)] shrink-0 mt-0.5" />
          <p className="text-[var(--color-coral)] leading-relaxed">{consequence}</p>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
          <Button variant="outline" size="sm" onClick={onClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onConfirm(mode === "end" ? hasLivePitch : isLive)}
            isLoading={isLoading}
            disabled={isLoading}
          >
            {action}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
