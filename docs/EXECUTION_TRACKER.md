# GoMeshCentral Execution Tracker

Use this file as the authoritative handoff artifact.

## Current Milestone

- Phase: 1
- Name: Agent Enrollment and Persistence
- Status: In Progress

## Completed

- [x] Baseline server, agent, ACL, SQLite, and React UI
- [x] Role-based UI controls and admin user management
- [x] Backend serves built SPA

## In Progress

- [x] Persist agent identity on disk
- [x] Agent reconnect with exponential backoff
- [x] Enrollment contract draft (server and agent)
- [x] Agent credential rotation support in runtime (manual + periodic)
- [x] Admin enrollment UI supports per-device credential rotation and audit visibility
- [x] Agent reports pipeline (agent -> API -> dashboard) with unenrolled recovery guidance
- [x] Device metrics history and graphs (CPU/memory trends in Reports)

## Next Implementation Slice

- [x] Server-issued enrollment token API
- [x] Agent enroll endpoint and token redemption
- [x] Persist agent-issued credential (replace shared secret usage)
- [x] Add admin UI workflow to mint enrollment tokens
- [x] Add agent credential rotation endpoint
- [x] Remove legacy shared-secret fallback path

## Installed Agent Runtime (Phase 1b)

- [x] Windows service install/uninstall/run lifecycle (LocalSystem, auto-start, failure restart)
- [x] Shared headless runtime used by both service and interactive modes
- [x] Verify terminal relay works under the LocalSystem service (session 0 spawning) — validated end-to-end via the tray "Unattended Access" toggle; ConPTY does not require an interactive desktop
- [x] Linux systemd install helper wired to the existing unit template (`-install-service`/`-uninstall-service` in cmd/agent/service_linux.go; `packaging/linux/install.sh` one-line installer)
- [x] Linux terminal upgraded from plain stdio pipes to a real PTY (github.com/creack/pty), matching ConPTY on Windows: correct TERM/job-control semantics, live resize, full-screen apps (vim/top/less) all work, including headless under systemd
- [ ] Keep session/terminal model transport-agnostic for future RDP/VNC and third-party (RustDesk-style) clients

## Upcoming

- [ ] Security hardening plan and implementation (Phase 2)
- [ ] Multi-tenant domain model (Phase 3)
- [ ] Remote terminal and file transfer (Phase 4)
- [ ] Third-party remote-desktop client integration hooks (future)

## Risks

- Current token model is not production grade.
- No relay/NAT traversal support yet.

## Handoff Notes

- If taking over now, start in cmd/agent and internal/httpapi enrollment/auth paths.
- Validate changes with: go test ./..., frontend build, manual reconnect test.
- Keep API contracts backward compatible where possible.
