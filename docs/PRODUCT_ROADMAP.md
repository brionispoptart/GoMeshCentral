# GoMeshCentral Product Roadmap

This roadmap is designed for predictable execution and easy human handoff at any stage.

## Guiding Principles

- Keep vertical slices deployable at each milestone.
- Prefer observable, testable interfaces over hidden coupling.
- Keep backend and frontend container-ready through env-based config.
- Record architecture decisions as they are made.

## Phase 0: Current Baseline (Done)

- Go server with auth and ACL.
- SQLite persistence for users and devices.
- React web UI (Vite + Tailwind + shadcn-style patterns).
- Agent websocket heartbeat and command channel.

## Phase 1: Agent Enrollment and Persistence (In Progress)

Goal: move from demo agent process to persistent endpoint identity and robust connectivity.

Deliverables:

- Persistent local agent identity file.
- Automatic reconnect loop with exponential backoff.
- Heartbeat/session resilience after transient failures.
- Initial enrollment flow contract (token/device claim).

Exit criteria:

- Agent survives server restarts without manual intervention.
- Agent retains stable device identity across process restarts.
- Dashboard always reflects reconnect/disconnect transitions.

## Phase 1b: Installed Agent Runtime and Elevated Execution (In Progress)

Goal: match how commercial RMM agents (MeshCentral, Atera) install and run, so remote actions execute reliably in an always-on, elevated context.

Design notes:

- Agent connects outbound to the server over a persistent WebSocket (reverse connection), so no inbound firewall/NAT rules are required on the endpoint.
- Agent installs as a background OS service that starts at boot and auto-reconnects.
- On Windows the service runs as LocalSystem (permanently elevated); on Linux it runs as a systemd unit under root.
- Interactive/portable runs keep a self-elevation fallback for quick testing.

Deliverables:

- Windows service install/uninstall/run lifecycle (LocalSystem, automatic start, failure restart).
- Linux systemd install flow (root service, auto-restart) documented via existing unit template.
- Service-mode headless runtime shared with the interactive runtime.
- Documented production install flow using a built binary (not `go run`).

Remote access strategy:

- Near-term focus is the shell/terminal relay (ConPTY on Windows, PTY on Linux) — the current "SSH-style" remote shell.
- Keep the terminal/session model transport-agnostic so future remote desktop paths (RDP/VNC) and third-party integrations (e.g. a RustDesk-style bring-your-own client) can attach without reworking the agent connection layer.

Exit criteria:

- Agent can be installed as an always-on elevated service and reconnects across reboots.
- Terminal sessions run elevated by default when served by the installed service.
- No architectural blockers introduced for later desktop/relay transports or third-party client integrations.

## Phase 1c: PSA / Business Operations (Atera Parity) (In Progress)

Goal: cover the business-operations half of an RMM+PSA platform (Atera-style), not just remote management.

Deliverables:

- Client (organization) model; devices can be assigned/sorted/filtered by client.
- Contracts: service agreement per client with rate type (hourly/monthly/fixed/per-device) and billing cycle.
- Ticketing: helpdesk tickets linked to client/device, status + priority lifecycle, assignee.
- Billing/Invoicing: itemized invoices per client with computed subtotal/tax/total and status lifecycle (draft/sent/paid/void).
- New `psa:view` / `psa:manage` permissions (admin + operator manage; viewer view-only).

Exit criteria:

- Devices page supports client assignment, filtering, and sorting.
- Clients/Contracts/Tickets/Billing pages are functional end-to-end (create + list) against the live API.
- Server runs on Linux, clients (agents) run on Linux and Windows, with no PSA feature gated behind OS.

Not yet implemented (follow-up):

- Payment gateway integration (Stripe/etc.) — invoices currently track status manually, no processor charge.
- Client portal (customer-facing login) and email delivery of invoices/tickets.

## Phase 1f: PSA Billing Automation (Done)

Goal: reduce manual invoicing work — time entries roll into invoices automatically, recurring contracts bill themselves on schedule, and overdue invoices are flagged without operator intervention.

Deliverables:

