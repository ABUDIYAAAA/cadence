"use client";

import React, { useState } from "react";
import { RoundSummary } from "@/types/judging";
import { useImportTeamCriteria } from "@/hooks/use-judging";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { TeamCriteriaPicker, TeamCriteriaPickerState } from "./team-criteria-picker";

interface Props {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
}

export function RoundTeamImportDialog({ round, open, onClose }: Props) {
  const toast = useToast();
  const importMutation = useImportTeamCriteria();
  const [state, setState] = useState<TeamCriteriaPickerState>({ status: "empty", payload: null });

  if (!round || !open) return null;

  const count = state.status === "valid" ? state.payload.rows.length : 0;

  const handleImport = async () => {
    if (state.status !== "valid") return;
    try {
      await importMutation.mutateAsync({ roundId: round.id, payload: state.payload });
      toast.success(`Imported ${count} teams into "${round.name}".`);
      onClose();
    } catch (err) {
      toast.error(apiErrorMessage(err, "Failed to import teams"));
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Import teams & criterion"
      description={`Round: ${round.name}. Sets the teams in this round and each team's extra criterion. Importing again replaces the previous import.`}
      className="max-w-2xl"
      footer={
        <div className="flex items-center justify-end gap-2 w-full">
          <Button variant="outline" size="sm" onClick={onClose} className="text-xs">
            Cancel
          </Button>
          <Button
            variant="primary"
            size="sm"
            disabled={state.status !== "valid" || importMutation.isPending}
            isLoading={importMutation.isPending}
            onClick={handleImport}
            className="text-xs"
          >
            Import {count || ""} {count === 1 ? "team" : "teams"}
          </Button>
        </div>
      }
    >
      <TeamCriteriaPicker onChange={setState} />
    </Dialog>
  );
}
