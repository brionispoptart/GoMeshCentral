# GoMeshCentral

GoMeshCentral is a MeshCentral-inspired MVP written in Go. This repository includes:

- HTTP API server for auth, ACL, user management, and device management
- WebSocket hub for device heartbeats and command dispatch
- Sample Go agent that connects to the server and receives commands
- React + Vite web dashboard using shadcn-style component patterns
- SQLite-backed persistence for users and devices
- Permanent dark mode UI with route-based app shell and milestone page scaffolding

## Architecture

- cmd/server: starts API + WebSocket server
- cmd/agent: sample endpoint agent
- internal/auth: token issuing and validation
- internal/authz: role and permission policy
- internal/storage: storage interface + SQLite implementation
- internal/hub: agent and dashboard websocket coordination
- internal/httpapi: REST and websocket handlers
- web: Vite + React frontend

Web UI route map (implemented + scaffolded):

- /overview (implemented)
- /work-queue (implemented)
- /devices (implemented)
- /users (implemented)
- /enrollment (implemented)
- /events (implemented)
- /reports (implemented)
- /assistant (implemented; OpenRouter/Hermes-compatible with approved actions)
- /device-groups (scaffold)
- /alerts (scaffold)
- /terminal (implemented, Linux + Windows PowerShell MVP)
- /files (scaffold)
- /desktop (scaffold)
- /scripts (scaffold)
- /policies (scaffold)
- /relay (scaffold)
- /amt (scaffold)
- /integrations (scaffold)
- /audit (scaffold)
- /settings (implemented)

Execution artifacts:

- docs/PRODUCT_ROADMAP.md: end-to-end build plan from MVP to production readiness
- docs/EXECUTION_TRACKER.md: current milestone state and handoff checklist

## Environment Variables

- GMC_LISTEN_ADDR (default :8080)
- GMC_AGENT_PUBLIC_ADDR (optional; example mesh.example.com:8080; used in generated agent install commands)
- GMC_JWT_SECRET (default dev-secret-change-me)
- GMC_DB_PATH (default data/gomeshcentral.db)
- GMC_BOOTSTRAP_ADMIN_USER (default admin)
- GMC_BOOTSTRAP_ADMIN_PASS (default admin123!)

Frontend environment variables (for future Docker split deployment):

- VITE_API_BASE_URL (default empty; same-origin API calls)

## Quick Start

1. Start server
   - make run-server
2. Build frontend once
   - make web-build
3. In another terminal, start agent
   - make run-agent
4. Open http://localhost:8080
5. Login with bootstrap admin credentials (admin / admin123! by default)
6. Confirm device appears and send ping command
7. As admin, create operator/viewer users from the dashboard

Frontend development (hot reload):

- make web-dev

## Linux VM Agent Install (Headless)

Use this flow to test an agent on a Linux VM without tray dependencies.

1. Build Linux agent binary from your development machine
   - GOOS=linux GOARCH=amd64 go build -o dist/gomesh-agent-linux-amd64 ./cmd/agent
2. Copy binary to VM
   - scp dist/gomesh-agent-linux-amd64 user@<vm-ip>:/opt/gomeshcentral/gomesh-agent
3. On VM, make executable
   - chmod +x /opt/gomeshcentral/gomesh-agent
4. In dashboard, create enrollment token (Enrollment page)
5. Start agent on VM
   - /opt/gomeshcentral/gomesh-agent -server <server-ip>:8080 -state /var/lib/gomeshcentral/agent-state.json -name "linux-vm-01" -enroll-token <token>
6. Verify device heartbeat and reports in dashboard

Optional systemd service:

- Use docs/gomesh-agent.service as a template and replace SERVER_IP and ENROLLMENT_TOKEN.
- Copy template to /etc/systemd/system/gomesh-agent.service.
- Then run: systemctl daemon-reload && systemctl enable --now gomesh-agent

## Windows Agent Install (Elevated Service)

Run the agent as an always-on Windows service that starts at boot and runs
permanently elevated as LocalSystem. This mirrors how commercial RMM agents
(MeshCentral, Atera) keep a persistent, privileged control channel open.

1. Build the agent binary (do not use `go run` for the service; install a real exe)
   - go build -o dist\gomesh-agent.exe .\cmd\agent
2. Open an elevated (Administrator) PowerShell
3. Install and start the service (LocalSystem, auto-start, auto-restart on failure)
   - .\dist\gomesh-agent.exe -install-service -server <server-ip>:8080 -state C:\ProgramData\GoMeshCentral\agent-state.json -enroll-token <token>
4. Verify the service is running
   - Get-Service GoMeshCentralAgent
5. Uninstall when needed (elevated shell)
   - .\dist\gomesh-agent.exe -uninstall-service

Notes:

- The service runs as LocalSystem, so it is always elevated — no per-launch UAC prompt.
- The installed command line (server, state path, enrollment token, intervals) is baked
  into the service and reused on every automatic start and reconnect.
- For interactive/portable testing you can still run the agent directly; on Windows it
  self-elevates via a UAC relaunch when not already an administrator.

