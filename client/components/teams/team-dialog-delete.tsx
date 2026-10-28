"use client";

import React from "react";
import { TeamResponse } from "@/types/teams";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

interface TeamDeleteDialogProps {
  team: TeamResponse | null;
  open: boolean;
  onClose: () => void;
  onConfirm: (id: string) => Promise<void>;
  isLoading: boolean;
}

export function TeamDeleteDialog({
  team,
  open,
  onClose,
  onConfirm,
  isLoading,
}: TeamDeleteDialogProps) {
  if (!team) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Disband Team"
      description={`Disbanding squad: ${team.name}`}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onConfirm(team.id)}
            isLoading={isLoading}
          >
            Disband Team
          </Button>
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs">
        <div className="rounded-[16px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] p-3.5 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-[var(--color-coral)] shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-bold text-[var(--color-coral)]">
              Safe Member Detachment
            </p>
            <p className="text-[var(--color-coral)] leading-relaxed">
              Deleting this team will dissolve the squad and unlink all{" "}
              <strong className="text-[var(--ink-primary)]">{team.member_count} member(s)</strong>. Participant
              accounts and user profiles will remain completely intact and active.
            </p>
          </div>
        </div>

        <div className="rounded-[14px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] space-y-1.5">
          <div>
            <span className="text-[var(--ink-secondary)]">Target Team: </span>
            <strong className="text-[var(--ink-primary)]">{team.name}</strong>
          </div>
          <div>
            <span className="text-[var(--ink-secondary)]">Roster Size: </span>
            <span className="font-semibold text-[var(--ink-primary)] font-mono">
              {team.member_count} Participants
            </span>
          </div>
          {team.leader_name && (
            <div>
              <span className="text-[var(--ink-secondary)]">Designated Leader: </span>
              <span className="font-semibold text-[var(--ink-primary)]">{team.leader_name}</span>
            </div>
          )}
        </div>
      </div>
    </Dialog>
  );
}
