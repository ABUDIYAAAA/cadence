"use client";

import React, { useState } from "react";
import { CreateUserRequest } from "@/types/users";
import { UserRole } from "@/types/auth";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";

interface UserCreateDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateUserRequest) => Promise<void>;
  isLoading: boolean;
}

export function UserCreateDialog({
  open,
  onClose,
  onSubmit,
  isLoading,
}: UserCreateDialogProps) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<UserRole>("participant");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [sendInvite, setSendInvite] = useState(true);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const resetForm = () => {
    setName("");
    setEmail("");
    setRole("participant");
    setPhoneNumber("");
    setSendInvite(true);
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
      newErrors.name = "Full name is required";
    }
    if (!email.trim()) {
      newErrors.email = "Email address is required";
    } else if (!/^\S+@\S+\.\S+$/.test(email)) {
      newErrors.email = "Enter a valid email address";
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      await onSubmit({
        name: name.trim(),
        email: email.trim(),
        role,
        phone_number: phoneNumber.trim() || undefined,
        send_invite: sendInvite,
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
      title="Create User Account"
      description="Add a single user record to Cadence."
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
            Create User
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-3 font-mono">
        <Input
          label="Full Name *"
          placeholder="e.g. Alex Morgan"
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            if (errors.name) setErrors((prev) => ({ ...prev, name: "" }));
          }}
          error={errors.name}
          disabled={isLoading}
          required
        />

        <Input
          label="Email Address *"
          type="email"
          placeholder="alex@example.com"
          value={email}
          onChange={(e) => {
            setEmail(e.target.value);
            if (errors.email) setErrors((prev) => ({ ...prev, email: "" }));
          }}
          error={errors.email}
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
            placeholder="+1 555-0100"
            value={phoneNumber}
            onChange={(e) => setPhoneNumber(e.target.value)}
            disabled={isLoading}
          />
        </div>

        <div className="pt-2 border-t border-[var(--card-line)] space-y-2 text-xs">
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={sendInvite}
              onChange={(e) => setSendInvite(e.target.checked)}
              disabled={isLoading}
              className="accent-[#83A2DB] h-4 w-4 rounded"
            />
            <span className="text-[var(--ink-secondary)]">
              Immediately dispatch password setup onboarding email
            </span>
          </label>
        </div>
      </form>
    </Dialog>
  );
}
