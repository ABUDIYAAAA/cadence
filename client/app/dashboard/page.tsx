"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { getTenantScope } from "@/lib/api-client";
import { getAuthenticatedHomeRoute } from "@/lib/auth-routes";
import { SkeletonCard } from "@/components/ui/skeleton";

export default function DashboardRedirectPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (!isLoading) {
      if (!isAuthenticated || !user) {
        router.replace("/login");
      } else {
        router.replace(getAuthenticatedHomeRoute(user, getTenantScope().eventSlug));
      }
    }
  }, [isLoading, isAuthenticated, user, router]);

  return (
    <div className="font-mono">
      <SkeletonCard />
    </div>
  );
}
