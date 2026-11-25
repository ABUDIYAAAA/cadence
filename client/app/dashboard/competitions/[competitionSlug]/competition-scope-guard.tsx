"use client";

import React, { useEffect, useMemo } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Trophy } from "lucide-react";
import { useTenant } from "@/providers/tenant-provider";
import { AdminTablePageSkeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";

export default function CompetitionScopedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const params = useParams<{ competitionSlug: string }>();
  const rawSlug = params?.competitionSlug ?? "";
  const routeSlug = useMemo(() => {
    try {
      return decodeURIComponent(rawSlug);
    } catch {
      return rawSlug;
    }
  }, [rawSlug]);

  const {
    competitionSlug: activeSlug,
    competitions,
    isLoadingCompetitions,
    selectCompetition,
  } = useTenant();

  const matchedCompetition = useMemo(
    () => competitions.find((c) => c.slug === routeSlug) ?? null,
    [competitions, routeSlug]
  );

  useEffect(() => {
    if (isLoadingCompetitions) return;
    if (matchedCompetition && activeSlug !== matchedCompetition.slug) {
      selectCompetition(matchedCompetition.slug);
    }
  }, [isLoadingCompetitions, matchedCompetition, activeSlug, selectCompetition]);

  if (isLoadingCompetitions) {
    return <AdminTablePageSkeleton showMetrics />;
  }

  if (!matchedCompetition) {
    return (
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 font-sans">
        <EmptyState
          icon={Trophy}
          title="Competition not found"
          description={`We couldn't find a competition matching "${routeSlug}" in this event.`}
          action={
            <Link href="/dashboard/competitions">
              <Button variant="primary" size="sm" className="h-9 px-4 text-xs gap-1.5 font-semibold">
                <ArrowLeft className="h-3.5 w-3.5" />
                Back to competitions
              </Button>
            </Link>
          }
        />
      </div>
    );
  }

  // Wait until TenantProvider & apiClient scope match the route slug before rendering children
  if (activeSlug !== matchedCompetition.slug) {
    return <AdminTablePageSkeleton showMetrics />;
  }

  return <>{children}</>;
}
