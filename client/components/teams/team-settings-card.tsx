"use client";

import React, { useState } from "react";
import { Lock, LockOpen } from "lucide-react";
import { useSetAllTeamsFrozen, useTeamSettings, useUpdateTeamSettings } from "@/hooks/use-teams";
import { useToast } from "@/providers/toast-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

function errorMessage(err: unknown, fallback: string): string {
  return err && typeof err === "object" && "error" in err
    ? (err as { error: string }).error
    : fallback;
}

interface TeamSettingsCardProps {
  /** Only admins change team size bounds; volunteers can still freeze. */
  canEditSize: boolean;
}

export function TeamSettingsCard({ canEditSize }: TeamSettingsCardProps) {
  const toast = useToast();
  const { data: settings, isLoading } = useTeamSettings();
  const setAllFrozen = useSetAllTeamsFrozen();
  const updateSettings = useUpdateTeamSettings();
  const [minDraft, setMinDraft] = useState<string | null>(null);
  const [maxDraft, setMaxDraft] = useState<string | null>(null);

  if (isLoading || !settings) {
    return <Skeleton className="h-[74px] w-full rounded-[16px]" />;
  }

  const minValue = minDraft ?? String(settings.min_team_size);
  const maxValue = maxDraft ?? String(settings.max_team_size);
  const minNumber = Number(minValue);
  const maxNumber = Number(maxValue);

  const minValid = Number.isInteger(minNumber) && minNumber >= 1 && minNumber <= 100;
  const maxValid = Number.isInteger(maxNumber) && maxNumber >= 1 && maxNumber <= 100;
  const rangeValid = minValid && maxValid && minNumber <= maxNumber;
  const sizeDirty =
    (minDraft !== null && minNumber !== settings.min_team_size) ||
    (maxDraft !== null && maxNumber !== settings.max_team_size);

  const sizeError = !sizeDirty
    ? null
    : !minValid || !maxValid
    ? "Enter whole numbers between 1 and 100."
    : minNumber > maxNumber
    ? "Max must be greater than or equal to min."
    : null;

  const toggleFreezeAll = async () => {
    const next = !settings.frozen_all;
    try {
      await setAllFrozen.mutateAsync(next);
      toast[next ? "warning" : "success"](
        next
          ? "Participants can no longer create, join, leave or change teams."
          : "Participants can change teams again.",
        next ? "All teams frozen" : "Teams unfrozen"
      );
    } catch (err) {
      toast.error(errorMessage(err, "Failed to update freeze state"), "Error");
    }
  };

  const saveSize = async () => {
    try {
      await updateSettings.mutateAsync({
        min_team_size: minNumber,
        max_team_size: maxNumber,
      });
      setMinDraft(null);
      setMaxDraft(null);
      toast.success(
        `Team size bounds updated to ${minNumber}–${maxNumber} members.`,
        "Team size updated"
      );
    } catch (err) {
      toast.error(errorMessage(err, "Failed to update team size"), "Error");
    }
  };

  const numberInputClass =
    "h-9 w-16 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] px-2.5 text-center text-xs font-medium tabular-nums text-[var(--ink-primary)] transition-[border-color,box-shadow] duration-150 focus:border-[var(--color-blue)] focus:outline-none focus:ring-3 focus:ring-[var(--color-blue)]/20 disabled:cursor-not-allowed disabled:opacity-50";

  return (
    <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:px-5 flex flex-col lg:flex-row lg:items-center justify-between gap-4">
      {/* Freeze All Teams */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 flex-1 min-w-0">
        <div className="space-y-0.5 min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
              Team registration
            </h2>
            <Badge variant={settings.frozen_all ? "amber" : "subtle"} size="sm">
              {settings.frozen_all ? "Frozen" : "Open"}
            </Badge>
          </div>
          <p className="text-xs text-[var(--ink-secondary)]">
            {settings.frozen_all
              ? "Participants cannot create, join, leave, or rename teams. Staff can still edit."
              : "Participants can create, join, leave, and rename teams."}
          </p>
        </div>

        <Button
          variant={settings.frozen_all ? "outline" : "primary"}
          size="sm"
          className="h-9 px-3.5 text-xs gap-1.5 shrink-0 self-start sm:self-center"
          onClick={toggleFreezeAll}
          isLoading={setAllFrozen.isPending}
        >
          {settings.frozen_all ? (
            <LockOpen className="h-3.5 w-3.5" />
          ) : (
            <Lock className="h-3.5 w-3.5" />
          )}
          {settings.frozen_all ? "Unfreeze all" : "Freeze all"}
        </Button>
      </div>

      {/* Team Size Bounds */}
      <form
        className="flex flex-col gap-1.5 pt-3 border-t border-[var(--card-line)] lg:pt-0 lg:border-t-0 lg:border-l lg:pl-5 shrink-0"
        onSubmit={(e) => {
          e.preventDefault();
          if (sizeDirty && rangeValid) saveSize();
        }}
      >
        <div className="flex flex-wrap items-center gap-2.5">
          <span className="text-xs font-semibold text-[var(--ink-secondary)] mr-0.5">
            Team size
          </span>

          <div className="flex items-center gap-1.5">
            <label htmlFor="min-team-size" className="sr-only">
              Min members
            </label>
            <input
              id="min-team-size"
              type="number"
              min={1}
              max={100}
              value={minValue}
              onChange={(e) => setMinDraft(e.target.value)}
              disabled={!canEditSize || updateSettings.isPending}
              aria-invalid={Boolean(sizeError) || undefined}
              className={numberInputClass}
            />
            <span className="text-xs text-[var(--ink-tertiary)] select-none">–</span>
            <label htmlFor="max-team-size" className="sr-only">
              Max members
            </label>
            <input
              id="max-team-size"
              type="number"
              min={1}
              max={100}
              value={maxValue}
              onChange={(e) => setMaxDraft(e.target.value)}
              disabled={!canEditSize || updateSettings.isPending}
              aria-invalid={Boolean(sizeError) || undefined}
              className={numberInputClass}
            />
            <span className="text-xs text-[var(--ink-tertiary)]">members</span>
          </div>

          {canEditSize && (
            <Button
              variant={sizeDirty && rangeValid ? "primary" : "outline"}
              size="sm"
              type="submit"
              disabled={!sizeDirty || !rangeValid}
              isLoading={updateSettings.isPending}
              className="h-9 px-3.5 text-xs"
            >
              Save
            </Button>
          )}
        </div>

        {sizeError && (
          <p role="alert" className="text-[11px] font-medium text-[var(--color-coral)]">
            {sizeError}
          </p>
        )}
      </form>
    </div>
  );
}
