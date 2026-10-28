"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";

export default function JudgeDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="judge"
      title="Judge Evaluation Panel"
      description="Scorecard interface to evaluate assigned team slots, enter criterion ratings, and lock submissions."
    />
  );
}
