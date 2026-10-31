"use client";

import React, { useState } from "react";
import { Copy, Crown, Lock, LogOut, Pencil, Plus, RefreshCw, UserMinus, Users } from "lucide-react";
import {
  useCreateTeam,
  useJoinTeam,
  useKickMember,
  useLeaveTeam,
  useMyTeam,
  useRegenerateMyCode,
  useRenameMyTeam,
} from "@/hooks/use-teams";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { TeamNameDialog } from "./team-dialog-create";
import { TeamJoinDialog } from "./team-dialog-join";

function errorMessage(err: unknown, fallback: string): string {
  return err && typeof err === "object" && "error" in err
    ? (err as { error: string }).error
    : fallback;
}

export function MyTeamView() {
  const { user } = useAuth();
  const toast = useToast();
  const { data: team, isLoading, isError, refetch } = useMyTeam();

  const createTeam = useCreateTeam();
  const joinTeam = useJoinTeam();
  const leaveTeam = useLeaveTeam();
  const renameTeam = useRenameMyTeam();
  const regenerateCode = useRegenerateMyCode();
  const kickMember = useKickMember();

  const [createOpen, setCreateOpen] = useState(false);
  const [joinOpen, setJoinOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [leaveOpen, setLeaveOpen] = useState(false);

  if (isLoading) {
    return <Skeleton className="h-40 w-full rounded-[20px]" />;
  }

  // `null` means "no team yet"; an error is a real failure.
  if (isError && !team) {
    return (
      <EmptyState
        title="Couldn't load your team"
        description="Check your connection and try again."
        action={
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            Retry
          </Button>
        }
      />
    );
  }

  const handleCreate = async (name: string) => {
    try {
      await createTeam.mutateAsync({ name });
      toast.success(`Team "${name}" created. Share the join code with your teammates.`, "Team created");
    } catch (err) {
      toast.error(errorMessage(err, "Failed to create team"), "Could not create team");
      throw err;
    }
  };

  // The join dialog shows failures inline, so no error toast here.
  const handleJoin = async (code: string) => {
    const joined = await joinTeam.mutateAsync({ code });
    toast.success(`You joined "${joined.name}".`, "Joined team");
  };

  if (!team) {
    return (
      <section className="font-sans space-y-3" aria-label="Your team">
        <EmptyState
          title="You're not on a team yet"
          description="Start a team and share its code, or join one with a code from your team leader."
          action={
            <div className="flex flex-wrap justify-center gap-2">
              <Button variant="primary" size="sm" className="gap-1.5" onClick={() => setCreateOpen(true)}>
                <Plus className="h-3.5 w-3.5" />
                Create team
              </Button>
              <Button variant="outline" size="sm" onClick={() => setJoinOpen(true)}>
                Join with code
              </Button>
            </div>
          }
        />
        <TeamNameDialog
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onSubmit={handleCreate}
          isLoading={createTeam.isPending}
          title="Create a team"
          description="You'll be the team leader. Teammates join with the code you share."
          submitLabel="Create team"
        />
        <TeamJoinDialog
          open={joinOpen}
          onClose={() => setJoinOpen(false)}
          onSubmit={handleJoin}
          isLoading={joinTeam.isPending}
        />
      </section>
    );
  }

  const isLeader = team.leader?.id === user?.id;
  const frozen = team.effective_frozen;

  const copyCode = async () => {
    if (!team.join_code) return;
    try {
      await navigator.clipboard.writeText(team.join_code);
      toast.success("Join code copied.", "Copied");
    } catch {
      toast.info(`Join code: ${team.join_code}`, "Copy it manually");
    }
  };

  const handleRename = async (name: string) => {
    try {
      await renameTeam.mutateAsync({ name });
      toast.success("Team renamed.", "Team updated");
    } catch (err) {
      toast.error(errorMessage(err, "Failed to rename team"), "Could not rename team");
      throw err;
    }
  };

  const handleRegenerate = async () => {
    try {
      await regenerateCode.mutateAsync();
      toast.success("The old code no longer works.", "New join code");
    } catch (err) {
      toast.error(errorMessage(err, "Failed to regenerate code"), "Error");
    }
  };

  const handleKick = async (userId: string, name: string) => {
    try {
      await kickMember.mutateAsync(userId);
      toast.info(`${name} was removed from the team.`, "Member removed");
    } catch (err) {
      toast.error(errorMessage(err, "Failed to remove member"), "Error");
    }
  };

  const handleLeave = async () => {
    try {
      await leaveTeam.mutateAsync();
      setLeaveOpen(false);
      toast.info("You left the team.", "Left team");
    } catch (err) {
      toast.error(errorMessage(err, "Failed to leave team"), "Error");
    }
  };

  return (
    <section className="font-sans space-y-3" aria-label="Your team">
      <div className="flex items-baseline justify-between gap-3">
        <div className="flex items-center gap-2 min-w-0">
          <h2 className="text-base font-semibold tracking-tight text-[var(--ink-primary)] truncate">
            {team.name}
          </h2>
          {frozen && (
            <Badge variant="amber" size="sm" className="gap-1 shrink-0">
              <Lock className="h-2.5 w-2.5" />
              Frozen
            </Badge>
          )}
        </div>
        <span className="text-xs text-[var(--ink-secondary)] shrink-0 tabular-nums">
          {team.members.length}/{team.effective_max_size} members
        </span>
      </div>

      {team.join_code && (
        <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] px-4 py-3 flex flex-wrap items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="text-xs text-[var(--ink-secondary)]">Join code</div>
            <div className="font-mono text-lg font-semibold tracking-[0.2em] text-[var(--ink-primary)]">
              {team.join_code}
            </div>
          </div>
          <div className="flex items-center gap-1.5">
            <Button variant="outline" size="sm" className="gap-1.5" onClick={copyCode}>
              <Copy className="h-3.5 w-3.5" />
              Copy
            </Button>
            {isLeader && (
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5"
                onClick={handleRegenerate}
                isLoading={regenerateCode.isPending}
                title="Invalidate the current code and create a new one"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                New code
              </Button>
            )}
          </div>
        </div>
      )}

      {frozen && (
        <p className="text-xs text-[var(--ink-secondary)]">
          Teams are frozen, so members can&apos;t join, leave or be removed. Ask a volunteer if something needs to change.
        </p>
      )}

      <ul className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] divide-y divide-[var(--card-line)] overflow-hidden">
        {team.members.map((member) => (
          <li key={member.id} className="flex items-center justify-between gap-3 px-4 py-3">
            <div className="min-w-0">
              <div className="text-sm font-medium text-[var(--ink-primary)] truncate">
                {member.name}
                {member.id === user?.id && (
                  <span className="text-[var(--ink-tertiary)] font-normal"> (you)</span>
                )}
              </div>
              <div className="text-xs text-[var(--ink-secondary)] truncate">{member.email}</div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              {member.is_leader && (
                <Badge variant="blue" size="sm" className="gap-1">
                  <Crown className="h-2.5 w-2.5" />
                  Lead
                </Badge>
              )}
              {isLeader && !member.is_leader && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 w-7 p-0 text-[var(--color-coral)]"
                  disabled={frozen || kickMember.isPending}
                  onClick={() => handleKick(member.id, member.name)}
                  title={`Remove ${member.name}`}
                  aria-label={`Remove ${member.name} from the team`}
                >
                  <UserMinus className="h-3.5 w-3.5" />
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>

      <div className="flex flex-wrap items-center gap-2">
        {isLeader && (
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            disabled={frozen}
            onClick={() => setRenameOpen(true)}
          >
            <Pencil className="h-3.5 w-3.5" />
            Rename
          </Button>
        )}
        <Button
          variant="outline"
          size="sm"
          className="gap-1.5 text-[var(--color-coral)]"
          disabled={frozen}
          onClick={() => setLeaveOpen(true)}
        >
          <LogOut className="h-3.5 w-3.5" />
          Leave team
        </Button>
        <span className="ml-auto inline-flex items-center gap-1 text-[11px] text-[var(--ink-tertiary)]">
          <Users className="h-3 w-3" />
          Updates automatically
        </span>
      </div>

      <TeamNameDialog
        key={team.name}
        open={renameOpen}
        onClose={() => setRenameOpen(false)}
        onSubmit={handleRename}
        isLoading={renameTeam.isPending}
        title="Rename team"
        description="Only the team leader can rename the team."
        submitLabel="Save name"
        initialName={team.name}
      />

      <Dialog
        open={leaveOpen}
        onClose={() => setLeaveOpen(false)}
        title="Leave team?"
        description={
          team.members.length === 1
            ? "You're the last member, so the team will be deleted."
            : isLeader
              ? "You're the leader. Leadership passes to the longest-standing member."
              : "You can rejoin later with the join code, if there's room."
        }
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setLeaveOpen(false)} disabled={leaveTeam.isPending}>
              Stay
            </Button>
            <Button variant="destructive" size="sm" onClick={handleLeave} isLoading={leaveTeam.isPending}>
              Leave team
            </Button>
          </div>
        }
      >
        <p className="text-sm text-[var(--ink-secondary)]">
          Leaving <strong className="text-[var(--ink-primary)]">{team.name}</strong> removes you from its roster.
        </p>
      </Dialog>
    </section>
  );
}
