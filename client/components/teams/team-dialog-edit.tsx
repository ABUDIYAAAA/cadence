"use client";

import React, { useState } from "react";
import { TeamResponse, UpdateTeamRequest } from "@/types/teams";
import { useTeam } from "@/hooks/use-teams";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";

interface TeamEditDialogProps {
  team: TeamResponse | null;
  open: boolean;
  onClose: () => void;
  onSubmit: (id: string, payload: UpdateTeamRequest) => Promise<void>;
  isLoading: boolean;
}

function TeamEditForm({
  team,
  onClose,
  onSubmit,
  isLoading,
}: {
  team: TeamResponse;
  onClose: () => void;
  onSubmit: (id: string, payload: UpdateTeamRequest) => Promise<void>;
  isLoading: boolean;
}) {
  // The leader must already be on the team, so offer its current members.
  const { data: detail, isLoading: membersLoading } = useTeam(team.id);
  const [name, setName] = useState(team.name);
  const [leaderId, setLeaderId] = useState(team.leader_id || "");
  const [maxSize, setMaxSize] = useState(team.max_size ? String(team.max_size) : "");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const newErrors: Record<string, string> = {};

    if (!name.trim()) {
      newErrors.name = "Team name cannot be empty";
    } else if (name.trim().length < 2 || name.trim().length > 100) {
      newErrors.name = "Team name must be between 2 and 100 characters";
    }

    const sizeNumber = Number(maxSize);
    if (maxSize.trim() && (!Number.isInteger(sizeNumber) || sizeNumber < 1 || sizeNumber > 100)) {
      newErrors.max_size = "Enter a whole number from 1 to 100, or leave blank to use the default";
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      await onSubmit(team.id, {
        name: name.trim(),
        leader_id: leaderId && leaderId !== team.leader_id ? leaderId : undefined,
        // 0 clears the per-team override on the server.
        max_size: maxSize.trim() ? sizeNumber : team.max_size ? 0 : undefined,
      });
      onClose();
    } catch (err: unknown) {
      if (err && typeof err === "object" && "details" in err) {
        setErrors((err as { details?: Record<string, string> }).details || {});
      }
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-3 font-mono">
      <Input
        label="Team Name *"
        value={name}
        onChange={(e) => {
          setName(e.target.value);
          if (errors.name) setErrors((prev) => ({ ...prev, name: "" }));
        }}
        error={errors.name}
        disabled={isLoading}
        required
      />

      <Select
        label="Team Leader"
        value={leaderId}
        onChange={(e) => setLeaderId(e.target.value)}
        disabled={isLoading || membersLoading}
        helperText={
          team.leader_name
            ? `Current leader: ${team.leader_name}`
            : "No leader currently assigned"
        }
      >
        {!team.leader_id && <option value="">No leader assigned</option>}
        {(detail?.members || []).map((m) => (
          <option key={m.id} value={m.id}>
            {m.name} ({m.email})
          </option>
        ))}
      </Select>

      <Input
        label="Max members (override)"
        type="number"
        min={1}
        max={100}
        value={maxSize}
        onChange={(e) => {
          setMaxSize(e.target.value);
          if (errors.max_size) setErrors((prev) => ({ ...prev, max_size: "" }));
        }}
        error={errors.max_size}
        disabled={isLoading}
        helperText={`Leave blank to use the default (${team.effective_max_size} currently applies).`}
      />

      <div className="flex justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
        <Button
          variant="outline"
          size="sm"
          type="button"
          onClick={onClose}
          disabled={isLoading}
        >
          Cancel
        </Button>
        <Button
          variant="primary"
          size="sm"
          type="submit"
          isLoading={isLoading}
        >
          Save Changes
        </Button>
      </div>
    </form>
  );
}

export function TeamEditDialog({
  team,
  open,
  onClose,
  onSubmit,
  isLoading,
}: TeamEditDialogProps) {
  if (!team || !open) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Edit Team Settings"
      description={`Editing team: ${team.name}`}
    >
      <TeamEditForm
        key={team.id}
        team={team}
        onClose={onClose}
        onSubmit={onSubmit}
        isLoading={isLoading}
      />
    </Dialog>
  );
}
