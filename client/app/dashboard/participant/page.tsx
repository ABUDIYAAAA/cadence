"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { MyTicketView } from "@/components/tickets/my-ticket-view";
import { MyTeamView } from "@/components/teams/my-team-view";

export default function ParticipantDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="participant"
      title="Participant Portal"
      description="View your digital event ticket pass, track real-time queue position, and submit project links."
    >
      <div className="pt-2 space-y-6">
        <MyTicketView />
        <MyTeamView />
      </div>
    </RoleDashboard>
  );
}
