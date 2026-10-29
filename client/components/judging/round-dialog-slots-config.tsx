"use client";

import React, { useState } from "react";
import { ConfigureSlotsRequest, RoundSummary } from "@/types/judging";
import { useTeams } from "@/hooks/use-teams";
import { useRoundPanels } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
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
  const { data: roundPanels, isLoading: panelsLoading } = useRoundPanels(
    open ? round?.id : undefined
  );
  const [panelId, setPanelId] = useState("");
  const [slottedTeamIds, setSlottedTeamIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  if (!round || !open) return null;

  const availableTeams = teamsData?.data || [];
  const panels = roundPanels ?? [];
  // A team plays once per round, so hide teams already scheduled in any of its panels.
  const scheduledIds = new Set(panels.flatMap((p) => p.team_ids));
  const selectedPanel = panels.find((p) => p.panel_id === panelId);

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
      setError("Choose a room to add teams to.");
      return;
    }
    if (slottedTeamIds.length === 0) {
      setError("Add at least one team. You can add the rest later.");
      return;
    }

    try {
      const slots = slottedTeamIds.map((teamId, index) => ({
        team_id: teamId,
        sequence_order: index + 1,
        submission_links: {},
      }));

      await onSubmit(panelId.trim(), { slots, append: true });
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
      title="Add Teams to a Room"
      description={`Round: ${round.name}. Teams are added after the ones already scheduled, so you can distribute them in stages.`}
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
            <span>Add {slottedTeamIds.length} {slottedTeamIds.length === 1 ? "Team" : "Teams"}</span>
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-4 font-sans text-xs">
        {error && (
          <div className="rounded-[14px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] p-3 text-[var(--color-coral)] font-medium">
            Notice: {error}
          </div>
        )}

        <Select
          label="Room *"
          value={panelId}
          onChange={(e) => {
            setPanelId(e.target.value);
            if (error) setError(null);
          }}
          disabled={panelsLoading || isLoading}
          helperText={
            panels.length === 0 && !panelsLoading
              ? "This round has no rooms yet. Add a room first."
              : selectedPanel
              ? `${selectedPanel.slot_count} ${selectedPanel.slot_count === 1 ? "team" : "teams"} already scheduled here.`
              : undefined
          }
        >
          <option value="">{panelsLoading ? "Loading rooms..." : "Select a room..."}</option>
          {panels.map((p) => (
            <option key={p.panel_id} value={p.panel_id}>
              {p.name} · {p.room} ({p.slot_count} scheduled)
            </option>
          ))}
        </Select>

        {/* Add Team Dropdown */}
        <div className="space-y-1.5 pt-1 border-t border-[var(--card-line)]">
          <label className="block text-[10px] uppercase font-bold text-[var(--ink-secondary)]">
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
              .filter((t) => !slottedTeamIds.includes(t.id) && !scheduledIds.has(t.id))
              .map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.member_count} members)
                </option>
              ))}
          </Select>
        </div>

        {/* Slotted Teams List */}
        <div className="space-y-1.5 rounded-[16px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)]">
          <div className="flex justify-between items-center text-[10px] uppercase font-bold text-[var(--ink-secondary)] border-b border-[var(--card-line)] pb-1.5">
            <span>New teams, in pitch order</span>
            <span>{slottedTeamIds.length} Teams</span>
          </div>

          {slottedTeamIds.length === 0 ? (
            <div className="p-4 text-center text-[var(--ink-tertiary)] italic">
              No teams selected yet. Pick the teams decided so far; others can be added later.
            </div>
          ) : (
            <div className="space-y-1.5 max-h-48 overflow-y-auto">
              {slottedTeamIds.map((teamId, index) => {
                const team = availableTeams.find((t) => t.id === teamId);
                return (
                  <div
                    key={teamId}
                    className="flex items-center justify-between p-2.5 rounded-[10px] bg-[var(--card-solid)] border border-[var(--card-line)]"
                  >
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-[var(--ink-primary)] text-xs min-w-5">
                        #{index + 1}
                      </span>
                      <div>
                        <strong className="text-[var(--ink-primary)]">{team ? team.name : teamId}</strong>
                        {team?.leader_name && (
                          <span className="text-[10px] text-[var(--ink-secondary)] ml-2 font-mono">
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
                          className="h-6 w-6 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
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
                        className="h-6 w-6 p-0 text-[var(--color-coral)] hover:bg-[#CE6969]/10"
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
