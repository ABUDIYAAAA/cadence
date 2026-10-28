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
import { useUsers } from "@/hooks/use-users";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
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
  const { data: usersData, isLoading: usersLoading } = useUsers({ limit: 100 });
  const [newUserId, setNewUserId] = useState("");
  const [isAdding, setIsAdding] = useState(false);
  const [actionInProgressUserId, setActionInProgressUserId] = useState<string | null>(null);
  const [addError, setAddError] = useState<string | null>(null);

  if (!open || !teamId) return null;

  const availableUsers = (usersData?.data || []).filter(
    (u) => !team?.members.some((m) => m.id === u.id)
  );

  const handleAddSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUserId.trim()) {
      setAddError("Please select a participant from the list");
      return;
    }

    setIsAdding(true);
    setAddError(null);
    try {
      await onAddMember(teamId, newUserId.trim());
      setNewUserId("");
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
    } finally {
      setActionInProgressUserId(null);
    }
  };

  const handlePromote = async (userId: string) => {
    setActionInProgressUserId(userId);
    try {
      await onSetLeader(teamId, userId);
    } finally {
      setActionInProgressUserId(null);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={team ? team.name : "Team Roster"}
      description={team ? `Squad Roster & Member Management` : "Loading squad roster..."}
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
            <div className="border border-neutral-300 p-3 bg-neutral-50 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Layers className="h-4 w-4 text-black" />
                <span className="font-bold text-black text-sm">{team.name}</span>
                <Badge variant="solid" size="sm">
                  {team.members.length} {team.members.length === 1 ? "Member" : "Members"}
                </Badge>
              </div>

              {team.leader ? (
                <div className="flex items-center gap-1.5 text-[11px]">
                  <Crown className="h-3.5 w-3.5 text-black" />
                  <span className="text-neutral-500">Leader:</span>
                  <strong className="text-black">{team.leader.name}</strong>
                </div>
              ) : (
                <span className="text-neutral-400 italic text-[11px]">
                  No Leader Assigned
                </span>
              )}
            </div>

            {/* Add Member Form (Admin only) */}
            {canManage && (
              <form
                onSubmit={handleAddSubmit}
                className="border border-neutral-300 p-3 bg-white space-y-2"
              >
                <div className="flex items-center justify-between">
                  <span className="font-bold text-black uppercase text-[11px] flex items-center gap-1">
                    <UserPlus className="h-3.5 w-3.5" />
                    <span>Add Participant to Squad</span>
                  </span>
                </div>

                <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-end">
                  <div className="flex-1">
                    <Select
                      value={newUserId}
                      onChange={(e) => {
                        setNewUserId(e.target.value);
                        if (addError) setAddError(null);
                      }}
                      disabled={isAdding || usersLoading}
                      error={addError || undefined}
                      className="h-9 text-xs font-mono"
                    >
                      <option value="">Select a user to enroll in this squad...</option>
                      {availableUsers.map((u) => (
                        <option key={u.id} value={u.id}>
                          {u.name} ({u.email}) — [{u.role.toUpperCase()}]
                          {u.team_name ? ` (Current Team: ${u.team_name})` : " (No Team)"}
                        </option>
                      ))}
                    </Select>
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    type="submit"
                    disabled={!newUserId || isAdding}
                    isLoading={isAdding}
                    className="h-9 text-xs shrink-0"
                  >
                    Enroll in Squad
                  </Button>
                </div>
              </form>
            )}

            {/* Members Table */}
            <div className="border border-neutral-300 bg-white overflow-hidden">
              <div className="px-3 py-2 bg-neutral-100 border-b border-neutral-300 font-bold text-[11px] uppercase tracking-wider text-neutral-700 flex items-center justify-between">
                <span>Roster Members ({team.members.length})</span>
                <Users className="h-3.5 w-3.5 text-neutral-500" />
              </div>

              {team.members.length === 0 ? (
                <div className="p-6 text-center text-neutral-400 italic">
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
                              <div className="flex items-center gap-1 font-bold text-black">
                                <span>{member.name}</span>
                                {isLeader && (
                                  <span
                                    title="Designated Team Leader"
                                    className="inline-flex items-center gap-0.5 text-[9px] bg-black text-white px-1 py-0.2 tracking-wider uppercase font-bold"
                                  >
                                    <Crown className="h-2.5 w-2.5" />
                                    Leader
                                  </span>
                                )}
                              </div>
                              <span className="text-[11px] text-neutral-500 font-mono">
                                {member.email}
                              </span>
                              {member.phone_number && (
                                <span className="text-[10px] text-neutral-400">
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
                              <span className="inline-flex items-center gap-1 text-[11px] text-black font-medium">
                                <CheckCircle2 className="h-3 w-3" />
                                Configured
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
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
                                  className="h-6 w-6 p-0 text-neutral-600 hover:bg-black hover:text-white"
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
