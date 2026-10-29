"use client";

import React, { useState, useEffect } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/alert";
import { ArrowRightLeft, Layers, Volume2, AlertTriangle } from "lucide-react";
import { VolunteerPanelSchedule, TeamSlotSummary } from "@/types/judging";

interface RoundSlotMoveDialogProps {
  open: boolean;
  onClose: () => void;
  slot: TeamSlotSummary | null;
  currentPanel: VolunteerPanelSchedule | null;
  allPanels: VolunteerPanelSchedule[];
  onMove: (
    slotId: string,
    payload: {
      target_panel_id?: string;
      target_sequence_order?: number;
      status?: string;
    },
  ) => Promise<void>;
  isLoading: boolean;
}

export function RoundSlotMoveDialog({
  open,
  onClose,
  slot,
  currentPanel,
  allPanels,
  onMove,
  isLoading,
}: RoundSlotMoveDialogProps) {
  const [targetPanelId, setTargetPanelId] = useState("");
  const [targetPosition, setTargetPosition] = useState("1");
  const [forceQueued, setForceQueued] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (currentPanel && slot) {
      setTargetPanelId(currentPanel.panel_id);
      setTargetPosition(String(slot.sequence_order || 1));
      setForceQueued(slot.status === "queued");
      setError(null);
    }
  }, [currentPanel, slot, open]);

  if (!open || !slot || !currentPanel) return null;

  const targetPanel =
    allPanels.find((p) => p.panel_id === targetPanelId) || currentPanel;
  const isChangingRoom = targetPanelId !== currentPanel.panel_id;
  const isOverBuffer =
    forceQueued && targetPanel.queued_teams.length >= targetPanel.buffer_size;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const pos = parseInt(targetPosition, 10);
    if (isNaN(pos) || pos < 1) {
      setError("Please enter a valid sequence position (1 or greater).");
      return;
    }

    try {
      await onMove(slot.slot_id, {
        target_panel_id: targetPanelId,
        target_sequence_order: pos,
        status: forceQueued ? "queued" : "scheduled",
      });
      onClose();
    } catch (err: unknown) {
      setError(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to move team",
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Move Team Slot & Manage Queue"
      description={`Reorder or transfer ${slot.team_name} across evaluation rooms`}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            form="slot-move-form"
            variant="primary"
            size="sm"
            isLoading={isLoading}
            className="flex items-center gap-1.5"
          >
            <ArrowRightLeft className="h-3.5 w-3.5" />
            Apply Move
          </Button>
        </div>
      }
    >
      <form
        id="slot-move-form"
        onSubmit={handleSubmit}
        className="space-y-4 text-xs font-sans"
      >
        {error && (
          <Alert variant="error" title="MOVE ERROR">
            {error}
          </Alert>
        )}

        {/* Current Info */}
        <div className="rounded-[14px] bg-card-subtle border border-card-line p-3 space-y-1">
          <div className="flex justify-between items-center">
            <span className="text-ink-secondary">Team:</span>
            <span className="font-bold text-ink-primary text-sm">
              {slot.team_name}
            </span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-ink-secondary">Current Room & Panel:</span>
            <span className="font-mono text-ink-primary">
              {currentPanel.room} ({currentPanel.panel_name})
            </span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-ink-secondary">Current Status:</span>
            <span className="font-mono uppercase font-semibold text-(--color-blue)">
              {slot.status} (Slot #{slot.sequence_order})
            </span>
          </div>
        </div>

        {/* Target Room / Panel */}
        <div className="space-y-1.5">
          <label className="text-xs font-semibold text-ink-primary flex items-center gap-1.5">
            <Layers className="h-3.5 w-3.5 text-ink-tertiary" />
            Destination Room & Panel
          </label>
          <Select
            value={targetPanelId}
            onChange={(e) => setTargetPanelId(e.target.value)}
            disabled={isLoading}
            className="text-xs"
          >
            {allPanels.map((p) => (
              <option key={p.panel_id} value={p.panel_id}>
                {p.room} — {p.panel_name} ({p.total_count} teams, buffer:{" "}
                {p.buffer_size})
              </option>
            ))}
          </Select>
          {isChangingRoom && (
            <p className="text-[11px] text-[var(--color-blue)]">
              Team will be transferred from {currentPanel.room} to{" "}
              {targetPanel.room}.
            </p>
          )}
        </div>

        {/* Target Sequence Position */}
        <div className="space-y-1.5">
          <label className="text-xs font-semibold text-[var(--ink-primary)]">
            Sequence Position in Room
          </label>
          <Input
            type="number"
            min={1}
            value={targetPosition}
            onChange={(e) => setTargetPosition(e.target.value)}
            disabled={isLoading}
            placeholder="e.g. 1"
            className="text-xs font-mono"
          />
          <p className="text-[11px] text-[var(--ink-secondary)]">
            Position 1 places this team at the front of the room's evaluation
            order. Existing slots will adjust automatically.
          </p>
        </div>

        {/* Throw directly into Room Queue */}
        <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5 space-y-2">
          <label className="flex items-start gap-2.5 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={forceQueued}
              onChange={(e) => setForceQueued(e.target.checked)}
              className="mt-0.5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            <div>
              <span className="font-bold text-[var(--ink-primary)] flex items-center gap-1.5">
                <Volume2 className="h-3.5 w-3.5 text-[var(--color-coral)]" />
                Throw directly into room staging queue (On-Deck)
              </span>
              <p className="text-[11px] text-[var(--ink-secondary)] mt-0.5">
                Immediately flags this squad as On-Deck, notifying participants
                and volunteer escorts to bring them to the room right away.
              </p>
            </div>
          </label>

          {isOverBuffer && (
            <div className="rounded-[10px] bg-[var(--color-coral-dim)] border border-[#CE6969]/30 p-2.5 flex items-start gap-2 text-[11px] text-[var(--color-coral)]">
              <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
              <span>
                <strong>Note:</strong> {targetPanel.room} already has{" "}
                {targetPanel.queued_teams.length} team(s) in its queue (standard
                buffer size: {targetPanel.buffer_size}). Throwing this team in
                will exceed the buffer limit, which is fully permitted for admin
                interventions.
              </span>
            </div>
          )}
        </div>
      </form>
    </Dialog>
  );
}
