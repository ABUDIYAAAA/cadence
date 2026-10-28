"use client";

import React, { useState } from "react";
import { UpdateUserRequest, UserDetailResponse } from "@/types/users";
import { UserRole } from "@/types/auth";
import { useTeams } from "@/hooks/use-teams";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";

interface UserEditDialogProps {
  user: UserDetailResponse | null;
  open: boolean;
  onClose: () => void;
  onSubmit: (id: string, data: UpdateUserRequest) => Promise<void>;
  isLoading: boolean;
}

function UserEditForm({
  user,
  onClose,
  onSubmit,
  isLoading,
}: {
  user: UserDetailResponse;
  onClose: () => void;
  onSubmit: (id: string, data: UpdateUserRequest) => Promise<void>;
  isLoading: boolean;
}) {
  const { data: teamsData, isLoading: teamsLoading } = useTeams({ limit: 100 });
  const [name, setName] = useState(user.name);
  const [phoneNumber, setPhoneNumber] = useState(user.phone_number || "");
  const [role, setRole] = useState<UserRole>(user.role);
  const [teamId, setTeamId] = useState(user.team_id || "");
  const [isActive, setIsActive] = useState(user.is_active);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const newErrors: Record<string, string> = {};

    if (!name.trim()) {
      newErrors.name = "Name cannot be empty";
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      await onSubmit(user.id, {
        name: name.trim(),
        phone_number: phoneNumber.trim() || undefined,
        role,
        team_id: teamId ? teamId : null,
        is_active: isActive,
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
        label="Full Name *"
        value={name}
        onChange={(e) => {
          setName(e.target.value);
          if (errors.name) setErrors((prev) => ({ ...prev, name: "" }));
        }}
        error={errors.name}
        disabled={isLoading}
        required
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Select
          label="System Role"
          value={role}
          onChange={(e) => setRole(e.target.value as UserRole)}
          disabled={isLoading}
        >
          <option value="participant">Participant</option>
          <option value="judge">Judge</option>
          <option value="volunteer">Volunteer</option>
          <option value="admin">Admin</option>
        </Select>

        <Input
          label="Phone Number"
          type="tel"
          value={phoneNumber}
          onChange={(e) => setPhoneNumber(e.target.value)}
          disabled={isLoading}
        />
      </div>

      <Select
        label="Assigned Squad / Team"
        value={teamId}
        onChange={(e) => setTeamId(e.target.value)}
        disabled={isLoading || teamsLoading}
        helperText={user.team_name ? `Current team: ${user.team_name}` : "Participant is unassigned"}
      >
        <option value="">None (Unassigned)</option>
        {(teamsData?.data || []).map((t) => (
          <option key={t.id} value={t.id}>
            {t.name} ({t.member_count} {t.member_count === 1 ? "member" : "members"})
          </option>
        ))}
      </Select>

      <div className="pt-2 border-t border-[var(--card-line)] text-xs">
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={isActive}
            onChange={(e) => setIsActive(e.target.checked)}
            disabled={isLoading}
            className="accent-[#83A2DB] h-4 w-4 rounded"
          />
          <span className="font-semibold text-[var(--ink-primary)]">
            Account Active Status
          </span>
        </label>
        <p className="mt-1 text-[11px] text-[var(--ink-secondary)]">
          Deactivating an account prevents future logins.
        </p>
      </div>

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

export function UserEditDialog({
  user,
  open,
  onClose,
  onSubmit,
  isLoading,
}: UserEditDialogProps) {
  if (!user || !open) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Edit User Record"
      description={`Editing ${user.email}`}
    >
      <UserEditForm
        key={user.id}
        user={user}
        onClose={onClose}
        onSubmit={onSubmit}
        isLoading={isLoading}
      />
    </Dialog>
  );
}
