"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { MyTicketView } from "@/components/tickets/my-ticket-view";
import { MyTeamView } from "@/components/teams/my-team-view";
import { ParticipantJudgingStatusView } from "@/components/judging";

export default function ParticipantDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="participant"
      title="Participant portal"
      description="View your event pass, check judging schedule, and manage your team submission."
    >
      <div className="pt-2 space-y-8">
        <MyTicketView />
        <ParticipantJudgingStatusView />
        <MyTeamView />
      </div>
    </RoleDashboard>
  );
}
