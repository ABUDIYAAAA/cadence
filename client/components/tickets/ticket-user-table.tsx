"use client";

import React from "react";
import { QrCode, CheckSquare, Square } from "lucide-react";
import { UserDetailResponse } from "@/types/users";
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

interface TicketUserTableProps {
  users: UserDetailResponse[];
  isLoading: boolean;
  selectedIds: string[];
  onToggleSelect: (userId: string) => void;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onSendSingle: (user: UserDetailResponse) => void;
  canSendTickets?: boolean;
  actionInProgressId?: string | null;
}

export function TicketUserTable({
  users,
  isLoading,
  selectedIds,
  onToggleSelect,
  onSelectAll,
  onClearSelection,
  onSendSingle,
  canSendTickets = true,
  actionInProgressId,
}: TicketUserTableProps) {
  if (isLoading) {
    return (
      <div className="space-y-3 font-sans">
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex items-center space-x-4 py-2">
              <Skeleton className="h-4 w-8" />
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (users.length === 0) {
    return (
      <EmptyState
        title="No Users Found"
        description="No attendee accounts matched your filter criteria."
      />
    );
  }

  const allSelected =
    users.length > 0 && users.every((u) => selectedIds.includes(u.id));

  return (
    <div className="w-full overflow-x-auto font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            {canSendTickets && (
              <TableHead className="w-10 text-center">
                <button
                  type="button"
                  onClick={allSelected ? onClearSelection : onSelectAll}
                  className="p-1 hover:text-[var(--ink-primary)] cursor-pointer"
                  title={allSelected ? "Deselect all" : "Select all on page"}
                >
                  {allSelected ? (
                    <CheckSquare className="h-4 w-4 text-[var(--color-blue)]" />
                  ) : (
                    <Square className="h-4 w-4 text-[var(--ink-tertiary)]" />
                  )}
                </button>
              </TableHead>
            )}
            <TableHead>Attendee</TableHead>
            <TableHead>System Role</TableHead>
            <TableHead>Squad / Team</TableHead>
            <TableHead>Account Status</TableHead>
            {canSendTickets && <TableHead className="text-right">Actions</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => {
            const isSelected = selectedIds.includes(user.id);
            const isProcessing = actionInProgressId === user.id;

            return (
              <TableRow
                key={user.id}
                className={`group ${
                  isSelected ? "opacity-95" : ""
                }`}
              >
                {/* Select Checkbox */}
                {canSendTickets && (
                  <TableCell className="text-center">
                    <button
                      type="button"
                      onClick={() => onToggleSelect(user.id)}
                      className="p-1 cursor-pointer"
                    >
                      {isSelected ? (
                        <CheckSquare className="h-4 w-4 text-[var(--color-blue)]" />
                      ) : (
                        <Square className="h-4 w-4 text-[var(--ink-tertiary)]" />
                      )}
                    </button>
                  </TableCell>
                )}

                {/* Name & Email */}
                <TableCell>
                  <div className="flex flex-col">
                    <div className="flex items-center gap-1.5 font-medium text-[var(--ink-primary)] text-sm">
                      <span>{user.name}</span>
                      {user.is_leader && (
                        <Badge variant="amber" size="sm">
                          Lead
                        </Badge>
                      )}
                    </div>
                    <span className="text-xs text-[var(--ink-secondary)] font-mono">
                      {user.email}
                    </span>
                    {user.phone_number && (
                      <span className="text-[10px] text-[var(--ink-tertiary)]">
                        {user.phone_number}
                      </span>
                    )}
                  </div>
                </TableCell>

                {/* Role */}
                <TableCell>
                  <Badge
                    variant={
                      user.role === "admin"
                        ? "indigo"
                        : user.role === "judge"
                        ? "amber"
                        : user.role === "volunteer"
                        ? "emerald"
                        : "subtle"
                    }
                    size="sm"
                  >
                    {user.role}
                  </Badge>
                </TableCell>

                {/* Team */}
                <TableCell className="text-xs">
                  {user.team_name ? (
                    <span className="font-semibold text-[var(--ink-primary)]">
                      {user.team_name}
                    </span>
                  ) : (
                    <span className="text-[var(--ink-tertiary)] italic">Unassigned</span>
                  )}
                </TableCell>

                {/* Account status */}
                <TableCell>
                  <Badge
                    variant={user.is_active ? "emerald" : "subtle"}
                    dot
                    size="sm"
                  >
                    {user.is_active ? "Active" : "Inactive"}
                  </Badge>
                </TableCell>

                {/* Single Send Action */}
                {canSendTickets && (
                  <TableCell className="text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={isProcessing}
                      onClick={() => onSendSingle(user)}
                      title="Generate & Email QR Entry Pass"
                      className="h-7 px-2.5 text-xs gap-1"
                    >
                      <QrCode className="h-3 w-3" />
                      <span>Send QR</span>
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
