import React from "react";
import { Sidebar } from "@/components/layout/sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="max-w-[1400px] mx-auto px-4 sm:px-6 lg:px-8 py-6 flex gap-6 items-start font-sans min-h-[calc(100vh-4rem)]">
      <Sidebar />
      <main className="flex-1 min-w-0 animate-in fade-in-0 duration-200">
        {children}
      </main>
    </div>
  );
}
