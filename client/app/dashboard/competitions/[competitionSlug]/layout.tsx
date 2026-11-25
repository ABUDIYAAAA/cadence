import React from "react";
import CompetitionScopeGuard from "./competition-scope-guard";

export const instant = false;

export default function CompetitionScopedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <CompetitionScopeGuard>{children}</CompetitionScopeGuard>;
}
