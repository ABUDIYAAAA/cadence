"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Crown } from "lucide-react";
import { eventsService } from "@/services/events.service";
import { useTransferEventOwnership } from "@/hooks/use-events";
import { useToast } from "@/providers/toast-provider";
import { UserDetailResponse } from "@/types/users";
import { apiErrorMessage } from "@/lib/utils";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/alert";
import { SearchSelect } from "@/components/ui/search-select";

interface TransferOwnershipDialogProps {
  open: boolean;
  onClose: () => void;
  eventSlug: string;
  eventName?: string;
}

export function TransferOwnershipDialog({
  open,
  onClose,
  eventSlug,
  eventName,
}: TransferOwnershipDialogProps) {
  const toast = useToast();
  const transferOwnership = useTransferEventOwnership(eventSlug);

  const [manualEmailMode, setManualEmailMode] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [selectedMember, setSelectedMember] =
    useState<UserDetailResponse | null>(null);
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(searchQuery.trim()), 200);
    return () => clearTimeout(t);
  }, [searchQuery]);

  const { data: membersData, isFetching: membersFetching } = useQuery({
    queryKey: ["events", eventSlug, "transfer-candidates", debouncedSearch],
    queryFn: () =>
      eventsService.listEventUsers(eventSlug, {
        search: debouncedSearch || undefined,
        limit: 20,
      }),
    enabled: open && Boolean(eventSlug) && !manualEmailMode,
  });

  const candidates = useMemo(
    () =>
      (membersData?.data ?? []).filter(
        (u) => u.role !== "owner" && !u.is_banned
      ),
    [membersData?.data]
  );

  const reset = () => {
    setManualEmailMode(false);
    setSearchQuery("");
    setDebouncedSearch("");
    setSelectedMember(null);
    setEmail("");
    setName("");
    setError(null);
  };

  const handleClose = () => {
    if (transferOwnership.isPending) return;
    reset();
    onClose();
  };

  const canSubmit = manualEmailMode
    ? email.trim().includes("@")
    : Boolean(selectedMember);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    try {
      if (manualEmailMode) {
        const trimmedEmail = email.trim();
        if (!trimmedEmail || !trimmedEmail.includes("@")) {
          setError("Enter a valid email address for the new owner.");
          return;
        }
        await transferOwnership.mutateAsync({
          new_owner_email: trimmedEmail,
          new_owner_name: name.trim() || undefined,
        });
        toast.success(
          `Ownership of ${eventName || eventSlug} transferred to ${trimmedEmail}.`,
          "Ownership transferred"
        );
      } else {
        if (!selectedMember) {
          setError("Select a member to transfer ownership to.");
          return;
        }
        await transferOwnership.mutateAsync({
          new_owner_user_id: selectedMember.id,
        });
        toast.success(
          `Ownership of ${eventName || eventSlug} transferred to ${selectedMember.name || selectedMember.email}.`,
          "Ownership transferred"
        );
      }
      reset();
      onClose();
    } catch (err: unknown) {
      setError(apiErrorMessage(err, "Failed to transfer event ownership"));
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Transfer event ownership"
      description={`Assign a new owner for ${eventName || eventSlug}.`}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            type="button"
            onClick={handleClose}
            disabled={transferOwnership.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            size="sm"
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit || transferOwnership.isPending}
            isLoading={transferOwnership.isPending}
            className="gap-1.5"
          >
            <Crown className="h-3.5 w-3.5" />
            <span>Transfer ownership</span>
          </Button>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-4 font-sans text-xs">
        {error && (
          <Alert variant="error" title="Could not transfer ownership">
            {error}
          </Alert>
        )}

        <Alert variant="warning" title="Owner role will change">
          The current owner will be demoted to <strong>Admin</strong>, and the
          selected user will become the sole <strong>Owner</strong> of this
          event. A user can only own one event at a time.
        </Alert>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
              New owner
            </label>
            <button
              type="button"
              onClick={() => {
                setManualEmailMode((prev) => !prev);
                setError(null);
              }}
              className="text-[11px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] underline cursor-pointer"
            >
              {manualEmailMode ? "Pick existing member" : "Enter email manually"}
            </button>
          </div>

          {manualEmailMode ? (
            <div className="space-y-3">
              <Input
                label="Email"
                type="email"
                required
                placeholder="new.owner@event.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={transferOwnership.isPending}
              />
              <Input
                label="Name (optional)"
                placeholder="Used if creating a new account"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={transferOwnership.isPending}
              />
            </div>
          ) : (
            <SearchSelect
              ariaLabel="Search event members"
              placeholder="Search members by name or email..."
              query={searchQuery}
              onQueryChange={setSearchQuery}
              isLoading={membersFetching}
              loadingMessage="Loading members..."
              emptyMessage="No eligible members match search"
              disabled={transferOwnership.isPending}
              maxVisible={3}
              totalMatches={
                (membersData?.total_count ?? 0) > (membersData?.data?.length ?? 0)
                  ? membersData?.total_count
                  : candidates.length
              }
              selectedOption={
                selectedMember
                  ? {
                      value: selectedMember.id,
                      label: selectedMember.name,
                      description: selectedMember.email,
                      meta: selectedMember.role,
                    }
                  : null
              }
              options={candidates.map((u) => ({
                value: u.id,
                label: u.name,
                description: u.email,
                meta: u.role,
              }))}
              onSelect={(opt) => {
                const picked = candidates.find((u) => u.id === opt.value) ?? null;
                setSelectedMember(picked);
                setSearchQuery("");
                setError(null);
              }}
              onClear={() => setSelectedMember(null)}
            />
          )}
        </div>
      </form>
    </Dialog>
  );
}
