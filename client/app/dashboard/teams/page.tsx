"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Trophy } from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { getCompetitionSectionHref } from "@/lib/competition-utils";
import { AdminTablePageSkeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";

export default function TeamsRedirectPage() {
  const router = useRouter();
  const { user, isAdmin, isJudge, isLoading: authLoading } = useAuth();
  const { competitionSlug, isLoadingCompetitions } = useTenant();

  useEffect(() => {
    if (authLoading || isLoadingCompetitions) return;
    if (!user) {
      router.replace("/login");
      return;
    }
    if (competitionSlug) {
      router.replace(getCompetitionSectionHref(competitionSlug, "teams"));
    }
  }, [authLoading, isLoadingCompetitions, user, competitionSlug, router]);

  if (authLoading || isLoadingCompetitions || !user || competitionSlug) {
    return <AdminTablePageSkeleton showMetrics />;
  }

  return (
    <div className="space-y-6 font-sans">
      <div>
        <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
          {isJudge && !isAdmin ? "My Teams" : "Teams"}
        </h1>
        <p className="text-sm text-[var(--ink-secondary)] mt-1">
          Participants create their own teams and share a join code. Freeze, edit or review them here.
        </p>
      </div>
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5">
        <EmptyState
          icon={Trophy}
          title="No competition in this event yet"
          description="Teams are scoped to a competition. Create a competition first to enable team registration."
          action={
            isAdmin ? (
              <Link href="/dashboard/competitions">
                <Button variant="primary" size="sm" className="h-9 px-4 text-xs font-semibold">
                  Go to competitions
                </Button>
              </Link>
            ) : undefined
          }
        />
      </div>
    </div>
  );
}
