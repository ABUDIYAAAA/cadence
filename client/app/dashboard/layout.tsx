import React from "react";
import { Sidebar } from "@/components/layout/sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] w-full font-sans bg-[var(--background)]">
      <Sidebar />
      <div className="flex-1 min-w-0">
        <div className="w-full px-5 sm:px-8 lg:px-10 py-6 sm:py-8">
          {children}
        </div>
      </div>
    </div>
  );
}
