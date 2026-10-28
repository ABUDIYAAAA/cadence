"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { SkeletonCard } from "@/components/ui/skeleton";

export default function DashboardRedirectPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (!isLoading) {
      if (!isAuthenticated || !user) {
        router.replace("/login");
      } else {
        router.replace(getDashboardRoute(user.role));
      }
    }
  }, [isLoading, isAuthenticated, user, router]);

  return (
    <div className="mx-auto max-w-4xl p-6 font-mono">
      <SkeletonCard />
    </div>
  );
}
