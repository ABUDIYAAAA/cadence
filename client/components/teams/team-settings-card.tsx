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
  /** Only admins change the default team size; volunteers can still freeze. */
  canEditSize: boolean;
}

export function TeamSettingsCard({ canEditSize }: TeamSettingsCardProps) {
  const toast = useToast();
  const { data: settings, isLoading } = useTeamSettings();
  const setAllFrozen = useSetAllTeamsFrozen();
  const updateSettings = useUpdateTeamSettings();
  const [sizeDraft, setSizeDraft] = useState<string | null>(null);

  if (isLoading || !settings) {
    return <Skeleton className="h-24 w-full rounded-[16px]" />;
  }

  const sizeValue = sizeDraft ?? String(settings.max_team_size);
  const sizeNumber = Number(sizeValue);
  const sizeValid = Number.isInteger(sizeNumber) && sizeNumber >= 1 && sizeNumber <= 100;
  const sizeDirty = sizeDraft !== null && sizeNumber !== settings.max_team_size;

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
      await updateSettings.mutateAsync({ max_team_size: sizeNumber });
      setSizeDraft(null);
      toast.success(`Teams can now have up to ${sizeNumber} members.`, "Team size updated");
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
          if (sizeDirty && sizeValid) saveSize();
        }}
      >
        <div className="flex-1">
          <Input
            label="Max members per team"
            type="number"
            min={1}
            max={100}
            value={sizeValue}
            onChange={(e) => setSizeDraft(e.target.value)}
            disabled={!canEditSize || updateSettings.isPending}
            error={sizeDirty && !sizeValid ? "Enter a whole number from 1 to 100" : undefined}
            helperText={
              settings.max_team_size_source === "environment"
                ? "Using the server default. Individual teams can override this."
                : "Individual teams can override this."
            }
          />
        </div>
        {canEditSize && (
          <Button
            variant="primary"
            size="sm"
            type="submit"
            disabled={!sizeDirty || !sizeValid}
            isLoading={updateSettings.isPending}
          >
            Save
          </Button>
        )}
      </form>
    </div>
  );
}
