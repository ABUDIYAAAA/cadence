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
      <main className="flex-1 min-w-0 px-6 sm:px-8 lg:px-10 py-7 max-w-[1600px] w-full">
        {children}
      </main>
    </div>
  );
}