## Roles and ACL

- admin: devices:view, devices:command, users:manage
- operator: devices:view, devices:command
- viewer: devices:view

Admin-only APIs:

- GET /api/users
- POST /api/users
- POST /api/enrollment-tokens
- POST /api/agents/admin-rotate-key
- GET /api/audit-events

Remote terminal APIs:

- POST /api/devices/{deviceId}/terminal/sessions
- GET /ws/terminal?session_id={sessionId}&token={jwt}

Agent command execution:

- Device commands are executed on the agent host shell.
- Windows agents execute commands with PowerShell.
- Command results are recorded as audit events with action agent_command_result.

Agent enrollment API:

- POST /api/agents/enroll
- POST /api/agents/rotate-key

Enrollment bootstrap API:

- GET /api/enrollment-bootstrap (admin only)
- POST /api/enrollment-tokens now returns preconfigured install commands using GMC_AGENT_PUBLIC_ADDR (or request host fallback)

Runtime settings API:

- GET /api/settings/agent-endpoint (admin only)
- PUT /api/settings/agent-endpoint (admin only)
- GET /api/settings/application (admin only; AI keys are redacted)
- PUT /api/settings/application (admin only; a blank AI key retains the stored key)
- Changes apply immediately; server restart is not required

AI assistant APIs:

- POST /api/ai/chat builds organization- and client-scoped operational context and returns an answer plus proposed actions.
- POST /api/ai/actions executes an explicitly approved proposal using the current user's permissions and client scope.
- OpenRouter uses `https://openrouter.ai/api/v1` plus an API key. Hermes can run through an OpenAI-compatible endpoint from Ollama, llama.cpp, vLLM, or another local runtime; select Hermes/Local and configure its `/v1` base URL and model.
- Supported approved actions are ticket creation, device command dispatch, and alert acknowledgement. Model responses never execute actions automatically.

Agent reports API:

- GET /api/reports
- GET /api/reports/{deviceId}
- GET /api/reports/{deviceId}/metrics?minutes=180

## Manual Commands

- go run ./cmd/server
- go run ./cmd/server -tray-icon assets/icons/server/server.ico
- go run ./cmd/agent -id agent-1 -name "Windows Endpoint"
- go run ./cmd/agent -server localhost:8080 -state data/agent-state.json -heartbeat-seconds 10
- go run ./cmd/agent -server localhost:8080 -state data/agent-state.json -report-seconds 60
- go run ./cmd/agent -server localhost:8080 -state data/agent-state.json -enroll-token <token>
- go run ./cmd/agent -server localhost:8080 -state data/agent-state.json -rotate-now
- go run ./cmd/agent -server localhost:8080 -state data/agent-state.json -rotate-every-minutes 720
- go run ./cmd/agent -tray-icon assets/icons/agent/agent.ico
- cd web && npm.cmd install
- cd web && npm.cmd run build
- cd web && npm.cmd run dev
- go test ./...
- go vet ./...

## Agent Runtime Notes

- Agent now persists identity to disk (default path data/agent-state.json).
- Agent automatically reconnects with exponential backoff and jitter.
- If id/name flags are omitted, persisted values are reused across restarts.
- Enrollment flow now supports server-issued per-agent credentials:
   - Admin creates one-time token via POST /api/enrollment-tokens.
   - Admin can rotate any device credential via POST /api/agents/admin-rotate-key.
   - Agent redeems token using -enroll-token and receives persistent agent key.
   - Agent websocket auth requires agent_key/device_id credentials.
   - Agent can rotate its credential with POST /api/agents/rotate-key.
   - Agent supports startup rotation (-rotate-now) and periodic rotation (-rotate-every-minutes).
   - Audit entries for enrollment and rotation actions are available via GET /api/audit-events.

Reports now include point-in-time plus trend metrics:

- CPU usage percent
- Memory usage percent
- Memory used/total bytes
- Per-device time-series samples for charting in the Reports page

## Unenrolled Agent Recovery

If an agent is shown as unenrolled in the Reports page:

1. Open Reports and select the device row.
2. Click Create Recovery Token (60m).
3. Restart the target agent with the shown command and token.
4. Confirm the device status changes to Enrolled and reports continue updating.

## Tray Integration (Windows)

- Server and agent run with tray menus by default.
- Server menu includes: Status and Shutdown Server.
- Agent menu includes: Status and Shutdown Agent.

On Linux and other non-Windows/non-macOS targets, the agent runs in headless mode and logs status to stdout.

## Roadmap

- Multi-tenant org model and org-level RBAC
- Agent capability negotiation and sessions
- Remote terminal module (Linux and Windows shell relay MVP implemented)
- File transfer module
- Desktop streaming and control module
- Mesh relay and NAT traversal

## Notes

This is an MVP clone starter and not production-ready. Replace the token format with standard JWT/OIDC and rotate secrets before real deployment.

The app is being developed with future Dockerization in mind: frontend uses env-based API base URL and backend serves compiled SPA assets for simple single-container deployment during core development.
