"use client";

import React from "react";
import Link from "next/link";
import { Users, ArrowRight, ShieldCheck, Mail, UserPlus, Activity, FileSpreadsheet, Bell, Send, Radio, Layers, Ticket, QrCode, Scan } from "lucide-react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export default function AdminDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="admin"
      title="Administrator Operations"
      description="Full administrative access to manage events, participants, scoring rounds, panels, and tickets."
    >
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {/* Push & Broadcast Notifications Card */}
        <Card className="hover:border-black transition-colors">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Bell className="h-5 w-5 text-black" />
                <CardTitle className="text-sm">Push & Broadcast</CardTitle>
              </div>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                Admin
              </span>
            </div>
            <CardDescription>
              Publish real-time announcements, emergency alerts, and room calls via RabbitMQ Web Push and in-app feeds.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <Radio className="h-3.5 w-3.5" />
              <span>Target: Broadcast, Role, Team, or Direct User</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <Send className="h-3.5 w-3.5" />
              <span>Delivery tracking & read receipts history</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/admin/notifications">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5">
                  <span>Open Notification Console</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Users Management Quick Card */}
        <Card className="hover:border-black transition-colors">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Users className="h-5 w-5 text-black" />
                <CardTitle className="text-sm">User Directory</CardTitle>
              </div>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                Admin
              </span>
            </div>
            <CardDescription>
              Manage accounts, assign roles (Judges, Volunteers, Participants), link teams, and track onboarding status.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <UserPlus className="h-3.5 w-3.5" />
              <span>Bulk CSV imports & single-user creation</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <Mail className="h-3.5 w-3.5" />
              <span>Password setup & token reset dispatch</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/users">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5">
                  <span>Open Users Directory</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Audit Trail Card */}
        <Card className="hover:border-black transition-colors">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Activity className="h-5 w-5 text-black" />
                <CardTitle className="text-sm">Security Audit Trail</CardTitle>
              </div>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                Admin
              </span>
            </div>
            <CardDescription>
              Immutable historical logs of user actions, auth attempts, ticket scans, score submissions, and broadcast notifications.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <FileSpreadsheet className="h-3.5 w-3.5" />
              <span>CSV streaming export for forensic auditing</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <ShieldCheck className="h-3.5 w-3.5" />
              <span>IP, User-Agent, and device correlation</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/audit">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5">
                  <span>View Audit Trail</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Squads & Teams Card */}
        <Card className="hover:border-black transition-colors">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Layers className="h-5 w-5 text-black" />
                <CardTitle className="text-sm">Teams & Squads</CardTitle>
              </div>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                Admin
              </span>
            </div>
            <CardDescription>
              Organize hackathon squads, manage participant allocations, designate leaders, and inspect team rosters.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <Users className="h-3.5 w-3.5" />
              <span>Full roster member management & leader promotion</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <ShieldCheck className="h-3.5 w-3.5" />
              <span>Safe member detachment upon squad dissolution</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/teams">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5">
                  <span>Open Teams Manager</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Ticketing & Gate Operations Card */}
        <Card className="hover:border-black transition-colors">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Ticket className="h-5 w-5 text-black" />
                <CardTitle className="text-sm">Tickets & Gate Check-In</CardTitle>
              </div>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                Admin
              </span>
            </div>
            <CardDescription>
              Bulk QR pass issuance, domain-filtered distribution, live camera scanner, and real-time attendance analytics.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <QrCode className="h-3.5 w-3.5" />
              <span>Multi-select QR dispatch with excluded domain filtering</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-neutral-600">
              <Scan className="h-3.5 w-3.5" />
              <span>Instant camera scanning & duplicate detection</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/tickets">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5">
                  <span>Open Ticketing Console</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Security & Access Policy Card */}
        <Card className="hover:border-black transition-colors">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <ShieldCheck className="h-5 w-5 text-black" />
                <CardTitle className="text-sm">Access Control Policy</CardTitle>
              </div>
              <span className="text-[10px] border border-black px-2 py-0.5 uppercase tracking-wider font-bold">
                Active
              </span>
            </div>
            <CardDescription>
              Distributed JWT session enforcement backed by high-speed Redis blocklists for instantaneous account revocation (&lt;1ms).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-neutral-600 space-y-1">
              <div className="flex justify-between py-1 border-b border-neutral-100">
                <span>Session Duration:</span>
                <span className="font-bold text-black">15 Minutes (Sliding)</span>
              </div>
              <div className="flex justify-between py-1 border-b border-neutral-100">
                <span>Refresh Lifetime:</span>
                <span className="font-bold text-black">7 Days (HTTP-only)</span>
              </div>
              <div className="flex justify-between py-1">
                <span>Revocation Latency:</span>
                <span className="font-bold text-black">&lt;1 ms (Redis Cluster)</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </RoleDashboard>
  );
}
