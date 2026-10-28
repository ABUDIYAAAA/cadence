"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";

export default function VolunteerDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="volunteer"
      title="Volunteer Coordination Center"
      description="Queue status coordination, team staging schedules, and gate check-in scanner controls."
    />
  );
}
