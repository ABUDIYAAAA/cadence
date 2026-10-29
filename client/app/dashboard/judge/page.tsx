"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { JudgeEvaluationView } from "@/components/judging";

export default function JudgeDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="judge"
      title="Judging panel"
      description="Evaluate assigned team presentations, review submissions, and submit ratings."
    >
      <div className="pt-2">
        <JudgeEvaluationView />
      </div>
    </RoleDashboard>
  );
}
