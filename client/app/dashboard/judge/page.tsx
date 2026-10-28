"use client";

import React from "react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { JudgeEvaluationView } from "@/components/judging";

export default function JudgeDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="judge"
      title="Judge Evaluation Panel"
      description="Scorecard interface to evaluate assigned team slots, review submitted code repositories, run countdown timers, and lock in ratings."
    >
      <div className="pt-2">
        <JudgeEvaluationView />
      </div>
    </RoleDashboard>
  );
}
