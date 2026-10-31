"use client";

import React, { useState } from "react";
import {
  Users,
  Crown,
  UserPlus,
  Trash2,
  CheckCircle2,
  Clock,
  Layers,
} from "lucide-react";
import { useTeam } from "@/hooks/use-teams";
import { UserDetailResponse } from "@/types/users";
import { ParticipantPicker } from "./participant-picker";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface TeamDetailDialogProps {
  teamId: string | null;
  open: boolean;
  onClose: () => void;
  onAddMember: (teamId: string, userId: string) => Promise<void>;
  onRemoveMember: (teamId: string, userId: string) => Promise<void>;
  onSetLeader: (teamId: string, leaderId: string) => Promise<void>;
  canManage?: boolean;
}

export function TeamDetailDialog({
  teamId,
  open,
  onClose,
  onAddMember,
  onRemoveMember,
  onSetLeader,
  canManage = true,
}: TeamDetailDialogProps) {
  const { data: team, isLoading } = useTeam(teamId || "");
  const [selectedUser, setSelectedUser] = useState<UserDetailResponse | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [actionInProgressUserId, setActionInProgressUserId] = useState<string | null>(null);
  const [addError, setAddError] = useState<string | null>(null);

  if (!open || !teamId) return null;

  const handleAddSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUser) {
      setAddError("Search for a participant and pick one from the list");
      return;
    }

    setIsAdding(true);
    setAddError(null);
    try {
      await onAddMember(teamId, selectedUser.id);
      setSelectedUser(null);
    } catch (err: unknown) {
      setAddError(
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to add member"
      );
    } finally {
      setIsAdding(false);
    }
  };

  const handleRemove = async (userId: string) => {
    setActionInProgressUserId(userId);
    try {
      await onRemoveMember(teamId, userId);
    } catch {
      // The page handler already shows an error toast.
    } finally {
      setActionInProgressUserId(null);
    }
  };

  const handlePromote = async (userId: string) => {
    setActionInProgressUserId(userId);
    try {
      await onSetLeader(teamId, userId);
    } catch {
      // The page handler already shows an error toast.
    } finally {
      setActionInProgressUserId(null);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={team ? team.name : "Team Roster"}
      description={team ? (canManage ? "Squad roster & member management" : "Squad roster") : "Loading squad roster..."}
      className="max-w-3xl"
      footer={
        <div className="flex justify-end w-full">
          <Button variant="primary" size="sm" onClick={onClose}>
            Done
          </Button>
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs max-h-[70vh] overflow-y-auto pr-1">
        {isLoading || !team ? (
          <div className="space-y-3 py-4">
            <Skeleton className="h-6 w-1/3" />
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-20 w-full" />
          </div>
        ) : (
          <>
            {/* Header Summary */}
            <div className="rounded-[16px] border border-[var(--card-line)] p-4 bg-[var(--card-solid-subtle)] flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Layers className="h-4 w-4 text-[var(--color-blue)]" />
                <span className="font-bold text-[var(--ink-primary)] text-sm">{team.name}</span>
                <Badge variant="solid" size="sm">
                  {team.members.length}/{team.effective_max_size} Members
                </Badge>
                {team.effective_frozen && (
                  <Badge variant="amber" size="sm">
                    Frozen
                  </Badge>
                )}
                {team.join_code && (
                  <span className="font-mono tracking-widest text-[11px] text-[var(--ink-secondary)]">
                    {team.join_code}
                  </span>
                )}
              </div>

              {team.leader ? (
                <div className="flex items-center gap-1.5 text-[11px]">
                  <Crown className="h-3.5 w-3.5 text-[var(--ink-primary)]" />
                  <span className="text-[var(--ink-secondary)]">Leader:</span>
                  <strong className="text-[var(--ink-primary)]">{team.leader.name}</strong>
                </div>
              ) : (
                <span className="text-[var(--ink-tertiary)] italic text-[11px]">
                  No Leader Assigned
                </span>
              )}
            </div>

            {/* Add Member Form (staff only) */}
            {canManage && (
              <form
                onSubmit={handleAddSubmit}
                className="rounded-[16px] border border-[var(--card-line)] p-4 bg-[var(--card-solid)] space-y-2"
              >
                <div className="flex items-center justify-between">
                  <span className="font-bold text-[var(--ink-primary)] uppercase text-[11px] flex items-center gap-1">
                    <UserPlus className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                    <span>Add Participant to Squad</span>
                  </span>
                </div>

                <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-end">
                  <div className="flex-1">
                    <ParticipantPicker
                      value={selectedUser}
                      onChange={(u) => {
                        setSelectedUser(u);
                        if (addError) setAddError(null);
                      }}
                      disabled={isAdding}
                      error={addError || undefined}
                    />
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    type="submit"
                    disabled={!selectedUser || isAdding}
                    isLoading={isAdding}
                    className="h-9 text-xs shrink-0"
                  >
                    Enroll in Squad
                  </Button>
                </div>
              </form>
            )}

            {/* Members Table */}
            <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] overflow-hidden">
              <div className="px-4 py-3 bg-[var(--card-solid-subtle)] border-b border-[var(--card-line)] font-bold text-[11px] uppercase tracking-wider text-[var(--ink-primary)] flex items-center justify-between">
                <span>Roster Members ({team.members.length})</span>
                <Users className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              </div>

              {team.members.length === 0 ? (
                <div className="p-6 text-center text-[var(--ink-tertiary)] italic">
                  No participants currently assigned to this team.
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Member</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead>Password Setup</TableHead>
                      {canManage && <TableHead className="text-right">Manage</TableHead>}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {team.members.map((member) => {
                      const isLeader = Boolean(member.is_leader);
                      const isProcessing = actionInProgressUserId === member.id;

                      return (
                        <TableRow key={member.id}>
                          {/* Member info */}
                          <TableCell>
                            <div className="flex flex-col">
                              <div className="flex items-center gap-1 font-bold text-[var(--ink-primary)]">
                                <span>{member.name}</span>
                                {isLeader && (
                                  <span
                                    title="Designated Team Leader"
                                    className="inline-flex items-center gap-0.5 text-[9px] bg-[var(--color-dark)] text-[var(--ink-inverse)] px-1 py-0.2 rounded uppercase font-bold"
                                  >
                                    <Crown className="h-2.5 w-2.5" />
                                    Leader
                                  </span>
                                )}
                              </div>
                              <span className="text-[11px] text-[var(--ink-secondary)] font-mono">
                                {member.email}
                              </span>
                              {member.phone_number && (
                                <span className="text-[10px] text-[var(--ink-tertiary)]">
                                  {member.phone_number}
                                </span>
                              )}
                            </div>
                          </TableCell>

                          {/* Role */}
                          <TableCell>
                            <Badge variant="subtle" size="sm">
                              {member.role}
                            </Badge>
                          </TableCell>

                          {/* Password configured */}
                          <TableCell>
                            {member.is_password_configured ? (
                              <span className="inline-flex items-center gap-1.5 text-[11px] text-[var(--color-blue)] font-medium">
                                <CheckCircle2 className="h-3 w-3" />
                                Configured
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1.5 text-[11px] text-[var(--ink-tertiary)]">
                                <Clock className="h-3 w-3" />
                                Pending
                              </span>
                            )}
                          </TableCell>

                          {/* Manage actions */}
                          {canManage && (
                            <TableCell className="text-right">
                              <div className="flex items-center justify-end gap-1.5">
                                {!isLeader && (
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    disabled={isProcessing}
                                    onClick={() => handlePromote(member.id)}
                                    title="Set as team leader"
                                    className="h-6 px-2 text-[10px]"
                                  >
                                    Set Leader
                                  </Button>
                                )}

                                <Button
                                  variant="ghost"
                                  size="sm"
                                  disabled={isProcessing}
                                  onClick={() => handleRemove(member.id)}
                                  title="Remove member from team"
                                  className="h-6 w-6 p-0 text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)]"
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              </div>
                            </TableCell>
                          )}
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              )}
            </div>
          </>
        )}
      </div>
    </Dialog>
  );
}
