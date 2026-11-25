"use client";

import React, { useState } from "react";
import { CreatePanelRequest, RoundSummary } from "@/types/judging";
import { useUsers } from "@/hooks/use-users";
import { useAssignedJudges } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DoorOpen } from "lucide-react";

interface RoundPanelCreateDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
  onSubmit: (payload: CreatePanelRequest) => Promise<void>;
  isLoading: boolean;
}

export function RoundPanelCreateDialog({
  round,
  open,
  onClose,
  onSubmit,
  isLoading,
}: RoundPanelCreateDialogProps) {
  const { data: usersData, isLoading: usersLoading } = useUsers({
    role: "judge",
    limit: 100,
  });
  const { data: assignedJudges, isLoading: assignedLoading } = useAssignedJudges(
    open ? round?.id : undefined
  );
  const [panelName, setPanelName] = useState("");
  const [room, setRoom] = useState("");
  const [selectedJudgeIds, setSelectedJudgeIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    name?: string;
    room?: string;
    judges?: string;
  }>({});

  if (!round || !open) return null;

  // A judge can only sit on one panel per round, so hide those already seated elsewhere.
  const assignedIds = new Set((assignedJudges ?? []).map((a) => a.judge_id));
  const allJudges = (usersData?.data || []).filter((u) => u.role === "judge");
  const availableJudges = allJudges.filter((u) => !assignedIds.has(u.id));
  const hiddenCount = allJudges.length - availableJudges.length;

  const resetForm = () => {
    setPanelName("");
    setRoom("");
    setSelectedJudgeIds([]);
    setError(null);
    setFieldErrors({});
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleToggleJudge = (judgeId: string) => {
    setSelectedJudgeIds((prev) =>
      prev.includes(judgeId)
        ? prev.filter((id) => id !== judgeId)
        : [...prev, judgeId]
    );
    if (fieldErrors.judges) {
      setFieldErrors((prev) => ({ ...prev, judges: undefined }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    const nextErrors: { name?: string; room?: string; judges?: string } = {};
    if (!panelName.trim()) nextErrors.name = "Required";
    if (!room.trim()) nextErrors.room = "Required";
    if (selectedJudgeIds.length === 0) nextErrors.judges = "Select at least one judge";

    if (Object.keys(nextErrors).length > 0) {
      setFieldErrors(nextErrors);
      return;
    }
    setFieldErrors({});

    try {
      await onSubmit({
        round_id: round.id,
        name: panelName.trim(),
        room: room.trim(),
        judge_ids: selectedJudgeIds,
      });
      handleClose();
    } catch (err: unknown) {
      setError(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to add room"
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Add a room"
      description={`Round: ${round.name}. A room is where teams pitch; choose the judges who score there.`}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClose}
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
            <DoorOpen className="h-3.5 w-3.5" />
            <span>Add room</span>
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-3 font-sans text-xs">
        {error && (
          <div className="rounded-[14px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] p-3 text-[var(--color-coral)] font-medium">
            Notice: {error}
          </div>
        )}

        <Input
          label="Room name *"
          placeholder="e.g. FinTech track"
          value={panelName}
          onChange={(e) => {
            setPanelName(e.target.value);
            if (fieldErrors.name) setFieldErrors((prev) => ({ ...prev, name: undefined }));
            if (error) setError(null);
          }}
          error={fieldErrors.name}
          disabled={isLoading}
          required
        />

        <Input
          label="Location *"
          placeholder="e.g. Room 302"
          value={room}
          onChange={(e) => {
            setRoom(e.target.value);
            if (fieldErrors.room) setFieldErrors((prev) => ({ ...prev, room: undefined }));
            if (error) setError(null);
          }}
          error={fieldErrors.room}
          disabled={isLoading}
          required
        />

        <div className="space-y-1.5 pt-1 border-t border-[var(--card-line)]">
          <div className="flex items-baseline justify-between gap-2">
            <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
              Judges for this room ({selectedJudgeIds.length} selected) *
            </label>
            {fieldErrors.judges && (
              <span role="alert" className="text-[11px] font-medium text-[var(--color-coral)]">
                {fieldErrors.judges}
              </span>
            )}
          </div>

          {usersLoading || assignedLoading ? (
            <div className="text-[var(--ink-tertiary)]">Loading judges...</div>
          ) : availableJudges.length === 0 ? (
            <div className="text-[var(--ink-tertiary)] italic p-3 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
              {allJudges.length === 0
                ? "No users with judge role found."
                : "All judges are already assigned to panels in this round."}
            </div>
          ) : (
            <div
              className={`grid grid-cols-1 sm:grid-cols-2 gap-1.5 max-h-40 overflow-y-auto rounded-[14px] border p-2 bg-[var(--card-solid-subtle)] transition-[border-color,box-shadow] ${
                fieldErrors.judges
                  ? "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20"
                  : "border-[var(--card-line)]"
              }`}
            >
              {availableJudges.map((j) => {
                const isSelected = selectedJudgeIds.includes(j.id);
                return (
                  <label
                    key={j.id}
                    className={`flex items-center gap-2 p-2 rounded-[10px] border cursor-pointer select-none transition-all ${
                      isSelected
                        ? "border-[#83A2DB] bg-[var(--color-blue-dim)] text-[var(--ink-primary)] font-semibold"
                        : "border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-secondary)]"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => handleToggleJudge(j.id)}
                      className="accent-[#83A2DB] h-3.5 w-3.5 rounded"
                    />
                    <div className="truncate">
                      <div className="text-xs font-semibold text-[var(--ink-primary)]">{j.name}</div>
                      <div className="text-[10px] text-[var(--ink-tertiary)] font-mono">
                        {j.email}
                      </div>
                    </div>
                  </label>
                );
              })}
            </div>
          )}
          {hiddenCount > 0 && (
            <p className="text-[11px] text-[var(--ink-tertiary)]">
              {hiddenCount} {hiddenCount === 1 ? "judge is" : "judges are"} hidden because{" "}
              {hiddenCount === 1 ? "they are" : "they're"} already assigned to another panel in this round.
            </p>
          )}
        </div>
      </form>
    </Dialog>
  );
}
