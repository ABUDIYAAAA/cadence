"use client";

import React from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { RoleDashboard } from "@/components/dashboard/role-dashboard";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export default function AdminDashboardPage() {
  return (
    <RoleDashboard
      expectedRole="admin"
      title="Admin overview"
      description="Manage hackathon operations, users, scoring rounds, and system status."
    >
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 font-sans">
        {/* Broadcast Notifications Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <CardTitle>Broadcasts</CardTitle>
            <CardDescription>
              Send announcements, emergency alerts, and room calls via Web Push and in-app feeds.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div>Target by role, team, or all attendees</div>
              <div>Delivery tracking and read receipts</div>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/admin/notifications">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Manage broadcasts</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Users Management Quick Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <CardTitle>Users</CardTitle>
            <CardDescription>
              Manage accounts, assign roles, link teams, and track account setup status.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div>CSV import and manual account creation</div>
              <div>Password setup and reset email dispatch</div>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/users">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Manage users</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Audit Trail Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <CardTitle>Audit log</CardTitle>
            <CardDescription>
              Activity history for logins, ticket scans, score submissions, and broadcasts.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div>CSV streaming export for compliance</div>
              <div>IP, user-agent, and actor tracking</div>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/audit">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>View audit log</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Teams Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <CardTitle>Teams</CardTitle>
            <CardDescription>
              Organize hackathon squads, manage participant allocations, and designate team leads.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div>Roster management and lead promotion</div>
              <div>Safe member detachment on team disbanding</div>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/teams">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Manage teams</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Ticketing & Gate Operations Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <CardTitle>Tickets & check-in</CardTitle>
            <CardDescription>
              QR pass issuance, bulk dispatch with domain filtering, camera scanner, and attendance stats.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div>Bulk QR dispatch with email domain exclusion</div>
              <div>Instant camera scanner with duplicate check</div>
            </div>
            <div className="pt-2">
              <Link href="/dashboard/tickets">
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>Manage tickets</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Security & Access Policy Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Session security</CardTitle>
              <Badge variant="emerald" dot size="sm">
                Enforced
              </Badge>
            </div>
            <CardDescription>
              Distributed JWT session enforcement backed by Redis blocklists for immediate revocation.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Session duration:</span>
                <span className="font-semibold text-[var(--ink-primary)]">15 minutes</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Refresh lifetime:</span>
                <span className="font-semibold text-[var(--ink-primary)]">7 days</span>
              </div>
              <div className="flex justify-between py-1.5">
                <span>Revocation:</span>
                <span className="font-semibold text-[var(--ink-primary)]">&lt;1 ms (Redis)</span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Telemetry Card */}
        <Card className="hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>System metrics</CardTitle>
              <Badge variant="emerald" dot size="sm">
                Live
              </Badge>
            </div>
            <CardDescription>
              Prometheus metrics dashboard tracking request rate, latencies, auth events, and database pool.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="text-xs text-[var(--ink-secondary)] space-y-1">
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Scrape interval:</span>
                <span className="font-semibold text-[var(--ink-primary)]">5 seconds</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-[var(--card-line)]">
                <span>Metrics:</span>
                <span className="font-semibold text-[var(--ink-primary)]">RED, latency, DB pool</span>
              </div>
            </div>
            <div className="pt-2">
              <a
                href="http://localhost:3001/d/cadence-overview/cadence-hackathon-operations-and-telemetry"
                target="_blank"
                rel="noopener noreferrer"
              >
                <Button variant="primary" size="sm" className="w-full text-xs gap-1.5 h-9 font-semibold">
                  <span>View metrics</span>
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
