"use client";

import React, { useState } from "react";
import { RoundSummary } from "@/types/judging";
import { useRoundPanels, useMoveSlot } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { ArrowDown, ArrowUp } from "lucide-react";

interface RoundTeamReassignDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
}

/**
 * Lets admins move teams between a draft round's rooms, or reorder them within a room.
 * Each change is saved immediately and the lists refresh from the server.
 */
export function RoundTeamReassignDialog({
  round,
  open,
  onClose,
}: RoundTeamReassignDialogProps) {
  const { data: panels, isLoading } = useRoundPanels(open ? round?.id : undefined);
  const moveSlot = useMoveSlot();
  const [error, setError] = useState<string | null>(null);

  if (!round || !open) return null;

  const list = panels ?? [];
  const totalTeams = list.reduce((n, p) => n + p.slots.length, 0);

  const run = async (
    slotId: string,
    payload: { target_panel_id?: string; target_sequence_order?: number }
  ) => {
    setError(null);
    try {
      await moveSlot.mutateAsync({ slotId, payload });
    } catch (err: unknown) {
      setError(
        err && typeof err === "object" && "message" in err
          ? String((err as { message: string }).message)
          : "Failed to move team"
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Reassign Teams"
      description={`Round: ${round.name}. Move teams between rooms or change their order before the round starts.`}
      footer={
        <div className="flex justify-end">
          <Button variant="outline" size="sm" onClick={onClose}>
            Done
          </Button>
        </div>
      }
    >
      <div className="space-y-3 font-sans text-xs">
        {error && (
          <Alert variant="error" title="Move failed">
            {error}
          </Alert>
        )}

        {isLoading ? (
          <div className="p-4 text-center text-[var(--ink-tertiary)]">Loading rooms...</div>
        ) : list.length === 0 ? (
          <div className="p-4 text-center italic text-[var(--ink-tertiary)] rounded-[12px] border border-dashed border-[var(--card-line)]">
            This round has no rooms yet. Add a room first.
          </div>
        ) : totalTeams === 0 ? (
          <div className="p-4 text-center italic text-[var(--ink-tertiary)] rounded-[12px] border border-dashed border-[var(--card-line)]">
            No teams are scheduled yet. Use Slots to add teams to a room.
          </div>
        ) : (
          list.map((panel) => (
            <section
              key={panel.panel_id}
              className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3 space-y-2"
            >
              <div className="flex items-center justify-between gap-2">
                <div className="font-semibold text-[var(--ink-primary)]">
                  {panel.name} <span className="font-normal text-[var(--ink-secondary)]">· {panel.room}</span>
                </div>
                <Badge variant="subtle" size="sm">
                  {panel.slots.length} {panel.slots.length === 1 ? "team" : "teams"}
                </Badge>
              </div>

              {panel.slots.length === 0 ? (
                <div className="text-[var(--ink-tertiary)] italic">No teams in this room.</div>
              ) : (
                <ul className="space-y-1.5">
                  {panel.slots.map((slot, idx) => (
                    <li
                      key={slot.slot_id}
                      className="flex items-center gap-2 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] p-2"
                    >
                      <span className="w-6 shrink-0 font-mono font-bold text-[var(--ink-secondary)]">
                        #{slot.sequence_order}
                      </span>
                      <span className="flex-1 truncate font-semibold text-[var(--ink-primary)]">
                        {slot.team_name}
                      </span>

                      <div className="flex items-center gap-0.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          type="button"
                          aria-label={`Move ${slot.team_name} up`}
                          disabled={idx === 0 || moveSlot.isPending}
                          onClick={() =>
                            run(slot.slot_id, { target_sequence_order: slot.sequence_order - 1 })
                          }
                          className="h-7 w-7 p-0"
                        >
                          <ArrowUp className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          type="button"
                          aria-label={`Move ${slot.team_name} down`}
                          disabled={idx === panel.slots.length - 1 || moveSlot.isPending}
                          onClick={() =>
                            run(slot.slot_id, { target_sequence_order: slot.sequence_order + 1 })
                          }
                          className="h-7 w-7 p-0"
                        >
                          <ArrowDown className="h-3.5 w-3.5" />
                        </Button>
                      </div>

                      {list.length > 1 && (
                        <div className="w-40 shrink-0">
                          <Select
                            aria-label={`Move ${slot.team_name} to another room`}
                            value=""
                            disabled={moveSlot.isPending}
                            onChange={(e) => {
                              if (e.target.value) {
                                run(slot.slot_id, { target_panel_id: e.target.value });
                              }
                            }}
                            className="h-8 text-xs"
                          >
                            <option value="">Move to...</option>
                            {list
                              .filter((p) => p.panel_id !== panel.panel_id)
                              .map((p) => (
                                <option key={p.panel_id} value={p.panel_id}>
                                  {p.name} · {p.room}
                                </option>
                              ))}
                          </Select>
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ))
        )}
      </div>
    </Dialog>
  );
}
