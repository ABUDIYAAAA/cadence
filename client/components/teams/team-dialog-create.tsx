"use client";

import React, { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

interface TeamNameDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (name: string) => Promise<void>;
  isLoading: boolean;
  title: string;
  description: string;
  submitLabel: string;
  initialName?: string;
}

/** Name-only dialog, used by participants to create a team or (as leader) rename it. */
export function TeamNameDialog({
  open,
  onClose,
  onSubmit,
  isLoading,
  title,
  description,
  submitLabel,
  initialName = "",
}: TeamNameDialogProps) {
  const [name, setName] = useState(initialName);
  const [error, setError] = useState<string | undefined>();

  const handleClose = () => {
    setName(initialName);
    setError(undefined);
    onClose();
  };

  const handleSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault();
    const trimmed = name.trim();
    if (trimmed.length < 2 || trimmed.length > 100) {
      setError("Team name must be between 2 and 100 characters");
      return;
    }

    try {
      await onSubmit(trimmed);
      handleClose();
    } catch (err: unknown) {
      if (err && typeof err === "object" && "details" in err) {
        const details = (err as { details?: Record<string, string> }).details;
        if (details?.Name) setError(details.Name);
      }
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title={title}
      description={description}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" type="button" onClick={handleClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" type="button" onClick={() => handleSubmit()} isLoading={isLoading}>
            {submitLabel}
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-3 font-sans">
        <Input
          label="Team name *"
          placeholder="e.g. Quantum Pioneers"
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            if (error) setError(undefined);
          }}
          error={error}
          disabled={isLoading}
          autoFocus
          required
        />
      </form>
    </Dialog>
  );
}
