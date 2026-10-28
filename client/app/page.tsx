"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { SkeletonCard } from "@/components/ui/skeleton";

export default function RootPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (!isLoading) {
      if (isAuthenticated && user) {
        router.replace(getDashboardRoute(user.role));
      } else {
        router.replace("/login");
      }
    }
  }, [isLoading, isAuthenticated, user, router]);

  return (
    <div className="mx-auto max-w-xl p-8 font-mono min-h-[60vh] flex items-center justify-center">
      <div className="w-full">
        <SkeletonCard />
      </div>
    </div>
  );
}
