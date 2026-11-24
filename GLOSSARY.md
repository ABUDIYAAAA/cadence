# Cadence

Cadence is a multi-tenant event and hackathon operations platform covering attendee check-in, team formation, live judging panels, and real-time notifications.

## Identity & Tenancy

**User**:
A person authenticated via Google OAuth who holds a single platform account across all events.
_Avoid_: Account, profile, member

**Global Role**:
A platform-wide privilege level on a User (`dev` or `user`) governing who can provision or delete Events and manage platform-wide Users.
_Avoid_: System role, superadmin

**Blacklist**:
A platform-wide block applied by a `dev` that revokes a User's sessions and prevents them from signing in anywhere on Cadence.
_Avoid_: Deactivate, global ban

**Event**:
A top-level tenant (identified by an `eventSlug`, such as a hackathon weekend or college fest) that owns its roster of Event Members, Tickets, Scan Sessions, Notifications, Audit Logs, and one or more Competitions.
_Avoid_: Organization, workspace, tenant, hackathon

**Event Member**:
A User enrolled in a specific Event with a single Event Role and an event-scoped ban status.
_Avoid_: Event user, participant (when referring to all roles)

**Event Role**:
The role an Event Member holds within one Event (`owner`, `admin`, `volunteer`, `judge`, or `participant`), which applies across every Competition inside that Event.
_Avoid_: Competition role, user role

**Event Ban**:
A restriction applied by an Event `owner` or `admin` that blocks an Event Member from accessing that specific Event and its Competitions without affecting their User account in other Events.
_Avoid_: Blacklist, kick, remove

**Competition**:
A competitive track encapsulated within an Event (identified by a `competitionSlug` unique within its parent Event) that scopes Teams and Judging Rounds.
_Avoid_: Sub-event, challenge, track

## Teams & Submissions

**Team**:
A group of Participants competing together within a single Competition, led by one Team Leader and joined via a shareable join code.
_Avoid_: Group, squad

**Team Leader**:
The Participant on a Team who can rename the Team, regenerate its join code, remove teammates, or transfer leadership to another teammate.
_Avoid_: Captain, owner

**Team Freeze**:
A lock (applied per Team or across all Teams in a Competition) that prevents Participants from joining, leaving, or modifying Team rosters unless overridden by staff.
_Avoid_: Roster lock

**Submission**:
The project artifacts (repository, demo, presentation links, and notes) submitted by a Team for evaluation, optionally gated by attendance in a Scan Session.
_Avoid_: Project, entry

## Judging & Scoring

**Round**:
A distinct phase of evaluation within a Competition with its own rubric of Criteria, default pitch duration, and set of Panels.
_Avoid_: Stage, phase

**Panel**:
A judging room within a Round staffed by assigned Judges and Volunteers that evaluates an ordered queue of Slots.
_Avoid_: Room (when referring to the group of judges), committee, board

**Slot**:
A scheduled evaluation session assigning one Team to one Panel in a Round, tracking its live pitch timer, queue position, and completion status.
_Avoid_: Pitch, session, turn

**Criterion**:
A rubric dimension defined on a Round (with minimum score, maximum score, and weight) that every Judge on a Panel marks for each Team.
_Avoid_: Metric, category, question

**Custom Criterion**:
A per-Team bonus or track-specific rubric dimension in a Round whose marks are awarded directly by an `admin` rather than scored by Judges.
_Avoid_: Bonus points, extra credit, manual score

**Scorecard**:
A single Judge's set of marks and notes for one Team's Slot across all Criteria in a Round, progressing from draft to locked (and counted on the leaderboard) unless unlocked by the Judge or an `admin`.
_Avoid_: Evaluation sheet, ballot, review

## Check-in & Operations

**Ticket**:
An Event-scoped QR pass issued to an Event Member used to verify arrival at the venue and record presence in Scan Sessions.
_Avoid_: Pass, badge, invite

**Scan Session**:
An Event-scoped attendance or exit checkpoint (e.g., "Day 1 Check-in", "Midnight Roll Call", "Lunch") where Volunteers scan Tickets to track presence or gate Submissions.
_Avoid_: Checkpoint, roll call, event
