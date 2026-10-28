"use client";

import React from "react";
import Link from "next/link";
import { Layers, Users, ArrowRight, ShieldCheck, QrCode, Scan } from "lucide-react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { VolunteerScheduleView } from "@/components/judging";

export default function VolunteerDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="volunteer"
      title="Volunteer Coordination Center"
      description="Queue status coordination, team staging schedules, and gate check-in scanner controls."
    >
      <div className="space-y-8">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Gate Scanner Card */}
          <Card className="hover:border-black transition-colors">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Scan className="h-5 w-5 text-black" />
                  <CardTitle className="text-sm">Gate QR Scanner</CardTitle>
                </div>
                <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                  Check-In Active
                </span>
              </div>
              <CardDescription>
                Scan attendee QR codes via live camera or barcode scanner with instant duplicate prevention.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-2 text-xs text-neutral-600">
                <QrCode className="h-3.5 w-3.5" />
                <span>Camera & USB handheld barcode wedge support</span>
              </div>
              <div className="flex items-center gap-2 text-xs text-neutral-600">
                <ShieldCheck className="h-3.5 w-3.5" />
                <span>Real-time duplicate scan conflict detection</span>
              </div>
              <div className="pt-2">
                <Link href="/dashboard/tickets">
                  <Button variant="primary" size="sm" className="w-full text-xs gap-1.5">
                    <span>Open Scanner & Attendees</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </Link>
              </div>
            </CardContent>
          </Card>

          {/* Teams Directory Card */}
          <Card className="hover:border-black transition-colors">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Layers className="h-5 w-5 text-black" />
                  <CardTitle className="text-sm">Teams & Squads</CardTitle>
                </div>
                <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                  Volunteer Access
                </span>
              </div>
              <CardDescription>
                Lookup registered squads, verify participant rosters, and locate designated team leads during staging.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-2 text-xs text-neutral-600">
                <Users className="h-3.5 w-3.5" />
                <span>Inspect full member lists, emails, and phone numbers</span>
              </div>
              <div className="flex items-center gap-2 text-xs text-neutral-600">
                <ShieldCheck className="h-3.5 w-3.5" />
                <span>Real-time check-in and account setup verification</span>
              </div>
              <div className="pt-2">
                <Link href="/dashboard/teams">
                  <Button variant="outline" size="sm" className="w-full text-xs gap-1.5">
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
