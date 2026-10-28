"use client";

import React, { useState } from "react";
import { ConfigureSlotsRequest, RoundSummary } from "@/types/judging";
import { useTeams } from "@/hooks/use-teams";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Layers, Trash2, ArrowUpDown } from "lucide-react";

interface RoundSlotsConfigDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
  onSubmit: (panelId: string, payload: ConfigureSlotsRequest) => Promise<void>;
  isLoading: boolean;
}

export function RoundSlotsConfigDialog({
  round,
  open,
  onClose,
  onSubmit,
  isLoading,
}: RoundSlotsConfigDialogProps) {
  const { data: teamsData, isLoading: teamsLoading } = useTeams({ limit: 100 });
  const [panelId, setPanelId] = useState("");
  const [slottedTeamIds, setSlottedTeamIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  if (!round || !open) return null;

  const availableTeams = teamsData?.data || [];

  const handleAddTeamToSlots = (teamId: string) => {
    if (!teamId || slottedTeamIds.includes(teamId)) return;
    setSlottedTeamIds((prev) => [...prev, teamId]);
  };

  const handleRemoveSlot = (index: number) => {
    setSlottedTeamIds((prev) => prev.filter((_, i) => i !== index));
  };

  const handleMoveUp = (index: number) => {
    if (index === 0) return;
    setSlottedTeamIds((prev) => {
      const copy = [...prev];
      const temp = copy[index - 1];
      copy[index - 1] = copy[index];
      copy[index] = temp;
      return copy;
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!panelId.trim()) {
      setError("Panel ID is required.");
      return;
    }
    if (slottedTeamIds.length === 0) {
      setError("Please add at least one team to the evaluation sequence.");
      return;
    }

    try {
      const slots = slottedTeamIds.map((teamId, index) => ({
        team_id: teamId,
        sequence_order: index + 1,
        submission_links: {},
      }));

      await onSubmit(panelId.trim(), { slots });
      onClose();
    } catch (err: unknown) {
      setError(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to configure slots"
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Configure Room Team Slots"
      description={`Scheduling presentation sequence for: ${round.name}`}
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
            variant="primary"
            size="sm"
            onClick={handleSubmit}
            isLoading={isLoading}
            className="gap-1.5"
          >
            <Layers className="h-3.5 w-3.5" />
            <span>Save Slot Sequence ({slottedTeamIds.length})</span>
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-4 font-mono text-xs">
        {error && (
          <div className="border border-black p-2 bg-neutral-100 font-bold">
            ! {error}
          </div>
        )}

        <Input
          label="Target Panel / Room UUID *"
          placeholder="Enter Panel ID"
          value={panelId}
          onChange={(e) => {
            setPanelId(e.target.value);
            if (error) setError(null);
          }}
          disabled={isLoading}
          required
        />

        {/* Add Team Dropdown */}
        <div className="space-y-1.5 pt-1 border-t border-neutral-200">
          <label className="block text-[10px] uppercase font-bold text-neutral-600">
            Enroll Team to Sequence
          </label>
          <Select
            onChange={(e) => {
              if (e.target.value) {
                handleAddTeamToSlots(e.target.value);
                e.target.value = "";
              }
            }}
            disabled={teamsLoading || isLoading}
            className="h-8 text-xs font-mono"
          >
            <option value="">Select a registered team to append...</option>
            {availableTeams
              .filter((t) => !slottedTeamIds.includes(t.id))
              .map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.member_count} members)
                </option>
              ))}
          </Select>
        </div>

        {/* Slotted Teams List */}
        <div className="space-y-1.5 border border-neutral-300 p-2.5 bg-neutral-50">
          <div className="flex justify-between items-center text-[10px] uppercase font-bold text-neutral-600 border-b border-neutral-200 pb-1">
            <span>Slot Presentation Order</span>
            <span>{slottedTeamIds.length} Teams</span>
          </div>

          {slottedTeamIds.length === 0 ? (
            <div className="p-4 text-center text-neutral-400 italic">
              No teams slotted yet. Select a team above to append to the presentation order.
            </div>
          ) : (
            <div className="space-y-1 max-h-48 overflow-y-auto">
              {slottedTeamIds.map((teamId, index) => {
                const team = availableTeams.find((t) => t.id === teamId);
                return (
                  <div
                    key={teamId}
                    className="flex items-center justify-between p-2 bg-white border border-neutral-200"
                  >
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-black text-xs min-w-5">
                        #{index + 1}
                      </span>
                      <div>
                        <strong className="text-black">{team ? team.name : teamId}</strong>
                        {team?.leader_name && (
                          <span className="text-[10px] text-neutral-500 ml-2">
                            (Lead: {team.leader_name})
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center gap-1">
                      {index > 0 && (
                        <Button
                          variant="ghost"
                          size="sm"
                          type="button"
                          onClick={() => handleMoveUp(index)}
                          className="h-6 w-6 p-0 text-neutral-500 hover:text-black"
                          title="Move up"
                        >
                          <ArrowUpDown className="h-3 w-3" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="sm"
                        type="button"
                        onClick={() => handleRemoveSlot(index)}
                        className="h-6 w-6 p-0 text-neutral-400 hover:text-black"
                        title="Remove from slot"
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </form>
    </Dialog>
  );
}
