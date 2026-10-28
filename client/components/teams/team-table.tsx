"use client";

import React from "react";
import { Users, Crown, Edit2, Trash2, Layers } from "lucide-react";
import { TeamResponse } from "@/types/teams";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";

interface TeamTableProps {
  teams: TeamResponse[];
  isLoading: boolean;
  onView: (team: TeamResponse) => void;
  onEdit: (team: TeamResponse) => void;
  onDelete: (team: TeamResponse) => void;
  canManage?: boolean;
}

export function TeamTable({
  teams,
  isLoading,
  onView,
  onEdit,
  onDelete,
  canManage = true,
}: TeamTableProps) {
  if (isLoading) {
    return (
      <div className="space-y-3 font-sans">
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex items-center space-x-4 py-2">
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (teams.length === 0) {
    return (
      <EmptyState
        title="No Teams Found"
        description="No teams matched your query. Create a new team or adjust search terms."
      />
    );
  }

  return (
    <div className="w-full overflow-x-auto font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Team Name</TableHead>
            <TableHead>Team Leader</TableHead>
            <TableHead>Roster Size</TableHead>
            <TableHead>Registered</TableHead>
            <TableHead>Last Updated</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {teams.map((team) => (
            <TableRow key={team.id} className="group">
              {/* Team Name */}
              <TableCell>
                <div className="flex flex-col">
                  <div className="flex items-center gap-1.5 font-bold text-[var(--ink-primary)] text-sm">
                    <Layers className="h-4 w-4 shrink-0 text-[var(--color-blue)]" />
                    <span>{team.name}</span>
                  </div>
                  <span className="text-[10px] text-[var(--ink-tertiary)] font-mono">
                    ID: {team.id.slice(0, 8)}...
                  </span>
                </div>
              </TableCell>

              {/* Leader */}
              <TableCell className="text-xs">
                {team.leader_name ? (
                  <div className="flex items-center gap-1.5">
                    <Crown className="h-3.5 w-3.5 text-[var(--color-blue)] shrink-0" />
                    <div className="flex flex-col">
                      <span className="font-semibold text-[var(--ink-primary)]">
                        {team.leader_name}
                      </span>
                      {team.leader_email && (
                        <span className="text-[10px] text-[var(--ink-secondary)] font-mono">
                          {team.leader_email}
                        </span>
                      )}
                    </div>
                  </div>
                ) : (
                  <span className="text-[var(--ink-tertiary)] italic text-[11px]">
                    No Leader Assigned
                  </span>
                )}
              </TableCell>

              {/* Member Count */}
              <TableCell>
                <Badge
                  variant={team.member_count > 0 ? "solid" : "subtle"}
                  size="sm"
                  className="gap-1"
                >
                  <Users className="h-3 w-3" />
                  <span>
                    {team.member_count}{" "}
                    {team.member_count === 1 ? "Member" : "Members"}
                  </span>
                </Badge>
              </TableCell>

              {/* Created */}
              <TableCell className="text-xs text-[var(--ink-secondary)] font-mono">
                {new Date(team.created_at).toLocaleDateString()}
              </TableCell>

              {/* Updated */}
              <TableCell className="text-xs text-[var(--ink-secondary)] font-mono">
                {new Date(team.updated_at).toLocaleDateString()}
              </TableCell>

              {/* Actions */}
              <TableCell className="text-right">
                <div className="flex items-center justify-end gap-1">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => onView(team)}
                    title="View Roster & Manage Members"
                    className="h-7 px-2.5 text-xs gap-1"
                  >
                    <Users className="h-3.5 w-3.5" />
                    <span>Roster</span>
                  </Button>

                  {canManage && (
                    <>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => onEdit(team)}
                        title="Edit Team Details"
                        className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                      >
                        <Edit2 className="h-3.5 w-3.5" />
                      </Button>

                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => onDelete(team)}
                        title="Delete Team"
                        className="h-7 w-7 p-0 text-[var(--color-coral)] hover:bg-[#CE6969]/10"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
