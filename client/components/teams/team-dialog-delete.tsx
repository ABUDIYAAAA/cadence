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
      <div className="space-y-4 font-mono text-xs">
        <div className="border border-black p-3 bg-neutral-100 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-black shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-bold text-black">
              Safe Member Detachment
            </p>
            <p className="text-neutral-600 leading-relaxed">
              Deleting this team will dissolve the squad and unlink all{" "}
              <strong>{team.member_count} member(s)</strong>. Participant
              accounts and user profiles will remain completely intact and active.
            </p>
          </div>
        </div>

        <div className="border border-neutral-300 p-3 bg-neutral-50 space-y-1">
          <div>
            <span className="text-neutral-500">Target Team: </span>
            <strong className="text-black">{team.name}</strong>
          </div>
          <div>
            <span className="text-neutral-500">Roster Size: </span>
            <span className="font-semibold text-black">
              {team.member_count} Participants
            </span>
          </div>
          {team.leader_name && (
            <div>
              <span className="text-neutral-500">Designated Leader: </span>
              <span className="font-semibold">{team.leader_name}</span>
            </div>
          )}
        </div>
      </div>
    </Dialog>
  );
}