- Time Entry model: billable/non-billable time logged against a client (optionally a ticket), tracked as invoiced/uninvoiced.
- `internal/billing` package (storage-only, no HTTP/hub dependency so it's callable from both a manual API trigger and the background scheduler): `GenerateInvoiceForContract` builds an invoice from a contract's recurring rate plus all unbilled billable time entries for that client, marks those entries invoiced, and stamps the contract's last-invoiced date.
- Background scheduler in `cmd/server` runs hourly (plus once at startup): auto-generates invoices for any active contract whose billing cycle (monthly/quarterly/annual) has elapsed since it was last invoiced, and flips `sent` invoices past their due date to `overdue`.
- Manual "Generate Invoice Now" button per contract (`POST /api/contracts/{id}/generate-invoice`) bypasses the due-date check for on-demand billing/testing.

Exit criteria:

- Unit-tested: due-date calculation per billing cycle, invoice generation rolling in time entries and marking them billed, forced vs. scheduled generation, and overdue-flagging (`internal/billing/billing_test.go`, all passing).
- Verified live against the API: contract + time entry + manual invoice generation produced a correct itemized total ($300 base + 1.5h × $300 = $750), contract `lastInvoicedAt` updated, and the time entry was marked invoiced.

Not yet implemented (follow-up):

- Payment gateway integration (Stripe/etc.).
- Email delivery of generated invoices.
- Per-time-entry custom billing rate (currently always uses the contract's rate amount as the hourly rate).

## Phase 1d: Device Grouping and Script Library (Done)

Goal: give operators day-to-day RMM organization/automation tools beyond raw device list.

Deliverables:

- Device Group model; devices can be assigned/filtered/sorted by group independent of client.
- Script library: saved reusable commands/scripts (Windows PowerShell / Linux shell / any), run on demand against any connected device via the existing agent command channel.

Exit criteria:

- Devices page supports group assignment, filtering, and sorting alongside client filtering.
- Script run dispatches over the existing hub command channel and records an audit event; offline devices return a clear conflict response.

## Phase 1e: Alerts (Threshold Rules) (Done)

Goal: proactive notification when a device breaches a CPU/memory threshold or goes offline, without requiring an operator to be watching the dashboard.

Deliverables:

- AlertRule model: metric (cpu/memory/offline), comparator (above/below), threshold, severity, optional device/customer scope.
- Alert model: opened automatically by the hub when a rule is breached on report ingestion (cpu/memory) or on agent disconnect (offline); auto-resolves when the metric recovers or the device reconnects; deduplicates so a sustained breach only opens one alert.
- Alerts page: manage rules, filter/acknowledge/resolve triggered alerts.
- Notification bell in the top bar shows a live open-alert count and links to the Alerts page.

Exit criteria:

- Unit-tested trigger/dedupe/auto-resolve behavior for both metric and offline rules (internal/hub/alerts_test.go).
- Alert rule and alert CRUD verified against the live API.

## Phase 1g: AI Operations Assistant (Done)

Goal: let operators query system-wide operational context and complete controlled tasks through OpenRouter or a local Hermes-compatible runtime.

Deliverables:

- Provider settings for OpenRouter and OpenAI-compatible Hermes/local endpoints with redacted API-key responses.
- Client-scoped assistant context covering devices, alerts, PSA clients, contracts, tickets, invoices, and time entries.
- Explicit approval before ticket creation, device command dispatch, or alert acknowledgement; existing permissions and client ownership checks remain authoritative.

Exit criteria:

- Fake-provider contract and API-key preservation/redaction tests pass.
- Assistant UI supports chat, proposed-action review, approval, and affected dashboard refreshes.

## Phase 2: Security Hardening

Goal: production-safe trust and authentication model.

Deliverables:

- Replace custom token format with standards-based JWT/OIDC or signed enrollment credentials.
- Password policy and reset workflows.
- TLS enforcement, cert pinning options, and secret rotation support.
- Audit logs for authn/authz operations.

Exit criteria:

- No plaintext secrets in runtime defaults for production profile.
- Auth endpoints and agent handshake covered by automated tests.

## Phase 3: Multi-Tenancy and Domain Model

Goal: represent organizations, users, devices, and permissions with clear boundaries.

Deliverables:

- Organization model and tenant scoping.
- Device groups and scoped permissions.
- User-role assignments per org.
- Migration-safe schema versioning.

Exit criteria:

- Cross-tenant isolation proven by tests.
- Admin UI supports org and role assignment flows.

## Phase 4: Remote Operations Core

Goal: establish practical remote management value.

Deliverables:

- [x] Remote terminal streaming channel (ConPTY on Windows, real PTY on Linux).
- [x] File transfer module (browse/list, upload, download over the existing agent websocket channel; 100MB cap per transfer, no chunked progress reporting yet).
- [x] Command history and result capture (audit events for command dispatch, file download/upload).

Exit criteria:

- [x] Operator can execute remote commands and transfer files reliably (verified end-to-end against a live connected agent: directory listing, file download, and file upload all round-tripped correctly).
- [x] Access controls enforced per action type (devices:command required for file browse/upload/download and script run; devices:view for read-only lists).

Follow-ups not yet implemented: upload/download progress percentage in the UI (current implementation is all-or-nothing per request), files above 100MB, and resumable transfers.

## Phase 5: Desktop Stream and Control

Goal: interactive remote desktop capability.

Deliverables:

- Video/frame transport channel.
- Input control channel (mouse/keyboard).
- Session recording and policy hooks.

Exit criteria:

- Interactive control is usable at acceptable latency on LAN and WAN.
- Session authorization and audit trails in place.

## Phase 6: Relay, NAT Traversal, and Scale

Goal: resilient connectivity and horizontal scalability.

Deliverables:

- Relay service and fallback routing.
- NAT traversal strategy and tuning.
- Connection broker and sharding plan.
- Rate limits, quotas, and abuse controls.

Exit criteria:

- Multiple server instances handle active sessions with stable performance.
- Failover strategy documented and tested.

## Phase 7: Packaging and Operations

Goal: operational readiness and deployment consistency.

Deliverables:

- Dockerfiles and compose setup for local and production-like deployment.
- CI pipelines for lint/test/build/package.
- Structured telemetry, metrics, dashboards, and alerts.
- Backup/restore strategy for DB and config.

Exit criteria:

- One-command local deployment and repeatable CI release artifacts.
- Runbook exists for on-call and incident response.

## Phase 8: Release Readiness

Goal: stable, supportable product release.

Deliverables:

- Versioned upgrade/migration path.
- Documentation for install, admin, and security posture.
- Release checklist and rollback procedure.

Exit criteria:

- Green test gates, signed artifacts, and documented rollback.
- External reviewer can deploy from docs only.
