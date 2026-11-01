"use client";

import React, { useState } from "react";
import { RoundSummary } from "@/types/judging";
import { useUpdateRoundDuration } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/alert";
import { apiErrorMessage } from "@/lib/utils";

interface RoundDurationDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
}

/** Change the default time per team for a whole round, optionally wiping per-team custom lengths. */
export function RoundDurationDialog({ round, open, onClose }: RoundDurationDialogProps) {
  const update = useUpdateRoundDuration();
  const [mins, setMins] = useState("");
  const [resetCustom, setResetCustom] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [prevRoundId, setPrevRoundId] = useState<string | null>(null);

  const roundId = open && round ? round.id : null;
  if (roundId && roundId !== prevRoundId) {
    setPrevRoundId(roundId);
    setMins(String(round?.slot_duration_mins ?? 10));
    setResetCustom(false);
    setError(null);
  }
  if (!open && prevRoundId) setPrevRoundId(null);

  if (!round || !open) return null;

  const save = async (e?: React.FormEvent) => {
    e?.preventDefault();
    const value = Number(mins);
    if (!Number.isInteger(value) || value < 1 || value > 600) {
      setError("Enter a whole number of minutes between 1 and 600.");
      return;
    }
    try {
      await update.mutateAsync({
        roundId: round.id,
        payload: { slot_duration_mins: value, reset_custom: resetCustom },
      });
      onClose();
    } catch (err: unknown) {
      setError(apiErrorMessage(err, "Failed to update time per team"));
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Time per team"
      description={`Round: ${round.name}. Sets the default pitch length for every team in this round.`}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={onClose} disabled={update.isPending}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" onClick={() => save()} isLoading={update.isPending}>
            Apply to all teams
          </Button>
        </div>
      }
    >
      <form onSubmit={save} className="space-y-3 font-sans text-xs">
        {error && (
          <Alert variant="error" title="Couldn't update">
            {error}
          </Alert>
        )}
        <Input
          label="Minutes per team"
          type="number"
          min={1}
          max={600}
          inputMode="numeric"
          value={mins}
          onChange={(e) => {
            setMins(e.target.value);
            if (error) setError(null);
          }}
          disabled={update.isPending}
        />
        <label className="flex cursor-pointer items-start gap-2.5 select-none rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
          <input
            type="checkbox"
            checked={resetCustom}
            onChange={(e) => setResetCustom(e.target.checked)}
            disabled={update.isPending}
            className="mt-0.5 h-3.5 w-3.5 rounded accent-[#83A2DB]"
          />
          <span>
            <span className="block font-semibold text-[var(--ink-primary)]">
              Overwrite teams with a custom length
            </span>
            <span className="block text-[11px] text-[var(--ink-secondary)]">
              Off: teams you gave their own length keep it. On: every waiting team uses this value.
            </span>
          </span>
        </label>
        <p className="text-[11px] text-[var(--ink-tertiary)]">
          Start times are recalculated for every room. Pinned start times are kept. A team that is presenting
          right now without a custom length gets the new time on its clock.
        </p>
      </form>
    </Dialog>
  );
}
