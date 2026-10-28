"use client";

import React from "react";
import Link from "next/link";
import { Users, ArrowRight, ShieldCheck, Mail, UserPlus, Activity, FileSpreadsheet, Bell, Send, Radio, Layers, Ticket, QrCode, Scan } from "lucide-react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export default function AdminDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="admin"
      title="Administrator Operations"
      description="Full administrative access to manage events, participants, scoring rounds, panels, and tickets."
    >
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 font-sans">
        {/* Push & Broadcast Notifications Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Bell className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">Push & Broadcast</CardTitle>
              </div>
              <Badge variant="blue" size="sm">
                Admin
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Publish real-time announcements, emergency alerts, and room calls via RabbitMQ Web Push and in-app feeds.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <Radio className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Target: Broadcast, Role, Team, or Direct User</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <Send className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Delivery tracking & read receipts history</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/admin/notifications">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Open Notification Console</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Users Management Quick Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Users className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">User Directory</CardTitle>
              </div>
              <Badge variant="blue" size="sm">
                Admin
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Manage accounts, assign roles (Judges, Volunteers, Participants), link teams, and track onboarding status.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <UserPlus className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Bulk CSV imports & single-user creation</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <Mail className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Password setup & token reset dispatch</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/users">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Open Users Directory</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Audit Trail Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Activity className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">Security Audit Trail</CardTitle>
              </div>
              <Badge variant="blue" size="sm">
                Admin
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Immutable historical logs of user actions, auth attempts, ticket scans, score submissions, and broadcast notifications.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <FileSpreadsheet className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>CSV streaming export for forensic auditing</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <ShieldCheck className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>IP, User-Agent, and device correlation</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/audit">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>View Audit Trail</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Squads & Teams Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Layers className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">Teams & Squads</CardTitle>
              </div>
              <Badge variant="blue" size="sm">
                Admin
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Organize hackathon squads, manage participant allocations, designate leaders, and inspect team rosters.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <Users className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Full roster member management & leader promotion</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <ShieldCheck className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Safe member detachment upon squad dissolution</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/teams">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Open Teams Manager</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Ticketing & Gate Operations Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Ticket className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">Tickets & Gate Check-In</CardTitle>
              </div>
              <Badge variant="blue" size="sm">
                Admin
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Bulk QR pass issuance, domain-filtered distribution, live camera scanner, and real-time attendance analytics.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <QrCode className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Multi-select QR dispatch with excluded domain filtering</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-[var(--ink-secondary)]">
              <Scan className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Instant camera scanning & duplicate detection</span>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/tickets">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Open Ticketing Console</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Security & Access Policy Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <ShieldCheck className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">Access Control Policy</CardTitle>
              </div>
              <Badge variant="subtle" size="sm">
                Active
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Distributed JWT session enforcement backed by high-speed Redis blocklists for instantaneous account revocation (&lt;1ms).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Session Duration:</span>
                <span className="font-bold font-mono text-[var(--ink-primary)]">15 Minutes (Sliding)</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Refresh Lifetime:</span>
                <span className="font-bold font-mono text-[var(--ink-primary)]">7 Days (HTTP-only)</span>
              </div>
              <div className="flex justify-between py-1.5">
                <span>Revocation Latency:</span>
                <span className="font-bold font-mono text-[var(--ink-primary)]">&lt;1 ms (Redis Cluster)</span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Observability & Grafana Telemetry Card */}
        <Card className="hover:border-[var(--color-blue)]/50 transition-all hover:-translate-y-0.5">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
                  <Activity className="h-4 w-4" />
                </div>
                <CardTitle className="text-base font-bold text-[var(--ink-primary)]">Grafana Telemetry</CardTitle>
              </div>
              <Badge variant="blue" size="sm">
                Port 3001
              </Badge>
            </div>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1.5">
              Live Prometheus metrics dashboard tracking RED requests, P95 latencies, auth events, rate limits, and PG pool connections.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Prometheus Scrape:</span>
                <span className="font-bold font-mono text-[var(--ink-primary)]">Every 5 Seconds</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Metrics Tracked:</span>
                <span className="font-bold font-mono text-[var(--ink-primary)]">RED, Security, PG Pool, Go GC</span>
              </div>
            </div>
            <div className="pt-2">
              <a
                href="http://localhost:3001/d/cadence-overview/cadence-hackathon-operations-and-telemetry"
                target="_blank"
                rel="noopener noreferrer"
              >
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Open Grafana Dashboard</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </a>
            </div>
          </CardContent>
        </Card>
      </div>
    </RoleDashboard>
  );
}
