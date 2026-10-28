"use client";

import React from "react";
import Link from "next/link";
import { Layers, Users, ArrowRight, ShieldCheck, QrCode, Scan } from "lucide-react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { VolunteerScheduleView } from "@/components/judging";

export default function VolunteerDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="volunteer"
      title="Volunteer Coordination Center"
      description="Queue status coordination, team staging schedules, and gate check-in scanner controls."
    >
      <div className="space-y-8 font-sans">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Gate Scanner Card */}
          <Card className="hover:border-[#83A2DB]/40 transition-all hover:-translate-y-0.5">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                    <Scan className="h-4 w-4" />
                  </div>
                  <CardTitle className="text-base font-bold text-[var(--ink-primary)]">
                    Gate QR Scanner
                  </CardTitle>
                </div>
                <Badge variant="blue" size="sm">
                  Check-In Active
                </Badge>
              </div>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
                Scan attendee QR codes via live camera or barcode scanner with instant duplicate prevention.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
                <QrCode className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Camera & USB handheld barcode wedge support</span>
              </div>
              <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
                <ShieldCheck className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Real-time duplicate scan conflict detection</span>
              </div>
              <div className="pt-2">
                <Link href="/dashboard/tickets">
                  <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                    <span>Open Scanner & Attendees</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </Link>
              </div>
            </CardContent>
          </Card>

          {/* Teams Directory Card */}
          <Card className="hover:border-[#83A2DB]/40 transition-all hover:-translate-y-0.5">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                    <Layers className="h-4 w-4" />
                  </div>
                  <CardTitle className="text-base font-bold text-[var(--ink-primary)]">
                    Teams & Squads
                  </CardTitle>
                </div>
                <Badge variant="subtle" size="sm">
                  Volunteer Access
                </Badge>
              </div>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
                Lookup registered squads, verify participant rosters, and locate designated team leads during staging.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
                <Users className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Inspect full member lists, emails, and phone numbers</span>
              </div>
              <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
                <ShieldCheck className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Real-time check-in and account setup verification</span>
              </div>
              <div className="pt-2">
                <Link href="/dashboard/teams">
                  <Button variant="outline" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                    <span>Open Teams Directory</span>
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
