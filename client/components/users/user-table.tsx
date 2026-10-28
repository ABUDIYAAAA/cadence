"use client";

import React from "react";
import {
  Mail,
  KeyRound,
  Edit2,
  Trash2,
  Eye,
  Crown,
  CheckCircle2,
  Clock,
} from "lucide-react";
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

interface UserTableProps {
  users: UserDetailResponse[];
  isLoading: boolean;
  onView: (user: UserDetailResponse) => void;
  onEdit: (user: UserDetailResponse) => void;
  onDelete: (user: UserDetailResponse) => void;
  onSendInvite: (user: UserDetailResponse) => void;
  onResetPassword: (user: UserDetailResponse) => void;
  actionInProgressId?: string | null;
}

export function UserTable({
  users,
  isLoading,
  onView,
  onEdit,
  onDelete,
  onSendInvite,
  onResetPassword,
  actionInProgressId,
}: UserTableProps) {
  if (isLoading) {
    return (
      <div className="border border-neutral-300 bg-white p-4 font-mono space-y-3">
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex items-center space-x-4 py-2">
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/4" />
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
        description="No users matched your current query or filter criteria. Try adjusting your filters or create a new user account."
      />
    );
  }

  const getRoleBadgeVariant = (role: string) => {
    switch (role) {
      case "admin":
        return "solid";
      case "judge":
        return "outline";
      case "volunteer":
        return "subtle";
      default:
        return "subtle";
    }
  };

  return (
    <div className="border border-neutral-300 bg-white overflow-hidden font-mono">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>User</TableHead>
            <TableHead>Role</TableHead>
            <TableHead>Team</TableHead>
            <TableHead>Password Setup</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => {
            const isProcessing = actionInProgressId === user.id;

            return (
              <TableRow key={user.id} className="group">
                {/* User Name & Email */}
                <TableCell className="font-mono">
                  <div className="flex flex-col">
                    <div className="flex items-center gap-1.5 font-bold text-black">
                      <span>{user.name}</span>
                      {user.is_leader && (
                        <span
                          title="Team Leader"
                          className="inline-flex items-center gap-0.5 text-[9px] bg-black text-white px-1 py-0.2 tracking-wider uppercase font-bold"
                        >
                          <Crown className="h-2.5 w-2.5" />
                          Lead
                        </span>
                      )}
                    </div>
                    <span className="text-xs text-neutral-500 font-mono">
                      {user.email}
                    </span>
                    {user.phone_number && (
                      <span className="text-[10px] text-neutral-400">
                        {user.phone_number}
                      </span>
                    )}
                  </div>
                </TableCell>

                {/* Role */}
                <TableCell>
                  <Badge variant={getRoleBadgeVariant(user.role)} size="sm">
                    {user.role}
                  </Badge>
                </TableCell>

                {/* Team */}
                <TableCell className="text-xs">
                  {user.team_name ? (
                    <span className="font-semibold text-black">
                      {user.team_name}
                    </span>
                  ) : (
                    <span className="text-neutral-400 italic">None</span>
                  )}
                </TableCell>

                {/* Password / Onboarding */}
                <TableCell>
                  {user.is_password_configured ? (
                    <span className="inline-flex items-center gap-1 text-[11px] text-neutral-800 font-medium">
                      <CheckCircle2 className="h-3 w-3 text-black" />
                      Configured
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500 font-medium">
                      <Clock className="h-3 w-3 text-neutral-400" />
                      Pending Setup
                    </span>
                  )}
                </TableCell>

                {/* Status */}
                <TableCell>
                  <Badge
                    variant={user.is_active ? "solid" : "subtle"}
                    size="sm"
                    className={
                      user.is_active
                        ? "bg-black text-white"
                        : "bg-neutral-200 text-neutral-600 border-neutral-300"
                    }
                  >
                    {user.is_active ? "Active" : "Inactive"}
                  </Badge>
                </TableCell>

                {/* Created */}
                <TableCell className="text-xs text-neutral-500">
                  {new Date(user.created_at).toLocaleDateString()}
                </TableCell>

                {/* Actions */}
                <TableCell className="text-right">
                  <div className="flex items-center justify-end gap-1">
                    {/* View Details */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onView(user)}
                      title="Inspect User Details"
                      className="h-7 w-7 p-0"
                    >
                      <Eye className="h-3.5 w-3.5" />
                    </Button>

                    {/* Edit User */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onEdit(user)}
                      title="Edit User"
                      className="h-7 w-7 p-0"
                    >
                      <Edit2 className="h-3.5 w-3.5" />
                    </Button>

                    {/* Send Invite (if pending setup) */}
                    {!user.is_password_configured && (
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={isProcessing}
                        onClick={() => onSendInvite(user)}
                        title="Send Onboarding Invite Email"
                        className="h-7 w-7 p-0 text-black hover:bg-neutral-100"
                      >
                        <Mail className="h-3.5 w-3.5" />
                      </Button>
                    )}

                    {/* Trigger Password Reset */}
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={isProcessing}
                      onClick={() => onResetPassword(user)}
                      title="Trigger Password Reset Email"
                      className="h-7 w-7 p-0"
                    >
                      <KeyRound className="h-3.5 w-3.5" />
                    </Button>

                    {/* Delete User */}
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={isProcessing}
                      onClick={() => onDelete(user)}
                      title="Delete User Account"
                      className="h-7 w-7 p-0 text-neutral-700 hover:bg-black hover:text-white"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
