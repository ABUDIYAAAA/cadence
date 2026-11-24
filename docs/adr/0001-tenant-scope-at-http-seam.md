# Resolve Event and Competition Scope at the HTTP Seam

We keep frontend routes flat (`/events` for event selection and `/dashboard/*` with a header Competition Switcher) and inject `/e/{eventSlug}` and `/c/{competitionSlug}` prefixes inside the HTTP client's request interceptor rather than nesting Next.js routes or passing slug parameters through every service and React Query hook. This hides multi-tenant path routing behind a single deep seam so existing feature views (Teams, Judging, Tickets, Notifications, Audit) operate without prop-drilling.
