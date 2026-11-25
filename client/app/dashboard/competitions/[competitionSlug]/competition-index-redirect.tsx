"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { getCompetitionSectionHref } from "@/lib/competition-utils";
import { AdminTablePageSkeleton } from "@/components/ui/skeleton";

export default function CompetitionIndexPage() {
  const router = useRouter();
  const params = useParams<{ competitionSlug: string }>();
  const { isAdmin, isVolunteer, isJudge, isParticipant, isLoading } = useAuth();

  const rawSlug = params?.competitionSlug ?? "";
  let slug = rawSlug;
  try {
    slug = decodeURIComponent(rawSlug);
  } catch {
    // keep raw
  }

  useEffect(() => {
    if (isLoading || !slug) return;
    const participantOnly = isParticipant && !isAdmin && !isVolunteer && !isJudge;
    const defaultSection = participantOnly ? "judging" : "teams";
    router.replace(getCompetitionSectionHref(slug, defaultSection));
  }, [isLoading, slug, isParticipant, isAdmin, isVolunteer, isJudge, router]);

  return <AdminTablePageSkeleton showMetrics />;
}
