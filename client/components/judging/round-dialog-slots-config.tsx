"use client";

import React, { useState } from "react";
import { ConfigureSlotsRequest, RoundSummary } from "@/types/judging";
import { useTeams } from "@/hooks/use-teams";
import { useRoundPanels, useRoundTeams, useSetTeamCriterion } from "@/hooks/use-judging";
import { TeamResponse } from "@/types/teams";
import { Input } from "@/components/ui/input";
import { apiErrorMessage } from "@/lib/utils";
import {
  MarksValue,
  TeamMarksFields,
  marksDefaults,
  parseMarks,
} from "./team-marks-editor";
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
  const [search, setSearch] = useState("");
  // Every team on Cadence can be added; the API returns at most 100 per request, so search narrows it.
  const { data: teamsData, isLoading: teamsLoading } = useTeams({
    limit: 100,
    search: search.trim() || undefined,
  });
  const { data: roundPanels, isLoading: panelsLoading } = useRoundPanels(
    open ? round?.id : undefined
  );
  const { data: roster } = useRoundTeams(open ? round?.id : undefined);
  const [panelId, setPanelId] = useState("");
  const [slottedTeamIds, setSlottedTeamIds] = useState<string[]>([]);
  // Teams picked so far, kept so their names survive a change of search text.
  const [pickedTeams, setPickedTeams] = useState<Record<string, TeamResponse>>({});
  // Marks entered for teams that weren't in the CSV import (they have no custom criterion yet).
  const [marks, setMarks] = useState<Record<string, MarksValue>>({});
  const setCriterion = useSetTeamCriterion();
  const [error, setError] = useState<string | null>(null);

  if (!round || !open) return null;

  // Teams from the CSV import; any other team is allowed too and gets marks entered by hand.
  const rosterIds = new Set((roster ?? []).map((t) => t.team_id));
  const hasRoster = rosterIds.size > 0;
  const isOffRoster = (teamId: string) => hasRoster && !rosterIds.has(teamId);
  const availableTeams = teamsData?.data || [];
  const matchingTotal = teamsData?.total_count ?? availableTeams.length;
  const panels = roundPanels ?? [];
  // A team plays once per round, so hide teams already scheduled in any of its panels.
  const scheduledIds = new Set(panels.flatMap((p) => p.team_ids));
  const selectedPanel = panels.find((p) => p.panel_id === panelId);

  const handleAddTeamToSlots = (teamId: string) => {
    if (!teamId || slottedTeamIds.includes(teamId)) return;
    const team = availableTeams.find((t) => t.id === teamId);
    if (team) setPickedTeams((prev) => ({ ...prev, [teamId]: team }));
    if (isOffRoster(teamId)) {
      setMarks((prev) => ({ ...prev, [teamId]: marksDefaults(roster) }));
    }
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
      // Teams outside the import: save any marks entered for them first. Leaving all four fields
      // empty skips it (no custom criterion); a half-filled form is an error.
      for (const teamId of slottedTeamIds.filter(isOffRoster)) {
        const v = marks[teamId];
        if (!v || (!v.name.trim() && !v.score.trim())) continue;
        const parsed = parseMarks(v);
        if (parsed.error !== undefined) {
          setError(`${pickedTeams[teamId]?.name ?? "A team"}: ${parsed.error}`);
          return;
        }
        await setCriterion.mutateAsync({ roundId: round.id, teamId, payload: parsed.payload });
      }

      const slots = slottedTeamIds.map((teamId, index) => ({
        team_id: teamId,
        sequence_order: index + 1,
        submission_links: {},
      }));

      await onSubmit(panelId.trim(), { slots, append: true });
      setSlottedTeamIds([]);
      setPickedTeams({});
      setMarks({});
      setSearch("");
      onClose();
    } catch (err: unknown) {
      setError(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : apiErrorMessage(err, "Failed to configure slots")
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Add teams to a room"
      description={`Round: ${round.name}. New teams join the end of the room's order, so you can add teams in stages.`}
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
            isLoading={isLoading || setCriterion.isPending}
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
            Team to add
          </label>
          <Input
            type="search"
            placeholder="Search all teams by name..."
            aria-label="Search teams"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            disabled={isLoading}
          />
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
            <option value="">Choose a team...</option>
            {availableTeams
              .filter((t) => !slottedTeamIds.includes(t.id) && !scheduledIds.has(t.id))
              .map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.member_count} members){isOffRoster(t.id) ? " · not in import" : ""}
                </option>
              ))}
          </Select>
          <p className="text-[11px] text-[var(--ink-tertiary)]">
            {matchingTotal > availableTeams.length
              ? `Showing ${availableTeams.length} of ${matchingTotal} teams. Search to narrow the list.`
              : `${matchingTotal} ${matchingTotal === 1 ? "team" : "teams"} on Cadence${search.trim() ? " match" : ""}.`}
          </p>
        </div>

        {/* Slotted Teams List */}
        <div className="space-y-1.5 rounded-[16px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)]">
          <div className="flex justify-between items-center text-[10px] uppercase font-bold text-[var(--ink-secondary)] border-b border-[var(--card-line)] pb-1.5">
            <span>Pitch order for new teams</span>
            <span>{slottedTeamIds.length} Teams</span>
          </div>

          {slottedTeamIds.length === 0 ? (
            <div className="p-4 text-center text-[var(--ink-tertiary)] italic">
              No teams selected yet. Pick the teams decided so far; others can be added later.
            </div>
          ) : (
            <div className="space-y-1.5 max-h-48 overflow-y-auto">
              {slottedTeamIds.map((teamId, index) => {
                const team = pickedTeams[teamId] ?? availableTeams.find((t) => t.id === teamId);
                const offRoster = isOffRoster(teamId);
                return (
                  <div
                    key={teamId}
                    className="p-2.5 rounded-[10px] bg-[var(--card-solid)] border border-[var(--card-line)]"
                  >
                    <div className="flex items-center justify-between">
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

                    {offRoster && marks[teamId] && (
                      <div className="mt-2 space-y-1.5 border-t border-[var(--card-line)] pt-2">
                        <p className="text-[11px] text-[var(--ink-secondary)]">
                          Not in the import. Enter their custom-criterion marks here, or leave empty to skip.
                        </p>
                        <TeamMarksFields
                          value={marks[teamId]}
                          onChange={(next) => setMarks((prev) => ({ ...prev, [teamId]: next }))}
                          disabled={isLoading || setCriterion.isPending}
                        />
                      </div>
                    )}
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
