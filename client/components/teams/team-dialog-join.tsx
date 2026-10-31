"use client";

import React, { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

interface TeamJoinDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (code: string) => Promise<void>;
  isLoading: boolean;
}

export function TeamJoinDialog({ open, onClose, onSubmit, isLoading }: TeamJoinDialogProps) {
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | undefined>();

  const handleClose = () => {
    setCode("");
    setError(undefined);
    onClose();
  };

  const handleSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault();
    const trimmed = code.trim().toUpperCase();
    if (trimmed.length < 4) {
      setError("Enter the join code your team leader shared");
      return;
    }

    try {
      await onSubmit(trimmed);
      handleClose();
    } catch (err: unknown) {
      // Show the server's reason (invalid code, team full, frozen, ...) inline.
      setError(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Could not join that team"
      );
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Join a team"
      description="Ask your team leader for the join code."
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" type="button" onClick={handleClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" type="button" onClick={() => handleSubmit()} isLoading={isLoading}>
            Join team
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-3 font-sans">
        <Input
          label="Join code *"
          placeholder="e.g. K7M2QX"
          value={code}
          onChange={(e) => {
            setCode(e.target.value.toUpperCase());
            if (error) setError(undefined);
          }}
          error={error}
          disabled={isLoading}
          maxLength={16}
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          className="font-mono tracking-widest uppercase"
          autoFocus
          required
        />
      </form>
    </Dialog>
  );
}
