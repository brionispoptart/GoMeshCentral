package hub

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"gomeshcentral/internal/storage"

	"github.com/gorilla/websocket"
)

type AgentEnvelope struct {
	Type      string          `json:"type"`
	DeviceID  string          `json:"deviceId,omitempty"`
	Name      string          `json:"name,omitempty"`
	Command   string          `json:"command,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Data      string          `json:"data,omitempty"`
	Cols      int             `json:"cols,omitempty"`
	Rows      int             `json:"rows,omitempty"`
	ExitCode  int             `json:"exitCode,omitempty"`
	Error     string          `json:"error,omitempty"`
	Report    json.RawMessage `json:"report,omitempty"`
	Path      string          `json:"path,omitempty"`
	FileSize  int64           `json:"fileSize,omitempty"`
}

type DashboardEvent struct {
	Type   string         `json:"type"`
	Device storage.Device `json:"device"`
}

type Hub struct {
	mu              sync.RWMutex
	agents          map[string]*websocket.Conn
	agentWrite      map[string]*sync.Mutex
	dashboards      map[*websocket.Conn]struct{}
	dashWrite       map[*websocket.Conn]*sync.Mutex
	sessions        map[string]TerminalSession
	fileSessions    map[string]chan AgentEnvelope
	store           storage.Store
	onAlertCreated  func(alert storage.Alert) error
	onTicketCreated func(ticket storage.Ticket) error
	onDeviceOffline func(device storage.Device) error
}

type TerminalSession struct {
	SessionID     string
	DeviceID      string
	Owner         string
	Client        *websocket.Conn
	CreatedAt     time.Time
	PendingOutput []AgentEnvelope
}

const maxPendingTerminalMessages = 512

var (
	ErrDeviceOffline           = errors.New("device offline")
	ErrTerminalSessionNotFound = errors.New("terminal session not found")
	ErrTerminalSessionDenied   = errors.New("terminal session forbidden")
)

func New(store storage.Store) *Hub {
	return &Hub{
		agents:       map[string]*websocket.Conn{},
		agentWrite:   map[string]*sync.Mutex{},
		dashboards:   map[*websocket.Conn]struct{}{},
		dashWrite:    map[*websocket.Conn]*sync.Mutex{},
		sessions:     map[string]TerminalSession{},
		fileSessions: map[string]chan AgentEnvelope{},
		store:        store,
	}
}

// SetOnAlertCreated registers a callback to be called when an alert is created
func (h *Hub) SetOnAlertCreated(fn func(alert storage.Alert) error) {
	h.mu.Lock()
	h.onAlertCreated = fn
	h.mu.Unlock()
}

// SetOnTicketCreated registers a callback to be called when a ticket is created
func (h *Hub) SetOnTicketCreated(fn func(ticket storage.Ticket) error) {
	h.mu.Lock()
	h.onTicketCreated = fn
	h.mu.Unlock()
}

// SetOnDeviceOffline registers a callback to be called when a device goes offline
func (h *Hub) SetOnDeviceOffline(fn func(device storage.Device) error) {
	h.mu.Lock()
	h.onDeviceOffline = fn
	h.mu.Unlock()
}

// GetOnTicketCreated returns the ticket created callback if set
func (h *Hub) GetOnTicketCreated() func(ticket storage.Ticket) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.onTicketCreated
}

// GetOnAlertCreated returns the alert created callback if set
func (h *Hub) GetOnAlertCreated() func(alert storage.Alert) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.onAlertCreated
}

func (h *Hub) RegisterAgent(deviceID string, conn *websocket.Conn) {
	h.mu.Lock()
	old := h.agents[deviceID]
	h.agents[deviceID] = conn
	h.agentWrite[deviceID] = &sync.Mutex{}
	h.store.SetDeviceConnection(deviceID, true)
	h.mu.Unlock()

	// If a previous connection still holds this device_id (e.g. the interactive
	// agent and the unattended service briefly overlap on the same identity),
	// close the stale one so exactly one live connection owns the device.
	if old != nil && old != conn {
		_ = old.Close()
	}

	h.resolveOfflineAlerts(deviceID)
	h.broadcastDevice(deviceID)
}

func (h *Hub) UnregisterAgent(deviceID string, conn *websocket.Conn) {
	h.mu.Lock()
	// Only tear down if this exact connection still owns the device. When two
	// connections share a device_id, an older connection's teardown must not wipe
	// the newer one's slot, which would orphan the live agent (device shows online
	// via heartbeats but terminal_open has nowhere to route).
	if current, ok := h.agents[deviceID]; ok && conn != nil && current != conn {
		h.mu.Unlock()
		return
	}
	delete(h.agents, deviceID)
	delete(h.agentWrite, deviceID)
	h.store.SetDeviceConnection(deviceID, false)
	staleSessions := make([]TerminalSession, 0)
	for sessionID, session := range h.sessions {
		if session.DeviceID == deviceID {
			staleSessions = append(staleSessions, session)
			delete(h.sessions, sessionID)
		}
	}
	h.mu.Unlock()

	for _, session := range staleSessions {
		if session.Client != nil {
			h.writeDashboard(session.Client, AgentEnvelope{Type: "terminal_exit", SessionID: session.SessionID, Error: "agent disconnected"})
		}
	}

	h.triggerOfflineAlerts(deviceID)
	h.broadcastDevice(deviceID)
}

func (h *Hub) RegisterDashboard(conn *websocket.Conn) {
	h.mu.Lock()
	h.dashboards[conn] = struct{}{}
	h.dashWrite[conn] = &sync.Mutex{}
	h.mu.Unlock()
}

func (h *Hub) UnregisterDashboard(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.dashboards, conn)
	delete(h.dashWrite, conn)
	h.mu.Unlock()
}

func (h *Hub) HandleAgentMessage(deviceID string, msg AgentEnvelope) {
	if strings.HasPrefix(msg.Type, "file_") {
		h.routeFileMessage(msg)
		return
	}

	if msg.Type == "heartbeat" {
		h.store.UpsertDevice(storage.Device{
			ID:            deviceID,
			Name:          msg.Name,
			LastHeartbeat: time.Now().UTC(),
			Connected:     true,
		})
		h.broadcastDevice(deviceID)
		return
	}

	if msg.Type == "report" && len(msg.Report) > 0 {
		var report storage.AgentReport
		if err := json.Unmarshal(msg.Report, &report); err != nil {
			return
		}
		report.DeviceID = deviceID
		report.ReportedAt = time.Now().UTC()
		_ = h.store.UpsertAgentReport(report)
		h.evaluateReportAlerts(deviceID, report)
		return
	}

	if msg.Type == "command_result" {
		details := strings.TrimSpace(msg.Command)
		if details == "" {
			details = "command=<empty>"
		} else {
			details = "command=" + details
		}
		details += ";exit_code=" + strconv.Itoa(msg.ExitCode)
		if msg.Error != "" {
			details += ";error=" + strings.TrimSpace(msg.Error)
		}
		if trimmedOutput := strings.TrimSpace(msg.Data); trimmedOutput != "" {
			if len(trimmedOutput) > 320 {
				trimmedOutput = trimmedOutput[:320] + "..."
			}
			details += ";output=" + trimmedOutput
		}
		_ = h.store.AppendAuditEvent(storage.AuditEvent{
			Action:  "agent_command_result",
			Actor:   "agent:" + deviceID,
			Target:  deviceID,
			Details: details,
		})
		return
	}

	if msg.Type == "terminal_data" || msg.Type == "terminal_exit" || msg.Type == "terminal_error" {
		h.mu.Lock()
		session, ok := h.sessions[msg.SessionID]
		if !ok || session.DeviceID != deviceID {
			h.mu.Unlock()
			return
		}
		client := session.Client
		if client == nil {
			// The shell emits its banner/prompt immediately after terminal_open,
			// which can arrive before the browser attaches its websocket. Buffer
			// that output and flush it on attach so nothing is lost.
			if len(session.PendingOutput) < maxPendingTerminalMessages {
				session.PendingOutput = append(session.PendingOutput, msg)
				h.sessions[msg.SessionID] = session
			}
			h.mu.Unlock()
			return
		}
		if msg.Type == "terminal_exit" {
			delete(h.sessions, msg.SessionID)
		}
		h.mu.Unlock()
		_ = h.writeDashboard(client, msg)
	}
}

func (h *Hub) SendCommand(deviceID, command string) error {
	msg := AgentEnvelope{Type: "command", Command: command}
	return h.writeAgent(deviceID, msg)
}

func (h *Hub) CreateTerminalSession(deviceID, owner string, cols, rows int) (TerminalSession, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 32
	}

	sessionID, err := randomSessionID()
	if err != nil {
		return TerminalSession{}, err
	}

	session := TerminalSession{
		SessionID: sessionID,
		DeviceID:  deviceID,
		Owner:     owner,
		CreatedAt: time.Now().UTC(),
	}

	h.mu.Lock()
	if _, ok := h.agents[deviceID]; !ok {
		h.mu.Unlock()
		return TerminalSession{}, ErrDeviceOffline
	}
	h.sessions[sessionID] = session
	h.mu.Unlock()

	openMsg := AgentEnvelope{Type: "terminal_open", SessionID: sessionID, DeviceID: deviceID, Cols: cols, Rows: rows}
	if err := h.writeAgent(deviceID, openMsg); err != nil {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
		return TerminalSession{}, ErrDeviceOffline
	}

	return session, nil
}

func (h *Hub) AttachTerminalClient(sessionID, owner string, conn *websocket.Conn) error {
	h.mu.Lock()
	session, ok := h.sessions[sessionID]
	if !ok {
		h.mu.Unlock()
		return ErrTerminalSessionNotFound
	}
	if session.Owner != owner {
		h.mu.Unlock()
		return ErrTerminalSessionDenied
	}
	session.Client = conn
	pending := session.PendingOutput
	session.PendingOutput = nil
	h.sessions[sessionID] = session
	h.dashWrite[conn] = &sync.Mutex{}
	h.mu.Unlock()

	for _, msg := range pending {
		_ = h.writeDashboard(conn, msg)
	}
	return nil
}

func (h *Hub) DetachTerminalClient(sessionID string, conn *websocket.Conn) {
	h.mu.Lock()
	session, ok := h.sessions[sessionID]
	if ok && session.Client == conn {
		session.Client = nil
		h.sessions[sessionID] = session
	}
	delete(h.dashWrite, conn)
	h.mu.Unlock()

	_ = h.writeToAgentSession(sessionID, AgentEnvelope{Type: "terminal_close", SessionID: sessionID})
	h.mu.Lock()
	delete(h.sessions, sessionID)
	h.mu.Unlock()
}

func (h *Hub) ForwardTerminalClientMessage(sessionID, owner string, msg AgentEnvelope) error {
	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()
	if !ok {
		return ErrTerminalSessionNotFound
	}
	if session.Owner != owner {
		return ErrTerminalSessionDenied
	}
	msg.SessionID = sessionID
	return h.writeAgent(session.DeviceID, msg)
}

func (h *Hub) writeToAgentSession(sessionID string, msg AgentEnvelope) error {
	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()
	if !ok {
		return ErrTerminalSessionNotFound
	}
	msg.SessionID = sessionID
	return h.writeAgent(session.DeviceID, msg)
}

func (h *Hub) writeAgent(deviceID string, msg AgentEnvelope) error {
	h.mu.RLock()
	conn, ok := h.agents[deviceID]
	mu := h.agentWrite[deviceID]
	h.mu.RUnlock()
	if !ok || conn == nil || mu == nil {
		return ErrDeviceOffline
	}
	mu.Lock()
	defer mu.Unlock()
	return conn.WriteJSON(msg)
}

func (h *Hub) writeDashboard(conn *websocket.Conn, msg AgentEnvelope) error {
	h.mu.RLock()
	mu := h.dashWrite[conn]
	h.mu.RUnlock()
	if mu == nil {
		return websocket.ErrCloseSent
	}
	mu.Lock()
	defer mu.Unlock()
	return conn.WriteJSON(msg)
}

func randomSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *Hub) broadcastDevice(deviceID string) {
	current, _ := h.store.GetDevice(deviceID)
	event := DashboardEvent{Type: "device_update", Device: current}
	payload, _ := json.Marshal(event)

	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.dashboards))
	for c := range h.dashboards {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		h.mu.RLock()
		mu := h.dashWrite[c]
		h.mu.RUnlock()
		if mu == nil {
			continue
		}
		mu.Lock()
		_ = c.WriteMessage(websocket.TextMessage, payload)
		mu.Unlock()
	}
}

// broadcastAlertsChanged notifies dashboards to refetch after an alert opens,
// resolves, or is acknowledged. The dashboard websocket handler refetches all
// state on any message, so the payload content only needs a distinct type.
func (h *Hub) broadcastAlertsChanged() {
	payload, _ := json.Marshal(DashboardEvent{Type: "alerts_update"})

	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.dashboards))
	for c := range h.dashboards {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		h.mu.RLock()
		mu := h.dashWrite[c]
		h.mu.RUnlock()
		if mu == nil {
			continue
		}
		mu.Lock()
		_ = c.WriteMessage(websocket.TextMessage, payload)
		mu.Unlock()
	}
}
