"use client";

import React from "react";
import { useMyTeam } from "@/hooks/use-teams";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Crown } from "lucide-react";

export function MyTeamView() {
  const { data: team, isLoading, error } = useMyTeam();

  if (isLoading) {
    return <Skeleton className="h-40 w-full rounded-[20px]" />;
  }

  if (error || !team) {
    return (
      <EmptyState
        title="No team yet"
        description="Ask an organizer to add you to a team."
      />
    );
  }

  return (
    <section className="font-sans space-y-3" aria-label="Your team">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="text-base font-semibold tracking-tight text-[var(--ink-primary)] truncate">
          {team.name}
        </h2>
        <span className="text-xs text-[var(--ink-secondary)] shrink-0 tabular-nums">
          {team.members.length} {team.members.length === 1 ? "member" : "members"}
        </span>
      </div>

      <ul className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] divide-y divide-[var(--card-line)] overflow-hidden">
        {team.members.map((member) => (
          <li key={member.id} className="flex items-center justify-between gap-3 px-4 py-3">
            <div className="min-w-0">
              <div className="text-sm font-medium text-[var(--ink-primary)] truncate">
                {member.name}
              </div>
              <div className="text-xs text-[var(--ink-secondary)] truncate">{member.email}</div>
            </div>
            {member.is_leader && (
              <Badge variant="blue" size="sm" className="gap-1 shrink-0">
                <Crown className="h-2.5 w-2.5" />
                Lead
              </Badge>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
