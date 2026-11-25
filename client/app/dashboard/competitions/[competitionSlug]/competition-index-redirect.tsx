"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { getCompetitionSectionHref } from "@/lib/competition-utils";
import { AdminTablePageSkeleton } from "@/components/ui/skeleton";

export default function CompetitionIndexPage() {
  const router = useRouter();
  const params = useParams<{ competitionSlug: string }>();

  const rawSlug = params?.competitionSlug ?? "";
  let slug = rawSlug;
  try {
    slug = decodeURIComponent(rawSlug);
  } catch {
    // keep raw
  }

  useEffect(() => {
    if (!slug) return;
    router.replace(getCompetitionSectionHref(slug, "teams"));
  }, [slug, router]);

  return <AdminTablePageSkeleton showMetrics />;
}

