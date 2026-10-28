"use client";

import React from "react";
import { useMyTeam } from "@/hooks/use-teams";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Layers, Crown, Users, CheckCircle2, Clock } from "lucide-react";

export function MyTeamView() {
  const { data: team, isLoading, error } = useMyTeam();

  if (isLoading) {
    return (
      <div className="space-y-4 font-sans">
        <Skeleton className="h-8 w-1/4 rounded-xl" />
        <Skeleton className="h-32 w-full rounded-[20px]" />
      </div>
    );
  }

  if (error || !team) {
    return (
      <EmptyState
        title="No Team Assigned Yet"
        description="You have not been assigned to a hackathon squad yet. Reach out to an organizer or event administrator to link your account to a team."
      />
    );
  }

  return (
    <div className="space-y-6 font-sans">
      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <Badge variant="subtle" size="sm">
                  {team.members.length} {team.members.length === 1 ? "participant" : "participants"}
                </Badge>
              </div>
              <CardTitle className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)] flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Layers className="h-4 w-4" />
                </div>
                <span>{team.name}</span>
              </CardTitle>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1 ml-10">
                Squad ID: <span className="font-mono text-[var(--ink-primary)]">{team.id}</span>
              </CardDescription>
            </div>

            {team.leader && (
              <div className="flex items-center gap-2.5 rounded-[16px] border border-[var(--card-line)] p-3 px-4 bg-[var(--card-solid-subtle)]">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)] shrink-0">
                  <Crown className="h-4 w-4" />
                </div>
                <div className="text-left">
                  <div className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)] tracking-wider">
                    Squad Leader
                  </div>
                  <div className="text-xs font-bold text-[var(--ink-primary)]">
                    {team.leader.name}
                  </div>
                </div>
              </div>
            )}
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="overflow-hidden">
            <div className="px-4 py-3 border-b border-[var(--card-line)] font-bold text-xs uppercase tracking-wider text-[var(--ink-secondary)] flex items-center justify-between">
              <span>Squad Roster</span>
              <Users className="h-4 w-4 text-[var(--ink-tertiary)]" />
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Teammate</TableHead>
                  <TableHead>System Role</TableHead>
                  <TableHead>Account Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {team.members.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <div className="flex flex-col">
                        <div className="flex items-center gap-2 font-bold text-[var(--ink-primary)]">
                          <span>{member.name}</span>
                          {member.is_leader && (
                            <Badge variant="blue" size="sm" className="gap-1">
                              <Crown className="h-2.5 w-2.5" />
                              Leader
                            </Badge>
                          )}
                        </div>
                        <span className="text-xs text-[var(--ink-secondary)] font-mono mt-0.5">
                          {member.email}
                        </span>
                        {member.phone_number && (
                          <span className="text-[10px] text-[var(--ink-tertiary)] mt-0.5">
                            {member.phone_number}
                          </span>
                        )}
                      </div>
                    </TableCell>

                    <TableCell>
                      <Badge variant="subtle" size="sm">
                        {member.role}
                      </Badge>
                    </TableCell>

                    <TableCell>
                      {member.is_password_configured ? (
                        <span className="inline-flex items-center gap-1.5 text-xs text-[#2EB88A] font-semibold">
                          <CheckCircle2 className="h-3.5 w-3.5" />
                          Ready
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-xs text-[var(--ink-tertiary)]">
                          <Clock className="h-3.5 w-3.5" />
                          Pending Setup
                        </span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
