"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";

export default function ParticipantDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="participant"
      title="Participant Portal"
      description="View your digital event ticket pass, track real-time queue position, and submit project links."
    />
  );
}
