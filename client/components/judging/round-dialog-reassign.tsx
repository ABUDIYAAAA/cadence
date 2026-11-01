"use client";

import React, { useState } from "react";
import { RoundPanelOverview, RoundPanelSlot, RoundSummary } from "@/types/judging";
import {
  useRoundPanels,
  useMoveSlot,
  useUpdateSlotTiming,
  useSetPanelJudges,
  useAssignedJudges,
} from "@/hooks/use-judging";
import { useUsers } from "@/hooks/use-users";
import { Input } from "@/components/ui/input";
import { TeamMarksEditor } from "./team-marks-editor";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { apiErrorMessage } from "@/lib/utils";
import { ArrowDown, ArrowUp, Clock, ListChecks, Users } from "lucide-react";

interface RoundTeamReassignDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
}

const WAITING = ["scheduled", "queued"];

/** ISO timestamp -> value for an <input type="datetime-local"> in the viewer's timezone. */
function toLocalInput(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatTime(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

/** Set one team's own pitch length and start time. Blank fields fall back to the round's defaults. */
function SlotTimingEditor({
  slot,
  roundDefaultMins,
  onDone,
  onError,
}: {
  slot: RoundPanelSlot;
  roundDefaultMins?: number;
  onDone: () => void;
  onError: (message: string) => void;
}) {
  const update = useUpdateSlotTiming();
  const [duration, setDuration] = useState(slot.custom_duration ? String(slot.duration_mins) : "");
  const [start, setStart] = useState(toLocalInput(slot.scheduled_start_time));

  const save = async () => {
    let durationMins: number | null = null;
    if (duration.trim() !== "") {
      durationMins = Number(duration);
      if (!Number.isInteger(durationMins) || durationMins < 1 || durationMins > 600) {
        onError("Duration must be a whole number of minutes between 1 and 600.");
        return;
      }
    }
    let startTime: string | null = null;
    if (start) {
      const d = new Date(start);
      if (Number.isNaN(d.getTime())) {
        onError("That start time isn't valid.");
        return;
      }
      startTime = d.toISOString();
    }
    try {
      await update.mutateAsync({ slotId: slot.slot_id, payload: { duration_mins: durationMins, start_time: startTime } });
      onDone();
    } catch (err: unknown) {
      onError(apiErrorMessage(err, "Failed to update team time"));
    }
  };

  return (
    <div className="mt-1.5 grid grid-cols-1 gap-2 border-t border-[var(--card-line)] pt-2 sm:grid-cols-[8rem_1fr_auto] sm:items-end">
      <Input
        label="Pitch length (min)"
        type="number"
        min={1}
        max={600}
        inputMode="numeric"
        placeholder={roundDefaultMins ? `Round default: ${roundDefaultMins}` : "Round default"}
        value={duration}
        onChange={(e) => setDuration(e.target.value)}
        disabled={update.isPending}
      />
      <Input
        label="Start time"
        type="datetime-local"
        value={start}
        onChange={(e) => setStart(e.target.value)}
        disabled={update.isPending}
      />
      <div className="flex gap-1.5">
        <Button variant="outline" size="sm" type="button" onClick={onDone} disabled={update.isPending}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" type="button" onClick={save} isLoading={update.isPending}>
          Save
        </Button>
      </div>
      <p className="text-[11px] text-[var(--ink-tertiary)] sm:col-span-3">
        Leave a field empty to use the round default. Teams after this one are rescheduled to follow it.
      </p>
    </div>
  );
}

/** Change which judges sit in a room. A judge can only sit in one room per round. */
function PanelJudgesEditor({
  roundId,
  room,
  onDone,
  onError,
}: {
  roundId: string;
  room: RoundPanelOverview;
  onDone: () => void;
  onError: (message: string) => void;
}) {
  const { data: usersData, isLoading: usersLoading } = useUsers({ role: "judge", limit: 100 });
  const { data: assigned, isLoading: assignedLoading } = useAssignedJudges(roundId);
  const setJudges = useSetPanelJudges();
  const [selected, setSelected] = useState<string[]>(room.judge_ids ?? []);

  const takenElsewhere = new Set(
    (assigned ?? []).filter((a) => a.panel_id !== room.panel_id).map((a) => a.judge_id)
  );
  const judges = (usersData?.data ?? []).filter((u) => u.role === "judge");
  const available = judges.filter((u) => !takenElsewhere.has(u.id) || selected.includes(u.id));
  const hidden = judges.length - available.length;

  const toggle = (id: string) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  const save = async () => {
    if (selected.length === 0) {
      onError("A room needs at least one judge.");
      return;
    }
    try {
      await setJudges.mutateAsync({ panelId: room.panel_id, judgeIds: selected });
      onDone();
    } catch (err: unknown) {
      onError(apiErrorMessage(err, "Failed to update judges"));
    }
  };

  return (
    <div className="space-y-2 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-2.5">
      <div className="font-semibold text-[var(--ink-secondary)]">
        Judges in {room.name} ({selected.length} selected)
      </div>
      {usersLoading || assignedLoading ? (
        <div className="text-[var(--ink-tertiary)]">Loading judges...</div>
      ) : available.length === 0 ? (
        <div className="italic text-[var(--ink-tertiary)]">No judges available.</div>
      ) : (
        <div className="grid max-h-40 grid-cols-1 gap-1.5 overflow-y-auto sm:grid-cols-2">
          {available.map((j) => {
            const isSelected = selected.includes(j.id);
            return (
              <label
                key={j.id}
                className={`flex cursor-pointer select-none items-center gap-2 rounded-[10px] border p-2 transition-colors ${
                  isSelected
                    ? "border-[#83A2DB] bg-[var(--color-blue-dim)] font-semibold text-[var(--ink-primary)]"
                    : "border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-secondary)]"
                }`}
              >
                <input
                  type="checkbox"
                  checked={isSelected}
                  onChange={() => toggle(j.id)}
                  disabled={setJudges.isPending}
                  className="h-3.5 w-3.5 rounded accent-[#83A2DB]"
                />
                <span className="min-w-0">
                  <span className="block truncate text-xs font-semibold text-[var(--ink-primary)]">{j.name}</span>
                  <span className="block truncate font-mono text-[10px] text-[var(--ink-tertiary)]">{j.email}</span>
                </span>
              </label>
            );
          })}
        </div>
      )}
      {hidden > 0 && (
        <p className="text-[11px] text-[var(--ink-tertiary)]">
          {hidden} {hidden === 1 ? "judge is" : "judges are"} hidden because they sit in another room this round.
        </p>
      )}
      <p className="text-[11px] text-[var(--ink-tertiary)]">
        A judge who has already scored a team in this room can&apos;t be removed.
      </p>
      <div className="flex justify-end gap-1.5">
        <Button variant="outline" size="sm" type="button" onClick={onDone} disabled={setJudges.isPending}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" type="button" onClick={save} isLoading={setJudges.isPending}>
          Save judges
        </Button>
      </div>
    </div>
  );
}

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
  const [timingSlotId, setTimingSlotId] = useState<string | null>(null);
  const [marksSlotId, setMarksSlotId] = useState<string | null>(null);
  const [editingJudges, setEditingJudges] = useState(false);

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
      title="Manage rooms"
      description={`Round: ${round.name}. Move waiting teams between rooms, set each team's pitch length and start time, and change a room's judges.`}
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
                    onClick={() => {
                      setPickedRoomId(p.panel_id);
                      setTimingSlotId(null);
                      setMarksSlotId(null);
                      setEditingJudges(false);
                    }}
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
              <span className="flex items-center gap-2">
                <span className="tabular-nums">
                  {room.slots.length} {room.slots.length === 1 ? "team" : "teams"}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={() => setEditingJudges((v) => !v)}
                  className="h-7 gap-1.5 px-2"
                  aria-expanded={editingJudges}
                >
                  <Users className="h-3.5 w-3.5" />
                  {room.judge_count} {room.judge_count === 1 ? "judge" : "judges"}
                </Button>
              </span>
            </div>

            {editingJudges && (
              <PanelJudgesEditor
                key={room.panel_id}
                roundId={round.id}
                room={room}
                onDone={() => setEditingJudges(false)}
                onError={setError}
              />
            )}

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
                      className="rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-2.5 py-1.5"
                    >
                      <div className="flex items-center gap-2">
                      <span className="w-7 shrink-0 font-mono font-bold tabular-nums text-[var(--ink-tertiary)]">
                        #{slot.sequence_order}
                      </span>
                      <span className="min-w-0 flex-1 truncate font-semibold text-[var(--ink-primary)]" title={slot.team_name}>
                        {slot.team_name}
                      </span>

                      <Button
                        variant="ghost"
                        size="sm"
                        type="button"
                        aria-label={`Enter marks for ${slot.team_name}`}
                        aria-expanded={marksSlotId === slot.slot_id}
                        title="Enter marks for this team's custom criterion"
                        onClick={() => {
                          setTimingSlotId(null);
                          setMarksSlotId(marksSlotId === slot.slot_id ? null : slot.slot_id);
                        }}
                        className="h-7 shrink-0 gap-1 px-2 active:scale-95"
                      >
                        <ListChecks className="h-3.5 w-3.5" />
                        Marks
                      </Button>

                      {slot.status !== "completed" && (
                        <Button
                          variant="ghost"
                          size="sm"
                          type="button"
                          aria-label={`Edit time for ${slot.team_name}`}
                          aria-expanded={timingSlotId === slot.slot_id}
                          onClick={() => {
                            setMarksSlotId(null);
                            setTimingSlotId(timingSlotId === slot.slot_id ? null : slot.slot_id);
                          }}
                          className="h-7 shrink-0 gap-1 px-2 font-mono tabular-nums active:scale-95"
                        >
                          <Clock className="h-3.5 w-3.5" />
                          {slot.duration_mins}m
                          {slot.scheduled_start_time ? ` · ${formatTime(slot.scheduled_start_time)}` : ""}
                        </Button>
                      )}

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
                      </div>

                      {marksSlotId === slot.slot_id && (
                        <TeamMarksEditor
                          roundId={round.id}
                          teamId={slot.team_id}
                          onDone={() => setMarksSlotId(null)}
                          onError={setError}
                        />
                      )}

                      {timingSlotId === slot.slot_id && (
                        <SlotTimingEditor
                          slot={slot}
                          roundDefaultMins={round.slot_duration_mins}
                          onDone={() => setTimingSlotId(null)}
                          onError={setError}
                        />
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
