import React from "react";
import { Sidebar } from "@/components/layout/sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] bg-white text-black font-mono">
      <Sidebar />
      <div className="flex-1 min-w-0 bg-neutral-50/40">{children}</div>
    </div>
  );
}
