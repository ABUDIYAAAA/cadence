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
      <div className="space-y-4 font-mono">
        <Skeleton className="h-8 w-1/4" />
        <Skeleton className="h-32 w-full" />
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
    <div className="space-y-6 font-mono">
      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                  MY SQUAD
                </span>
                <span className="text-xs text-neutral-500">
                  {team.members.length} {team.members.length === 1 ? "PARTICIPANT" : "PARTICIPANTS"}
                </span>
              </div>
              <CardTitle className="text-xl sm:text-2xl flex items-center gap-2">
                <Layers className="h-6 w-6 text-black" />
                <span>{team.name}</span>
              </CardTitle>
              <CardDescription>
                Squad ID: <span className="font-mono text-neutral-700">{team.id}</span>
              </CardDescription>
            </div>

            {team.leader && (
              <div className="flex items-center gap-2 border border-black p-2 bg-neutral-50">
                <Crown className="h-4 w-4 text-black shrink-0" />
                <div className="text-left">
                  <div className="text-[10px] uppercase font-bold text-neutral-500">
                    Squad Leader
                  </div>
                  <div className="text-xs font-bold text-black">
                    {team.leader.name}
                  </div>
                </div>
              </div>
            )}
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="border border-neutral-300 bg-white overflow-hidden">
            <div className="px-4 py-2 bg-neutral-100 border-b border-neutral-300 font-bold text-xs uppercase tracking-wider text-black flex items-center justify-between">
              <span>Squad Roster</span>
              <Users className="h-4 w-4 text-neutral-500" />
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
                        <div className="flex items-center gap-1.5 font-bold text-black">
                          <span>{member.name}</span>
                          {member.is_leader && (
                            <span className="inline-flex items-center gap-0.5 text-[9px] bg-black text-white px-1.5 py-0.2 uppercase font-bold">
                              <Crown className="h-2.5 w-2.5" />
                              Leader
                            </span>
                          )}
                        </div>
                        <span className="text-xs text-neutral-500 font-mono">
                          {member.email}
                        </span>
                        {member.phone_number && (
                          <span className="text-[10px] text-neutral-400">
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
                        <span className="inline-flex items-center gap-1 text-xs text-black font-semibold">
                          <CheckCircle2 className="h-3 w-3" />
                          Ready
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-xs text-neutral-500">
                          <Clock className="h-3 w-3" />
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
