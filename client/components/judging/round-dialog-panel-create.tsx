"use client";

import React, { useState } from "react";
import { CreatePanelRequest, RoundSummary } from "@/types/judging";
import { useUsers } from "@/hooks/use-users";
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
  const [panelName, setPanelName] = useState("");
  const [room, setRoom] = useState("");
  const [selectedJudgeIds, setSelectedJudgeIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  if (!round || !open) return null;

  const availableJudges = (usersData?.data || []).filter(
    (u) => u.role === "judge"
  );

  const resetForm = () => {
    setPanelName("");
    setRoom("");
    setSelectedJudgeIds([]);
    setError(null);
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
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!panelName.trim() || !room.trim()) {
      setError("Panel name and room location are required.");
      return;
    }
    if (selectedJudgeIds.length === 0) {
      setError("Please assign at least one evaluator/judge to this room.");
      return;
    }

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
          : "Failed to create panel"
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Add Evaluation Room / Panel"
      description={`Adding to round: ${round.name}`}
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
            <span>Create Room Panel</span>
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-3 font-mono text-xs">
        {error && (
          <div className="border border-black p-2 bg-neutral-100 font-bold">
            ! {error}
          </div>
        )}

        <Input
          label="Panel Name *"
          placeholder="e.g. Panel Beta - FinTech"
          value={panelName}
          onChange={(e) => {
            setPanelName(e.target.value);
            if (error) setError(null);
          }}
          disabled={isLoading}
          required
        />

        <Input
          label="Room Location *"
          placeholder="e.g. Room 302 - Executive Boardroom"
          value={room}
          onChange={(e) => {
            setRoom(e.target.value);
            if (error) setError(null);
          }}
          disabled={isLoading}
          required
        />

        <div className="space-y-1 pt-1 border-t border-neutral-200">
          <label className="block text-[10px] uppercase font-bold text-neutral-600">
            Select Evaluators ({selectedJudgeIds.length} Selected) *
          </label>

          {usersLoading ? (
            <div className="text-neutral-400">Loading evaluators...</div>
          ) : availableJudges.length === 0 ? (
            <div className="text-neutral-500 italic p-2 border border-neutral-200 bg-neutral-50">
              No users with judge role found.
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-1.5 max-h-40 overflow-y-auto border border-neutral-200 p-2 bg-neutral-50">
              {availableJudges.map((j) => {
                const isSelected = selectedJudgeIds.includes(j.id);
                return (
                  <label
                    key={j.id}
                    className={`flex items-center gap-2 p-1.5 border cursor-pointer select-none ${
                      isSelected
                        ? "border-black bg-white font-bold"
                        : "border-neutral-200 bg-white/70 text-neutral-700"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => handleToggleJudge(j.id)}
                      className="accent-black h-3.5 w-3.5 rounded-none"
                    />
                    <div className="truncate">
                      <div>{j.name}</div>
                      <div className="text-[10px] text-neutral-400 font-mono">
                        {j.email}
                      </div>
                    </div>
                  </label>
                );
              })}
            </div>
          )}
        </div>
      </form>
    </Dialog>
  );
}
