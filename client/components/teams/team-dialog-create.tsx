"use client";

import React, { useState } from "react";
import { CreateTeamRequest } from "@/types/teams";
import { useUsers } from "@/hooks/use-users";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";

interface TeamCreateDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (payload: CreateTeamRequest) => Promise<void>;
  isLoading: boolean;
}

export function TeamCreateDialog({
  open,
  onClose,
  onSubmit,
  isLoading,
}: TeamCreateDialogProps) {
  const { data: usersData, isLoading: usersLoading } = useUsers({ limit: 100 });
  const [name, setName] = useState("");
  const [leaderId, setLeaderId] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const resetForm = () => {
    setName("");
    setLeaderId("");
    setErrors({});
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const newErrors: Record<string, string> = {};

    if (!name.trim()) {
      newErrors.name = "Team name is required";
    } else if (name.trim().length < 2 || name.trim().length > 100) {
      newErrors.name = "Team name must be between 2 and 100 characters";
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      await onSubmit({
        name: name.trim(),
        leader_id: leaderId ? leaderId : undefined,
      });
      handleClose();
    } catch (err: unknown) {
      if (err && typeof err === "object" && "details" in err) {
        setErrors((err as { details?: Record<string, string> }).details || {});
      }
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Create New Team"
      description="Provision an event squad for hackathon collaboration and judging."
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            type="button"
            onClick={handleClose}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            size="sm"
            type="button"
            onClick={handleSubmit}
            isLoading={isLoading}
          >
            Create Team
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-3 font-mono">
        <Input
          label="Team Name *"
          placeholder="e.g. Quantum Pioneers"
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
          label="Designated Squad Leader (Optional)"
          value={leaderId}
          onChange={(e) => setLeaderId(e.target.value)}
          disabled={isLoading || usersLoading}
          helperText="Select a registered user as squad leader, or leave unassigned to designate later."
        >
          <option value="">None (Designate later)</option>
          {(usersData?.data || []).map((u) => (
            <option key={u.id} value={u.id}>
              {u.name} ({u.email}) — [{u.role.toUpperCase()}]
              {u.team_name ? ` (In: ${u.team_name})` : " (No Team)"}
            </option>
          ))}
        </Select>
      </form>
    </Dialog>
  );
}
