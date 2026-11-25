"use client";

import React, { useState } from "react";
import { Lock, LockOpen } from "lucide-react";
import { useSetAllTeamsFrozen, useTeamSettings, useUpdateTeamSettings } from "@/hooks/use-teams";
import { useToast } from "@/providers/toast-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
    return <Skeleton className="h-24 w-full rounded-[16px]" />;
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

  return (
    <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 grid gap-4 md:grid-cols-2">
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-[var(--ink-primary)]">Freeze all teams</h2>
            <Badge variant={settings.frozen_all ? "amber" : "subtle"} size="sm">
              {settings.frozen_all ? "Frozen" : "Open"}
            </Badge>
          </div>
          <p className="text-xs text-[var(--ink-secondary)]">
            Blocks participants from creating, joining, leaving or renaming. Staff can still edit.
          </p>
        </div>
        <Button
          variant={settings.frozen_all ? "outline" : "primary"}
          size="sm"
          className="gap-1.5 shrink-0"
          onClick={toggleFreezeAll}
          isLoading={setAllFrozen.isPending}
        >
          {settings.frozen_all ? <LockOpen className="h-3.5 w-3.5" /> : <Lock className="h-3.5 w-3.5" />}
          {settings.frozen_all ? "Unfreeze all" : "Freeze all"}
        </Button>
      </div>

      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (sizeDirty && rangeValid) saveSize();
        }}
      >
        <div className="flex-1 grid grid-cols-2 gap-2">
          <Input
            label="Min members"
            type="number"
            min={1}
            max={100}
            value={minValue}
            onChange={(e) => setMinDraft(e.target.value)}
            disabled={!canEditSize || updateSettings.isPending}
            error={sizeDirty && !minValid ? "1–100" : undefined}
          />
          <Input
            label="Max members"
            type="number"
            min={1}
            max={100}
            value={maxValue}
            onChange={(e) => setMaxDraft(e.target.value)}
            disabled={!canEditSize || updateSettings.isPending}
            error={
              sizeDirty && !maxValid
                ? "1–100"
                : sizeDirty && minValid && maxValid && minNumber > maxNumber
                ? "Must be ≥ min"
                : undefined
            }
          />
        </div>
        {canEditSize && (
          <Button
            variant="primary"
            size="sm"
            type="submit"
            disabled={!sizeDirty || !rangeValid}
            isLoading={updateSettings.isPending}
          >
            Save
          </Button>
        )}
      </form>
    </div>
  );
}
