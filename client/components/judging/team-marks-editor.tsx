"use client";

import React, { useState } from "react";
import { RoundTeam, SetTeamCriterionRequest } from "@/types/judging";
import { useRoundTeams, useSetTeamCriterion, useDeleteTeamCriterion } from "@/hooks/use-judging";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiErrorMessage } from "@/lib/utils";

/** Form values for a team's custom criterion. Strings so the inputs can be empty while typing. */
export interface MarksValue {
  name: string;
  max: string;
  weight: string;
  score: string;
}

export const EMPTY_MARKS: MarksValue = { name: "", max: "", weight: "", score: "" };

/** Name / max / weight copied from the first imported team, since most rounds share one criterion. */
export function marksDefaults(roster: RoundTeam[] | undefined): MarksValue {
  const ref = (roster ?? []).find((t) => t.criterion_name);
  if (!ref) return EMPTY_MARKS;
  return { name: ref.criterion_name, max: String(ref.max_score), weight: String(ref.weight), score: "" };
}

/** Parses the form. Returns an error message or the payload for the API. */
export type ParsedMarks =
  | { error: string; payload?: undefined }
  | { payload: SetTeamCriterionRequest; error?: undefined };

export function parseMarks(v: MarksValue): ParsedMarks {
  const name = v.name.trim();
  const max = Number(v.max);
  const weight = Number(v.weight);
  const score = Number(v.score);
  if (!name) return { error: "Enter the criterion name."};
  if (!Number.isInteger(max) || max < 1 || max > 999) return { error: "Max marks must be a whole number from 1 to 999."};
  if (!(weight > 0)) return { error: "Weight must be greater than 0."};
  if (v.score.trim() === "" || !Number.isFinite(score) || score < 0 || score > max)
    return { error: `Marks must be between 0 and ${max}.`};
  return { payload: { criterion_name: name, max_score: max, weight, score }};
}

export function TeamMarksFields({
  value,
  onChange,
  disabled,
}: {
  value: MarksValue;
  onChange: (next: MarksValue) => void;
  disabled?: boolean;
}) {
  const set = (k: keyof MarksValue) => (e: React.ChangeEvent<HTMLInputElement>) =>
    onChange({ ...value, [k]: e.target.value });
  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-[1fr_5rem_5rem_6rem]">
      <div className="col-span-2 sm:col-span-1">
        <Input label="Criterion" placeholder="e.g. Hardware" value={value.name} onChange={set("name")} disabled={disabled} />
      </div>
      <Input label="Max" type="number" min={1} inputMode="numeric" value={value.max} onChange={set("max")} disabled={disabled} />
      <Input label="Weight" type="number" min={0} step="any" inputMode="decimal" value={value.weight} onChange={set("weight")} disabled={disabled} />
      <Input label="Marks" type="number" min={0} step="any" inputMode="decimal" value={value.score} onChange={set("score")} disabled={disabled} />
    </div>
  );
}

/** Inline editor to enter (or change, or remove) the marks for one team's custom criterion. */
export function TeamMarksEditor({
  roundId,
  teamId,
  onDone,
  onError,
}: {
  roundId: string;
  teamId: string;
  onDone: () => void;
  onError: (message: string) => void;
}) {
  const { data: roster, isLoading } = useRoundTeams(roundId);
  const setCriterion = useSetTeamCriterion();
  const removeCriterion = useDeleteTeamCriterion();
  const [draft, setDraft] = useState<MarksValue | null>(null);

  if (isLoading || !roster) return <div className="mt-1.5 text-[var(--ink-tertiary)]">Loading...</div>;

  const current = roster.find((t) => t.team_id === teamId);
  const hasCriterion = Boolean(current?.criterion_name);
  const initial: MarksValue = hasCriterion
    ? {
        name: current!.criterion_name,
        max: String(current!.max_score),
        weight: String(current!.weight),
        score: current!.awarded_score != null ? String(current!.awarded_score) : "",
      }
    : marksDefaults(roster);
  const value = draft ?? initial;
  const busy = setCriterion.isPending || removeCriterion.isPending;

  const save = async () => {
    const parsed = parseMarks(value);
    if (parsed.error !== undefined) {
      onError(parsed.error);
      return;
    }
    try {
      await setCriterion.mutateAsync({ roundId, teamId, payload: parsed.payload });
      onDone();
    } catch (err: unknown) {
      onError(apiErrorMessage(err, "Failed to save marks"));
    }
  };

  const remove = async () => {
    try {
      await removeCriterion.mutateAsync({ roundId, teamId });
      onDone();
    } catch (err: unknown) {
      onError(apiErrorMessage(err, "Failed to remove criterion"));
    }
  };

  return (
    <div className="mt-1.5 space-y-2 border-t border-[var(--card-line)] pt-2">
      <TeamMarksFields value={value} onChange={setDraft} disabled={busy} />
      <p className="text-[11px] text-[var(--ink-tertiary)]">
        These marks aren&apos;t judged. They count straight into this team&apos;s total on the leaderboard.
      </p>
      <div className="flex justify-end gap-1.5">
        {hasCriterion && (
          <Button variant="ghost" size="sm" type="button" onClick={remove} disabled={busy} isLoading={removeCriterion.isPending} className="mr-auto text-[var(--color-coral)]">
            Remove
          </Button>
        )}
        <Button variant="outline" size="sm" type="button" onClick={onDone} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" type="button" onClick={save} isLoading={setCriterion.isPending} disabled={busy}>
          Save marks
        </Button>
      </div>
    </div>
  );
}
