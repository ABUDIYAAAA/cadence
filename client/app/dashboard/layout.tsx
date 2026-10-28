import React from "react";
import { Sidebar } from "@/components/layout/sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] w-full bg-white text-black font-mono overflow-x-hidden">
      <Sidebar />
      <main className="flex-1 min-w-0 bg-neutral-50/40 overflow-y-auto overflow-x-hidden">
        {children}
      </main>
    </div>
  );
}
