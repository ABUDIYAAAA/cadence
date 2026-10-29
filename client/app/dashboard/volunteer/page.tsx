"use client";

import React from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { VolunteerScheduleView } from "@/components/judging";

export default function VolunteerDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="volunteer"
      title="Volunteer operations"
      description="Queue status coordination, room staging schedules, and gate check-in scanner."
    >
      <div className="space-y-6 font-sans">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Gate Scanner Card */}
          <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Gate check-in</CardTitle>
                <Badge variant="emerald" dot size="sm">
                  Active
                </Badge>
              </div>
              <CardDescription>
                Scan attendee QR codes via device camera or handheld scanner with duplicate check.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="text-xs text-[var(--ink-secondary)] space-y-1">
                <div>Camera and barcode scanner support</div>
                <div>Real-time duplicate scan detection</div>
              </div>
              <div className="pt-2">
                <Link href="/dashboard/tickets">
                  <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                    <span>Open scanner & attendees</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </Link>
              </div>
            </CardContent>
          </Card>

          {/* Teams Directory Card */}
          <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Teams</CardTitle>
                <Badge variant="subtle" size="sm">
                  Directory
                </Badge>
              </div>
              <CardDescription>
                Look up registered teams, verify attendee rosters, and locate team leads.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="text-xs text-[var(--ink-secondary)] space-y-1">
                <div>Inspect member rosters, emails, and phone numbers</div>
                <div>Check-in and account setup status</div>
              </div>
              <div className="pt-2">
                <Link href="/dashboard/teams">
                  <Button variant="outline" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                    <span>View teams directory</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </Link>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Live Judging Staging & Room Coordinator View */}
        <div className="pt-2">
          <VolunteerScheduleView />
        </div>
      </div>
    </RoleDashboard>
  );
}
