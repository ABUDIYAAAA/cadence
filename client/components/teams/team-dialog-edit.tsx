"use client";

import React, { useState } from "react";
import { TeamResponse, UpdateTeamRequest } from "@/types/teams";
import { useUsers } from "@/hooks/use-users";
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
  const { data: usersData, isLoading: usersLoading } = useUsers({ limit: 100 });
  const [name, setName] = useState(team.name);
  const [leaderId, setLeaderId] = useState(team.leader_id || "");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const newErrors: Record<string, string> = {};

    if (!name.trim()) {
      newErrors.name = "Team name cannot be empty";
    } else if (name.trim().length < 2 || name.trim().length > 100) {
      newErrors.name = "Team name must be between 2 and 100 characters";
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      await onSubmit(team.id, {
        name: name.trim(),
        leader_id: leaderId ? leaderId : undefined,
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
        label="Designated Squad Leader"
        value={leaderId}
        onChange={(e) => setLeaderId(e.target.value)}
        disabled={isLoading || usersLoading}
        helperText={
          team.leader_name
            ? `Current leader: ${team.leader_name}`
            : "No leader currently assigned"
        }
      >
        <option value="">None (No leader assigned)</option>
        {(usersData?.data || []).map((u) => (
          <option key={u.id} value={u.id}>
            {u.name} ({u.email}) — [{u.role.toUpperCase()}]
            {u.team_id === team.id ? " (Team Member)" : ""}
          </option>
        ))}
      </Select>

      <div className="flex justify-end gap-2 pt-3 border-t border-neutral-200">
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
