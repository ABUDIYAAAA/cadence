"use client";

import React, { useState } from "react";
import { RoundSummary } from "@/types/judging";
import { useRoundPanels, useMoveSlot } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { apiErrorMessage } from "@/lib/utils";
import { ArrowDown, ArrowUp } from "lucide-react";

interface RoundTeamReassignDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
}

const WAITING = ["scheduled", "queued"];

/**
 * Move teams between a round's rooms or reorder them within a room. Only teams still waiting in the
 * queue can move. One room is shown at a time, so the list stays short enough to scroll comfortably.
 * Each change saves immediately and the list refreshes from the server.
 */
export function RoundTeamReassignDialog({
  round,
  open,
  onClose,
}: RoundTeamReassignDialogProps) {
  const { data: panels, isLoading } = useRoundPanels(open ? round?.id : undefined);
  const moveSlot = useMoveSlot();
  const [pickedRoomId, setPickedRoomId] = useState("");
  const [error, setError] = useState<string | null>(null);

  if (!round || !open) return null;

  const rooms = panels ?? [];
  const room = rooms.find((p) => p.panel_id === pickedRoomId) ?? rooms[0];
  const totalTeams = rooms.reduce((n, p) => n + p.slots.length, 0);

  const run = async (
    slotId: string,
    payload: { target_panel_id?: string; target_sequence_order?: number }
  ) => {
    setError(null);
    try {
      await moveSlot.mutateAsync({ slotId, payload });
    } catch (err: unknown) {
      setError(apiErrorMessage(err, "Failed to move team"));
    }
  };

  const empty = (text: string) => (
    <div className="p-4 text-center italic text-[var(--ink-tertiary)] rounded-[12px] border border-dashed border-[var(--card-line)]">
      {text}
    </div>
  );

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Move teams"
      description={`Round: ${round.name}. Teams waiting in the queue can move to another room or change order. Presenting and finished teams stay put.`}
      className="max-w-2xl"
      footer={
        <Button variant="outline" size="sm" onClick={onClose}>
          Done
        </Button>
      }
    >
      <div className="space-y-3 font-sans text-xs">
        {error && (
          <Alert variant="error" title="Move failed">
            {error}
          </Alert>
        )}

        {isLoading ? (
          empty("Loading rooms...")
        ) : rooms.length === 0 ? (
          empty("This round has no rooms yet. Add a room first.")
        ) : totalTeams === 0 ? (
          empty("No teams are scheduled yet. Use Add teams to put teams in a room.")
        ) : (
          <>
            {/* Room switcher */}
            <div
              role="tablist"
              aria-label="Rooms"
              className="flex gap-1 overflow-x-auto rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-0.5"
            >
              {rooms.map((p) => {
                const selected = p.panel_id === room.panel_id;
                return (
                  <button
                    key={p.panel_id}
                    type="button"
                    role="tab"
                    aria-selected={selected}
                    onClick={() => setPickedRoomId(p.panel_id)}
                    className={`inline-flex h-8 shrink-0 items-center gap-1.5 rounded-[10px] px-3 text-xs font-semibold transition-[background-color,color,transform] duration-150 ease-out active:scale-[0.97] cursor-pointer ${
                      selected
                        ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs"
                        : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                    }`}
                  >
                    <span>{p.name}</span>
                    <span className="font-mono tabular-nums text-[var(--ink-tertiary)]">{p.slots.length}</span>
                  </button>
                );
              })}
            </div>

            <div className="flex items-center justify-between text-[var(--ink-secondary)]">
              <span>{room.room}</span>
              <span className="tabular-nums">
                {room.slots.length} {room.slots.length === 1 ? "team" : "teams"}
              </span>
            </div>

            {room.slots.length === 0 ? (
              empty("No teams in this room.")
            ) : (
              <ul
                role="tabpanel"
                className="max-h-[min(50dvh,26rem)] space-y-1.5 overflow-y-auto overscroll-contain pr-1"
              >
                {room.slots.map((slot, idx) => {
                  const movable = WAITING.includes(slot.status);
                  const prevMovable = idx > 0 && WAITING.includes(room.slots[idx - 1].status);
                  const nextMovable = idx < room.slots.length - 1 && WAITING.includes(room.slots[idx + 1].status);
                  const otherRooms = rooms.filter((p) => p.panel_id !== room.panel_id);

                  return (
                    <li
                      key={slot.slot_id}
                      className="flex items-center gap-2 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-2.5 py-1.5"
                    >
                      <span className="w-7 shrink-0 font-mono font-bold tabular-nums text-[var(--ink-tertiary)]">
                        #{slot.sequence_order}
                      </span>
                      <span className="min-w-0 flex-1 truncate font-semibold text-[var(--ink-primary)]" title={slot.team_name}>
                        {slot.team_name}
                      </span>

                      {slot.status === "in_progress" && <Badge variant="coral" size="sm">Presenting</Badge>}
                      {slot.status === "completed" && <Badge variant="subtle" size="sm">Done</Badge>}
                      {slot.status === "queued" && <Badge variant="blue" size="sm">On deck</Badge>}

                      {movable && (
                        <>
                          <div className="flex shrink-0 items-center">
                            <Button
                              variant="ghost"
                              size="sm"
                              type="button"
                              aria-label={`Move ${slot.team_name} up`}
                              disabled={!prevMovable || moveSlot.isPending}
                              onClick={() => run(slot.slot_id, { target_sequence_order: slot.sequence_order - 1 })}
                              className="h-7 w-7 p-0 active:scale-90"
                            >
                              <ArrowUp className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              type="button"
                              aria-label={`Move ${slot.team_name} down`}
                              disabled={!nextMovable || moveSlot.isPending}
                              onClick={() => run(slot.slot_id, { target_sequence_order: slot.sequence_order + 1 })}
                              className="h-7 w-7 p-0 active:scale-90"
                            >
                              <ArrowDown className="h-3.5 w-3.5" />
                            </Button>
                          </div>

                          {otherRooms.length > 0 && (
                            <div className="w-32 shrink-0 sm:w-40">
                              <Select
                                aria-label={`Move ${slot.team_name} to another room`}
                                value=""
                                disabled={moveSlot.isPending}
                                onChange={(e) => {
                                  if (e.target.value) run(slot.slot_id, { target_panel_id: e.target.value });
                                }}
                                className="h-8 text-xs"
                              >
                                <option value="">Move to...</option>
                                {otherRooms.map((p) => (
                                  <option key={p.panel_id} value={p.panel_id}>
                                    {p.name}
                                  </option>
                                ))}
                              </Select>
                            </div>
                          )}
                        </>
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </>
        )}
      </div>
    </Dialog>
  );
}
