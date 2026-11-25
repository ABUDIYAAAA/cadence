"use client";

import React, { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { AdminTablePageSkeleton } from "@/components/ui/skeleton";

export default function EventsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const { isAuthenticated, isDev, isLoading } = useAuth();
  const { eventSlug } = useTenant();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !isAuthenticated) {
    return (
      <div className="mx-auto w-full max-w-5xl px-5 sm:px-8 py-8 sm:py-10 font-sans">
        <AdminTablePageSkeleton />
      </div>
    );
  }

  const hasSidebar = isDev || Boolean(eventSlug);

  return (
    <div
      className={
        hasSidebar
          ? "w-full px-5 sm:px-8 lg:px-10 py-6 sm:py-8"
          : "mx-auto w-full max-w-5xl px-5 sm:px-8 py-8 sm:py-10"
      }
    >
      {children}
    </div>
  );
}
