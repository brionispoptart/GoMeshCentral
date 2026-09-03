package httpapi

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gomeshcentral/internal/auth"
	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/config"
	"gomeshcentral/internal/email"
	"gomeshcentral/internal/hub"
	"gomeshcentral/internal/storage"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const claimsContextKey contextKey = "claims"

type Server struct {
	cfg                 config.Config
	store               storage.Store
	hub                 *hub.Hub
	httpSrv             *http.Server
	upgrader            websocket.Upgrader
	settingsMu          sync.RWMutex
	agentPublicAddr     string
	applicationSettings applicationSettings
	emailService        *email.Service
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

type commandRequest struct {
	Command string `json:"command"`
}

type terminalSessionCreateRequest struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type terminalSessionCreateResponse struct {
	SessionID string `json:"sessionId"`
	DeviceID  string `json:"deviceId"`
	WSPath    string `json:"wsPath"`
}

type enrollmentTokenRequest struct {
	TTLMinutes int `json:"ttlMinutes"`
}

type enrollmentTokenResponse struct {
	Token                        string    `json:"token"`
	ExpiresAt                    time.Time `json:"expiresAt"`
	AgentServer                  string    `json:"agentServer"`
	WindowsInteractiveCommand    string    `json:"windowsInteractiveCommand"`
	WindowsServiceInstallCommand string    `json:"windowsServiceInstallCommand"`
	WindowsOneLiner              string    `json:"windowsOneLiner"`
	WindowsMsiCommand            string    `json:"windowsMsiCommand"`
	LinuxInteractiveCommand      string    `json:"linuxInteractiveCommand"`
	LinuxOneLiner                string    `json:"linuxOneLiner"`
}

type enrollmentBootstrapResponse struct {
	AgentServer                   string `json:"agentServer"`
	WindowsInteractiveTemplate    string `json:"windowsInteractiveTemplate"`
	WindowsServiceInstallTemplate string `json:"windowsServiceInstallTemplate"`
	WindowsOneLinerTemplate       string `json:"windowsOneLinerTemplate"`
	WindowsMsiTemplate            string `json:"windowsMsiTemplate"`
	LinuxInteractiveTemplate      string `json:"linuxInteractiveTemplate"`
	LinuxOneLinerTemplate         string `json:"linuxOneLinerTemplate"`
}

type agentEndpointSettingsRequest struct {
	AgentPublicAddr string `json:"agentPublicAddr"`
}

type agentEndpointSettingsResponse struct {
	AgentPublicAddr      string `json:"agentPublicAddr"`
	EffectiveAgentServer string `json:"effectiveAgentServer"`
	RestartRequired      bool   `json:"restartRequired"`
}

type mailForwardingSettings struct {
	InvoiceTo    string `json:"invoiceTo"`
	AlertTo      string `json:"alertTo"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPUsername string `json:"smtpUsername"`
	SMTPPassword string `json:"smtpPassword"`
	FromAddress  string `json:"fromAddress"`
}

type applicationSettings struct {
	Theme          string                 `json:"theme"`
	LogoDataURL    string                 `json:"logoDataUrl"`
	CustomDomain   string                 `json:"customDomain"`
	MailForwarding mailForwardingSettings `json:"mailForwarding"`
	AI             aiSettings             `json:"ai"`
}

type aiSettings struct {
	Provider         string `json:"provider"`
	APIKey           string `json:"apiKey,omitempty"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
	BaseURL          string `json:"baseUrl"`
	Model            string `json:"model"`
}

type agentEnrollRequest struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId"`
	Name     string `json:"name,omitempty"`
}

type agentEnrollResponse struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
}

type agentRegisterRequest struct {
	Name          string `json:"name"`
	MachineIDHash string `json:"machineIdHash"`
	SystemIDHash  string `json:"systemIdHash"`
	BoardIDHash   string `json:"boardIdHash"`
}

type agentRegisterResponse struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
}

type rotateAgentCredentialRequest struct {
	DeviceID   string `json:"deviceId"`
	CurrentKey string `json:"currentAgentKey"`
}

type adminRotateAgentCredentialRequest struct {
	DeviceID string `json:"deviceId"`
}

type rotateAgentCredentialResponse struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
}

type authResponse struct {
	Token       string   `json:"token"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	OrgID       string   `json:"orgId"`
	OrgName     string   `json:"orgName"`
}

type workQueueItem struct {
	Text string `json:"text"`
	Done bool   `json:"done,omitempty"`
}

type workQueueSection struct {
	Title string          `json:"title"`
	Items []workQueueItem `json:"items"`
}

type workQueueResponse struct {
	CurrentMilestone map[string]string  `json:"currentMilestone"`
	Sections         []workQueueSection `json:"sections"`
}

func NewServer(cfg config.Config, store storage.Store) *Server {
	h := hub.New(store)
	mux := http.NewServeMux()

	defaultAppSettings := applicationSettings{
		Theme:        "default",
		CustomDomain: "",
		AI: aiSettings{
			Provider: "openrouter",
			BaseURL:  "https://openrouter.ai/api/v1",
			Model:    "openai/gpt-4o-mini",
		},
		MailForwarding: mailForwardingSettings{
			SMTPPort: 587,
		},
	}
	if store != nil {
		if raw, err := store.GetSetting("application_settings"); err == nil && strings.TrimSpace(raw) != "" {
			_ = json.Unmarshal([]byte(raw), &defaultAppSettings)
		}
	}

	agentPublicAddr := strings.TrimSpace(cfg.AgentPublicAddr)
	if store != nil {
		if storedAddr, err := store.GetSetting("agent_public_addr"); err == nil && strings.TrimSpace(storedAddr) != "" {
			agentPublicAddr = strings.TrimSpace(storedAddr)
		}
	}

	s := &Server{
		cfg:                 cfg,
		store:               store,
		hub:                 h,
		agentPublicAddr:     agentPublicAddr,
		applicationSettings: defaultAppSettings,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
	}

	// Initialize email service with current settings
	s.emailService = email.NewService(email.Config{
		SMTPHost:     defaultAppSettings.MailForwarding.SMTPHost,
		SMTPPort:     defaultAppSettings.MailForwarding.SMTPPort,
		SMTPUsername: defaultAppSettings.MailForwarding.SMTPUsername,
		SMTPPassword: defaultAppSettings.MailForwarding.SMTPPassword,
		FromAddress:  defaultAppSettings.MailForwarding.FromAddress,
	})

	// Set up hub callbacks for email notifications
	h.SetOnAlertCreated(func(alert storage.Alert) error {
		if !s.emailService.IsConfigured() || strings.TrimSpace(defaultAppSettings.MailForwarding.AlertTo) == "" {
			return nil
		}
		device, exists := store.GetDevice(alert.DeviceID)
		if !exists {
			return nil
		}
		return s.emailService.SendAlertEmail(
			defaultAppSettings.MailForwarding.AlertTo,
			device.Name,
			alert.MetricType,
			alert.Message,
		)
	})

	h.SetOnTicketCreated(func(ticket storage.Ticket) error {
		if !s.emailService.IsConfigured() || strings.TrimSpace(defaultAppSettings.MailForwarding.InvoiceTo) == "" {
			return nil
		}
		return s.emailService.SendTicketEmail(
			defaultAppSettings.MailForwarding.InvoiceTo,
			ticket.ID,
			ticket.Subject,
			"", // Will enhance with client name lookup if needed
		)
	})

	mux.HandleFunc("/api/login", s.handleLogin)
	// Force rebuild marker: 2026-09-01-17:40
	mux.HandleFunc("/api/admin/branding", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleBrandingCollection)))
	mux.HandleFunc("/api/organizations", s.authMiddleware(s.handleOrganizationsCollection))
	mux.HandleFunc("/api/organizations/switch", s.authMiddleware(s.handleSwitchOrganization))
	mux.HandleFunc("/api/organizations/register", s.handleRegisterOrganization)
	mux.HandleFunc("/api/me", s.authMiddleware(s.handleMe))
	mux.HandleFunc("/api/users", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleUsers)))
	mux.HandleFunc("/api/settings/application", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleApplicationSettings)))
	mux.HandleFunc("/api/ai/models", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleAIModels)))
	mux.HandleFunc("/api/ai/chat", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleAIChat)))
	mux.HandleFunc("/api/ai/actions", s.authMiddleware(s.handleAIActions))
	mux.HandleFunc("/api/settings/agent-endpoint", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleAgentEndpointSettings)))
	mux.HandleFunc("/api/enrollment-bootstrap", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleEnrollmentBootstrap)))
	mux.HandleFunc("/api/enrollment-tokens", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleCreateEnrollmentToken)))
	mux.HandleFunc("/api/download/install.sh", s.handleDownloadInstallSh)
	mux.HandleFunc("/api/download/install.ps1", s.handleDownloadInstallPs1)
	mux.HandleFunc("/api/download/install-verbose.ps1", s.handleDownloadInstallVerbosePs1)
	mux.HandleFunc("/api/download/agent/windows-amd64", s.handleDownloadAgentWindows)
	mux.HandleFunc("/api/download/agent/linux-amd64", s.handleDownloadAgentLinux)
	mux.HandleFunc("/api/download/agent/manifest", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleAgentManifest)))
	mux.HandleFunc("/api/download/agent/manifest-installer", s.authMiddleware(s.handleDownloadAgentManifestForInstaller))
	mux.HandleFunc("/api/agents/register", s.handleAgentRegister)
	mux.HandleFunc("/api/agents/enroll", s.handleAgentEnroll)
	mux.HandleFunc("/api/agents/rotate-key", s.handleRotateAgentCredential)
	mux.HandleFunc("/api/agents/admin-rotate-key", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleAdminRotateAgentCredential)))
	mux.HandleFunc("/api/audit-events", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleAuditEvents)))
	mux.HandleFunc("/api/work-queue", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleWorkQueue)))
	mux.HandleFunc("/api/devices", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleListDevices)))
	mux.HandleFunc("/api/reports", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleListReports)))
	mux.HandleFunc("/api/reports/", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleGetReport)))
	mux.HandleFunc("/api/devices/custom-fields", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleCustomFieldsCollection)))
	mux.HandleFunc("/api/devices/custom-fields/", s.authMiddleware(s.permissionMiddleware(authz.PermManageUsers, s.handleCustomFieldItem)))
	mux.HandleFunc("/api/devices/", s.authMiddleware(s.handleDeviceRoute))
	mux.HandleFunc("/api/clients", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleClientsCollection)))
	mux.HandleFunc("/api/clients/", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleClientItem)))
	// branding handler moved down before SPA
	mux.HandleFunc("/api/contracts", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleContractsCollection)))
	mux.HandleFunc("/api/contracts/", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleContractItem)))
	mux.HandleFunc("/api/time-entries", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleTimeEntriesCollection)))
	mux.HandleFunc("/api/time-entries/", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleTimeEntryItem)))
	mux.HandleFunc("/api/tickets", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleTicketsCollection)))
	mux.HandleFunc("/api/tickets/", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleTicketItem)))
	mux.HandleFunc("/api/invoices", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleInvoicesCollection)))
	mux.HandleFunc("/api/invoices/", s.authMiddleware(s.permissionMiddleware(authz.PermViewPSA, s.handleInvoiceItem)))
	mux.HandleFunc("/api/device-groups", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleDeviceGroupsCollection)))
	mux.HandleFunc("/api/device-groups/", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleDeviceGroupItem)))
	mux.HandleFunc("/api/scripts", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleScriptsCollection)))
	mux.HandleFunc("/api/scripts/", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleScriptItem)))
	mux.HandleFunc("/api/alert-rules", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleAlertRulesCollection)))
	mux.HandleFunc("/api/alert-rules/", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleAlertRuleItem)))
	mux.HandleFunc("/api/alerts", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleAlertsCollection)))
	mux.HandleFunc("/api/alerts/", s.authMiddleware(s.permissionMiddleware(authz.PermViewDevices, s.handleAlertItem)))

	// Client Portal Routes (public)
	mux.HandleFunc("/api/client/login", s.handleClientLogin)
	mux.HandleFunc("/api/client/me", s.clientAuthMiddleware(s.handleClientMe))
	mux.HandleFunc("/api/client/tickets", s.clientAuthMiddleware(s.handleClientTickets))
	mux.HandleFunc("/api/client/tickets/", s.clientAuthMiddleware(s.handleClientTicketAction))
	mux.HandleFunc("/api/client/invoices", s.clientAuthMiddleware(s.handleClientInvoices))
	mux.HandleFunc("/api/client/contracts", s.clientAuthMiddleware(s.handleClientContracts))
	mux.HandleFunc("/api/client/devices", s.clientAuthMiddleware(s.handleClientDevices))
	mux.HandleFunc("/api/admin/pending-approvals", s.authMiddleware(s.handlePendingApprovals))

	mux.HandleFunc("/ws/agent", s.handleAgentWS)
	mux.HandleFunc("/ws/dashboard", s.handleDashboardWS)
	mux.HandleFunc("/ws/terminal", s.handleTerminalWS)

	mux.HandleFunc("/", s.handleSPA)

	s.httpSrv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpSrv.Shutdown(ctx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
		http.NotFound(w, r)
		return
	}

	base := "web/dist"
	target := filepath.Join(base, filepath.Clean(r.URL.Path))
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return
	}

	indexPath := filepath.Join(base, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		http.Error(w, "frontend build missing: run 'npm run build' in web/", http.StatusServiceUnavailable)
		return
	}
	http.ServeFile(w, r, indexPath)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body credentials
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	user, ok := s.store.GetUser(body.Username)
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := auth.IssueToken(body.Username, user.Role, user.OrgID, s.cfg.JWTSecret, 12*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	orgName := "Default Organization"
	if org, ok := s.store.GetOrganization(user.OrgID); ok {
		orgName = org.Name
	}
	respondJSON(w, authResponse{
		Token:       token,
		Username:    user.Username,
		Role:        user.Role,
		Permissions: authz.Permissions(user.Role),
		OrgID:       user.OrgID,
		OrgName:     orgName,
	})
}

type registerOrgRequest struct {
	OrgName       string `json:"orgName"`
	AdminUsername string `json:"adminUsername"`
	AdminPassword string `json:"adminPassword"`
}

func (s *Server) handleRegisterOrganization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body registerOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body.OrgName = strings.TrimSpace(body.OrgName)
	body.AdminUsername = strings.TrimSpace(body.AdminUsername)
	if body.OrgName == "" || body.AdminUsername == "" || body.AdminPassword == "" {
		http.Error(w, "orgName, adminUsername, and adminPassword are required", http.StatusBadRequest)
		return
	}
	if _, exists := s.store.GetUser(body.AdminUsername); exists {
		http.Error(w, "username already taken", http.StatusConflict)
		return
	}
	org, err := s.store.CreateOrganization(storage.Organization{Name: body.OrgName})
	if err != nil {
		http.Error(w, "failed to create organization", http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	if err := s.store.CreateUser(body.AdminUsername, "", string(hash), authz.RoleAdmin, org.ID); err != nil {
		http.Error(w, "failed to create admin user", http.StatusInternalServerError)
		return
	}
	token, err := auth.IssueToken(body.AdminUsername, authz.RoleAdmin, org.ID, s.cfg.JWTSecret, 12*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "organization_registered",
		Actor:   body.AdminUsername,
		Target:  org.ID,
		OrgID:   org.ID,
		Details: "org_name=" + org.Name,
	})
	respondJSON(w, authResponse{
		Token:       token,
		Username:    body.AdminUsername,
		Role:        authz.RoleAdmin,
		Permissions: authz.Permissions(authz.RoleAdmin),
		OrgID:       org.ID,
		OrgName:     org.Name,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	orgName := "Default Organization"
	if org, ok := s.store.GetOrganization(claims.OrgID); ok {
		orgName = org.Name
	}
	respondJSON(w, authResponse{
		Username:    claims.Subject,
		Role:        claims.Role,
		Permissions: authz.Permissions(claims.Role),
		OrgID:       claims.OrgID,
		OrgName:     orgName,
	})
}

func (s *Server) handleOrganizationsCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		org, exists := s.store.GetOrganization(claims.OrgID)
		if !exists {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, []storage.Organization{org})
	case http.MethodPost:
		http.Error(w, "create clients within the current organization", http.StatusBadRequest)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSwitchOrganization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		OrgID string `json:"orgId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	targetOrgID := strings.TrimSpace(req.OrgID)
	if targetOrgID == "" {
		http.Error(w, "orgId is required", http.StatusBadRequest)
		return
	}
	if targetOrgID != claims.OrgID {
		http.Error(w, "organization access denied", http.StatusForbidden)
		return
	}
	org, ok := s.store.GetOrganization(targetOrgID)
	if !ok {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	token, err := auth.IssueToken(claims.Subject, claims.Role, org.ID, s.cfg.JWTSecret, 12*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "organization_switched",
		Actor:   claims.Subject,
		Target:  org.ID,
		OrgID:   org.ID,
		Details: "switched_to=" + org.Name,
	})

	respondJSON(w, authResponse{
		Token:       token,
		Username:    claims.Subject,
		Role:        claims.Role,
		Permissions: authz.Permissions(claims.Role),
		OrgID:       org.ID,
		OrgName:     org.Name,
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.store.ListUsers(claims.OrgID))
	case http.MethodPost:
		var body struct {
			Username  string `json:"username"`
			Password  string `json:"password"`
			Role      string `json:"role"`
			Email     string `json:"email,omitempty"`
			SendEmail bool   `json:"sendEmail"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if body.Username == "" || body.Password == "" || body.Role == "" {
			http.Error(w, "username, password, and role are required", http.StatusBadRequest)
			return
		}
		if !authz.IsValidRole(body.Role) {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		if err := s.store.CreateUser(body.Username, body.Email, string(hash), body.Role, claims.OrgID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		// Send credentials email if requested
		if body.SendEmail && body.Email != "" && s.emailService.IsConfigured() {
			html := fmt.Sprintf(`
				<html>
				<body style="font-family: Arial, sans-serif; color: #333;">
				<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
					<h2 style="color: #2563eb;">Welcome to GoMeshCentral</h2>
					<p>Your account has been created. Here are your login credentials:</p>
					<div style="background: #f3f4f6; padding: 15px; border-radius: 8px; margin: 20px 0;">
						<p><strong>Username:</strong> %s</p>
						<p><strong>Password:</strong> %s</p>
						<p><strong>Role:</strong> %s</p>
					</div>
					<p><strong>⚠️ Important:</strong> Please change your password immediately upon first login.</p>
					<p style="margin-top: 30px; font-size: 12px; color: #6b7280;">If you did not request this account, please contact your administrator.</p>
				</div>
				</body>
				</html>
			`, body.Username, body.Password, body.Role)

			_ = s.emailService.SendMessage(email.Message{
				To:       []string{body.Email},
				Subject:  "Your GoMeshCentral Account Credentials",
				HTMLBody: html,
			})
		}

		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	req := enrollmentTokenRequest{TTLMinutes: 60}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.TTLMinutes <= 0 {
		req.TTLMinutes = 60
	}
	if req.TTLMinutes > 24*60 {
		req.TTLMinutes = 24 * 60
	}

	expiresAt := time.Now().UTC().Add(time.Duration(req.TTLMinutes) * time.Minute)
	token, err := s.store.CreateEnrollmentToken(claims.Subject, claims.OrgID, expiresAt)
	if err != nil {
		http.Error(w, "failed to create enrollment token", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "enrollment_token_created",
		Actor:   claims.Subject,
		Target:  "enrollment",
		Details: "ttl_minutes=" + strconv.Itoa(req.TTLMinutes),
	})
	agentServer := s.resolveAgentServer(r)
	respondJSON(w, enrollmentTokenResponse{
		Token:                        token,
		ExpiresAt:                    expiresAt,
		AgentServer:                  agentServer,
		WindowsInteractiveCommand:    buildWindowsInteractiveEnrollCommand(agentServer, token),
		WindowsServiceInstallCommand: buildWindowsServiceInstallEnrollCommand(agentServer, token),
		WindowsOneLiner:              buildWindowsOneLinerCommand(agentServer, token),
		WindowsMsiCommand:            buildWindowsMsiCommand(agentServer, token),
		LinuxInteractiveCommand:      buildLinuxInteractiveEnrollCommand(agentServer, token),
		LinuxOneLiner:                buildLinuxOneLinerCommand(agentServer, token),
	})
}

func (s *Server) handleEnrollmentBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentServer := s.resolveAgentServer(r)
	respondJSON(w, enrollmentBootstrapResponse{
		AgentServer:                   agentServer,
		WindowsInteractiveTemplate:    buildWindowsInteractiveEnrollCommand(agentServer, "<token>"),
		WindowsServiceInstallTemplate: buildWindowsServiceInstallEnrollCommand(agentServer, "<token>"),
		WindowsOneLinerTemplate:       buildWindowsOneLinerCommand(agentServer, "<token>"),
		WindowsMsiTemplate:            buildWindowsMsiCommand(agentServer, "<token>"),
		LinuxInteractiveTemplate:      buildLinuxInteractiveEnrollCommand(agentServer, "<token>"),
		LinuxOneLinerTemplate:         buildLinuxOneLinerCommand(agentServer, "<token>"),
	})
}

func (s *Server) handleApplicationSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.getPublicApplicationSettings())
	case http.MethodPut:
		var req applicationSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if req.Theme == "" {
			req.Theme = "default"
		}
		if req.MailForwarding.SMTPPort == 0 {
			req.MailForwarding.SMTPPort = 587
		}
		current := s.getApplicationSettings()
		if strings.TrimSpace(req.AI.APIKey) == "" {
			req.AI.APIKey = current.AI.APIKey
		}
		s.setApplicationSettings(req)
		// Update email service with new settings
		s.emailService = email.NewService(email.Config{
			SMTPHost:     req.MailForwarding.SMTPHost,
			SMTPPort:     req.MailForwarding.SMTPPort,
			SMTPUsername: req.MailForwarding.SMTPUsername,
			SMTPPassword: req.MailForwarding.SMTPPassword,
			FromAddress:  req.MailForwarding.FromAddress,
		})
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "settings_application_updated",
			Actor:   claims.Subject,
			Target:  "settings",
			Details: fmt.Sprintf("theme=%s;custom_domain=%s;invoice_to=%s;alert_to=%s", req.Theme, req.CustomDomain, req.MailForwarding.InvoiceTo, req.MailForwarding.AlertTo),
		})
		respondJSON(w, s.getPublicApplicationSettings())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAgentEndpointSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, agentEndpointSettingsResponse{
			AgentPublicAddr:      s.getAgentPublicAddr(),
			EffectiveAgentServer: s.resolveAgentServer(r),
			RestartRequired:      false,
		})
	case http.MethodPut:
		var req agentEndpointSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		newValue := strings.TrimSpace(req.AgentPublicAddr)
		oldValue := s.getAgentPublicAddr()
		s.setAgentPublicAddr(newValue)
		details := "agent_public_addr=" + newValue
		if oldValue != newValue {
			details = "agent_public_addr_old=" + oldValue + ";agent_public_addr_new=" + newValue
		}
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "settings_agent_public_addr_updated",
			Actor:   claims.Subject,
			Target:  "settings",
			Details: details,
		})
		respondJSON(w, agentEndpointSettingsResponse{
			AgentPublicAddr:      s.getAgentPublicAddr(),
			EffectiveAgentServer: s.resolveAgentServer(r),
			RestartRequired:      false,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) resolveAgentServer(r *http.Request) string {
	configured := s.getAgentPublicAddr()
	if configured != "" {
		return configured
	}
	host := strings.TrimSpace(r.Host)
	if host != "" {
		return host
	}
	listen := strings.TrimSpace(s.cfg.ListenAddr)
	if listen == "" {
		return "localhost:8080"
	}
	if strings.HasPrefix(listen, ":") {
		return "localhost" + listen
	}
	return listen
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.httpSrv != nil && s.httpSrv.Handler != nil {
		s.httpSrv.Handler.ServeHTTP(w, r)
	}
}

func (s *Server) getApplicationSettings() applicationSettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	settings := s.applicationSettings
	settings.Theme = strings.TrimSpace(settings.Theme)
	if settings.Theme == "" {
		settings.Theme = "default"
	}
	settings.CustomDomain = strings.TrimSpace(settings.CustomDomain)
	if settings.MailForwarding.SMTPPort == 0 {
		settings.MailForwarding.SMTPPort = 587
	}
	settings.AI.BaseURL = aiProviderBaseURL(settings.AI.Provider, settings.AI.BaseURL)
	settings.AI.APIKeyConfigured = strings.TrimSpace(settings.AI.APIKey) != ""
	return settings
}

func (s *Server) getPublicApplicationSettings() applicationSettings {
	settings := s.getApplicationSettings()
	settings.AI.APIKey = ""
	return settings
}

func (s *Server) setApplicationSettings(settings applicationSettings) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if settings.Theme == "" {
		settings.Theme = "default"
	}
	if settings.MailForwarding.SMTPPort == 0 {
		settings.MailForwarding.SMTPPort = 587
	}
	if settings.AI.Provider == "" {
		settings.AI.Provider = "openrouter"
	}
	settings.AI.BaseURL = aiProviderBaseURL(settings.AI.Provider, settings.AI.BaseURL)
	if settings.AI.Model == "" {
		settings.AI.Model = "openai/gpt-4o-mini"
	}
	settings.AI.APIKeyConfigured = strings.TrimSpace(settings.AI.APIKey) != ""
	s.applicationSettings = settings
	if s.store != nil {
		if data, err := json.Marshal(settings); err == nil {
			_ = s.store.SaveSetting("application_settings", string(data))
		}
	}
}

func (s *Server) getAgentPublicAddr() string {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return strings.TrimSpace(s.agentPublicAddr)
}

func (s *Server) setAgentPublicAddr(value string) {
	s.settingsMu.Lock()
	s.agentPublicAddr = strings.TrimSpace(value)
	s.settingsMu.Unlock()
	if s.store != nil {
		_ = s.store.SaveSetting("agent_public_addr", s.agentPublicAddr)
	}
}

func buildWindowsInteractiveEnrollCommand(agentServer, token string) string {
	return fmt.Sprintf("agent.exe -server %s -state data\\agent-state.json -enroll-token %s", shellQuoteTokenLike(agentServer), shellQuoteTokenLike(token))
}

func buildWindowsServiceInstallEnrollCommand(agentServer, token string) string {
	return fmt.Sprintf("agent.exe -install-service -server %s -state C:\\ProgramData\\GoMeshCentral\\agent-state.json -enroll-token %s", shellQuoteTokenLike(agentServer), shellQuoteTokenLike(token))
}

func buildWindowsOneLinerCommand(agentServer, token string) string {
	if token != "" {
		return fmt.Sprintf("iwr -useb http://%s/api/download/install.ps1 | iex; Install-GoMeshAgent -Server %s -EnrollToken %s", agentServer, shellQuoteTokenLike(agentServer), shellQuoteTokenLike(token))
	}
	return fmt.Sprintf("iwr -useb http://%s/api/download/install.ps1 | iex", agentServer)
}

func buildWindowsMsiCommand(agentServer, token string) string {
	return fmt.Sprintf("msiexec /i GoMeshCentralAgent.msi SERVER=%s ENROLL_TOKEN=%s /qn", shellQuoteTokenLike(agentServer), shellQuoteTokenLike(token))
}

func buildLinuxInteractiveEnrollCommand(agentServer, token string) string {
	return fmt.Sprintf("./gomesh-agent -server %s -state /var/lib/gomeshcentral/agent-state.json -enroll-token %s", shellQuoteTokenLike(agentServer), shellQuoteTokenLike(token))
}

func buildLinuxOneLinerCommand(agentServer, token string) string {
	if token != "" {
		return fmt.Sprintf("curl -sSL http://%s/api/download/install.sh | sudo sh -s -- -server %s -enroll-token %s", agentServer, shellQuoteTokenLike(agentServer), shellQuoteTokenLike(token))
	}
	return fmt.Sprintf("curl -sSL http://%s/api/download/install.sh | sudo sh -s -- -server %s", agentServer, shellQuoteTokenLike(agentServer))
}

func (s *Server) handleDownloadInstallSh(w http.ResponseWriter, r *http.Request) {
	agentServer := s.resolveAgentServer(r)
	script, err := os.ReadFile("packaging/linux/install.sh")
	if err != nil {
		http.Error(w, "installer script unavailable", http.StatusNotFound)
		return
	}
	content := strings.ReplaceAll(string(script), `SERVER=""`, fmt.Sprintf(`SERVER="%s"`, agentServer))
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Header().Set("Content-Disposition", `attachment; filename="install.sh"`)
	_, _ = w.Write([]byte(content))
}

func (s *Server) handleDownloadInstallPs1(w http.ResponseWriter, r *http.Request) {
	agentServer := s.resolveAgentServer(r)
	script, err := os.ReadFile("packaging/windows/install.ps1")
	if err != nil {
		http.Error(w, "installer script unavailable", http.StatusNotFound)
		return
	}
	content := strings.ReplaceAll(string(script), `[string]$Server = ""`, fmt.Sprintf(`[string]$Server = "%s"`, agentServer))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="install.ps1"`)
	_, _ = w.Write([]byte(content))
}

func (s *Server) handleDownloadInstallVerbosePs1(w http.ResponseWriter, r *http.Request) {
	agentServer := s.resolveAgentServer(r)
	script, err := os.ReadFile("packaging/windows/install-verbose.ps1")
	if err != nil {
		http.Error(w, "verbose installer script unavailable", http.StatusNotFound)
		return
	}
	content := strings.ReplaceAll(string(script), `[string]$Server = ""`, fmt.Sprintf(`[string]$Server = "%s"`, agentServer))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="install-verbose.ps1"`)
	_, _ = w.Write([]byte(content))
}

func (s *Server) handleDownloadAgentWindows(w http.ResponseWriter, r *http.Request) {
	candidates := []string{"dist/agent.exe", "data/agent.exe", "agent.exe"}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="agent.exe"`)
			http.ServeFile(w, r, path)
			return
		}
	}
	http.Error(w, "Windows agent binary unavailable on server.", http.StatusNotFound)
}

func (s *Server) handleDownloadAgentLinux(w http.ResponseWriter, r *http.Request) {
	candidates := []string{"dist/gomesh-agent-linux", "data/gomesh-agent-linux", "gomesh-agent-linux"}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="gomesh-agent"`)
			http.ServeFile(w, r, path)
			return
		}
	}
	http.Error(w, "Linux agent binary unavailable on server.", http.StatusNotFound)
}

func shellQuoteTokenLike(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return "\"\""
	}
	v = strings.ReplaceAll(v, "\"", "\\\"")
	return "\"" + v + "\""
}

func (s *Server) handleWorkQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := os.ReadFile("docs/EXECUTION_TRACKER.md")
	if err != nil {
		http.Error(w, "execution tracker not found", http.StatusNotFound)
		return
	}

	parsed := parseWorkQueueMarkdown(string(data))
	respondJSON(w, parsed)
}

func (s *Server) handleAgentRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req agentRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	device, err := s.store.ResolveDeviceIdentity(req.MachineIDHash, req.SystemIDHash, req.BoardIDHash, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	agentKey, err := randomAgentKey()
	if err != nil {
		http.Error(w, "failed to issue agent key", http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(agentKey), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to secure agent key", http.StatusInternalServerError)
		return
	}
	if err := s.store.UpsertAgentCredential(device.ID, string(hash)); err != nil {
		http.Error(w, "failed to persist agent credential", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{Action: "agent_auto_registered", Actor: "agent:" + device.ID, Target: device.ID, Details: "hardware_identity=2-of-3"})
	respondJSON(w, agentRegisterResponse{DeviceID: device.ID, AgentKey: agentKey})
}

func (s *Server) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req agentEnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Token == "" || req.DeviceID == "" {
		http.Error(w, "token and deviceId are required", http.StatusBadRequest)
		return
	}

	orgID, consumed, err := s.store.ConsumeEnrollmentToken(req.Token, req.DeviceID)
	if err != nil {
		http.Error(w, "failed to validate enrollment token", http.StatusInternalServerError)
		return
	}
	if !consumed {
		http.Error(w, "invalid or expired enrollment token", http.StatusUnauthorized)
		return
	}

	agentKey, err := randomAgentKey()
	if err != nil {
		http.Error(w, "failed to issue agent key", http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(agentKey), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to secure agent key", http.StatusInternalServerError)
		return
	}
	if err := s.store.UpsertAgentCredential(req.DeviceID, string(hash)); err != nil {
		http.Error(w, "failed to persist agent credential", http.StatusInternalServerError)
		return
	}

	s.store.UpsertDevice(storage.Device{ID: req.DeviceID, Name: req.Name, Connected: false, OrgID: orgID})
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "agent_enrolled",
		Actor:   "agent:" + req.DeviceID,
		Target:  req.DeviceID,
		Details: "credential_issued=true",
	})
	respondJSON(w, agentEnrollResponse{DeviceID: req.DeviceID, AgentKey: agentKey})
}

func (s *Server) handleRotateAgentCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req rotateAgentCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.CurrentKey == "" {
		http.Error(w, "deviceId and currentAgentKey are required", http.StatusBadRequest)
		return
	}
	if !s.store.ValidateAgentCredential(req.DeviceID, req.CurrentKey) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	newKey, err := randomAgentKey()
	if err != nil {
		http.Error(w, "failed to issue agent key", http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newKey), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to secure agent key", http.StatusInternalServerError)
		return
	}
	if err := s.store.UpsertAgentCredential(req.DeviceID, string(hash)); err != nil {
		http.Error(w, "failed to persist agent credential", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "agent_key_rotated",
		Actor:   "agent:" + req.DeviceID,
		Target:  req.DeviceID,
		Details: "source=self_service",
	})

	respondJSON(w, rotateAgentCredentialResponse{DeviceID: req.DeviceID, AgentKey: newKey})
}

func (s *Server) handleAdminRotateAgentCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req adminRotateAgentCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" {
		http.Error(w, "deviceId is required", http.StatusBadRequest)
		return
	}

	newKey, err := randomAgentKey()
	if err != nil {
		http.Error(w, "failed to issue agent key", http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newKey), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to secure agent key", http.StatusInternalServerError)
		return
	}
	if err := s.store.UpsertAgentCredential(req.DeviceID, string(hash)); err != nil {
		http.Error(w, "failed to persist agent credential", http.StatusInternalServerError)
		return
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "agent_key_rotated",
		Actor:   claims.Subject,
		Target:  req.DeviceID,
		Details: "source=admin_console",
	})
	respondJSON(w, rotateAgentCredentialResponse{DeviceID: req.DeviceID, AgentKey: newKey})
}

func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	respondJSON(w, s.store.ListAuditEvents(claims.OrgID, 50))
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	clientID, valid := s.validClientScope(w, r, claims.OrgID)
	if !valid {
		return
	}
	devices := s.store.ListDevices(claims.OrgID)
	if clientID != "" {
		filtered := make([]storage.Device, 0, len(devices))
		for _, device := range devices {
			if device.ClientID == clientID {
				filtered = append(filtered, device)
			}
		}
		devices = filtered
	}
	respondJSON(w, devices)
}

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	clientID, valid := s.validClientScope(w, r, claims.OrgID)
	if !valid {
		return
	}
	reports := s.store.ListAgentReports(claims.OrgID)
	if clientID != "" {
		filtered := make([]storage.AgentReportView, 0, len(reports))
		for _, report := range reports {
			if report.Device.ClientID == clientID {
				filtered = append(filtered, report)
			}
		}
		reports = filtered
	}
	respondJSON(w, reports)
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/reports/")
	if strings.HasSuffix(path, "/metrics") {
		deviceID := strings.TrimSuffix(path, "/metrics")
		deviceID = strings.TrimSuffix(deviceID, "/")
		if deviceID == "" {
			http.NotFound(w, r)
			return
		}
		if !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
			return
		}
		minutes := 180
		if v := r.URL.Query().Get("minutes"); v != "" {
			parsed, err := strconv.Atoi(v)
			if err == nil && parsed > 0 {
				minutes = parsed
			}
		}
		if minutes > 7*24*60 {
			minutes = 7 * 24 * 60
		}
		since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
		samples := s.store.ListAgentMetricSamples(deviceID, since, 5000)
		respondJSON(w, samples)
		return
	}

	deviceID := path
	if deviceID == "" {
		http.NotFound(w, r)
		return
	}
	if !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	report, ok := s.store.GetAgentReport(deviceID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	respondJSON(w, report)
}

func (s *Server) handleDeviceRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if strings.HasSuffix(r.URL.Path, "/files/list") {
			s.permissionMiddleware(authz.PermSendCommand, s.handleListRemoteFiles)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/files/download") {
			s.permissionMiddleware(authz.PermSendCommand, s.handleDownloadRemoteFile)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/custom-fields") {
			s.permissionMiddleware(authz.PermViewDevices, s.handleGetDeviceCustomFields)(w, r)
			return
		}
		http.NotFound(w, r)
	case http.MethodPost:
		if strings.HasSuffix(r.URL.Path, "/command") {
			s.permissionMiddleware(authz.PermSendCommand, s.handleSendCommand)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/terminal/sessions") {
			s.permissionMiddleware(authz.PermSendCommand, s.handleCreateTerminalSession)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/client") {
			s.permissionMiddleware(authz.PermManagePSA, s.handleAssignDeviceClient)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/group") {
			s.permissionMiddleware(authz.PermSendCommand, s.handleAssignDeviceGroup)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/files/upload") {
			s.permissionMiddleware(authz.PermSendCommand, s.handleUploadRemoteFile)(w, r)
			return
		}
		http.NotFound(w, r)
	case http.MethodPut:
		if strings.HasSuffix(r.URL.Path, "/custom-fields") {
			s.permissionMiddleware(authz.PermManageUsers, s.handleUpdateDeviceCustomFields)(w, r)
			return
		}
		http.NotFound(w, r)
	case http.MethodDelete:
		s.permissionMiddleware(authz.PermManageUsers, s.handleDeleteDevice)(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAssignDeviceClient(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	deviceID := strings.TrimSuffix(path, "/client")
	if deviceID == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	var req struct {
		ClientID string `json:"clientId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ClientID != "" {
		client, exists := s.store.GetClient(req.ClientID)
		if !exists || client.OrgID != claims.OrgID {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
	}
	if err := s.store.AssignDeviceClient(deviceID, req.ClientID); err != nil {
		http.Error(w, "failed to assign client", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "device_client_assigned",
		Actor:   claims.Subject,
		Target:  deviceID,
		Details: "client_id=" + req.ClientID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAssignDeviceGroup(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	deviceID := strings.TrimSuffix(path, "/group")
	if deviceID == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	var req struct {
		GroupID string `json:"groupId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.store.AssignDeviceGroup(deviceID, req.GroupID); err != nil {
		http.Error(w, "failed to assign group", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "device_group_assigned",
		Actor:   claims.Subject,
		Target:  deviceID,
		Details: "group_id=" + req.GroupID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateTerminalSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[1] != "terminal" || parts[2] != "sessions" {
		http.NotFound(w, r)
		return
	}
	deviceID := parts[0]

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}

	req := terminalSessionCreateRequest{Cols: 120, Rows: 32}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	session, err := s.hub.CreateTerminalSession(deviceID, claims.Subject, req.Cols, req.Rows)
	if err != nil {
		if errors.Is(err, hub.ErrDeviceOffline) {
			http.Error(w, "device offline", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create terminal session", http.StatusInternalServerError)
		return
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "terminal_session_created",
		Actor:   claims.Subject,
		Target:  session.DeviceID,
		Details: "session_id=" + session.SessionID,
	})

	respondJSON(w, terminalSessionCreateResponse{
		SessionID: session.SessionID,
		DeviceID:  session.DeviceID,
		WSPath:    "/ws/terminal?session_id=" + session.SessionID,
	})
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	if len(parts) != 1 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	deviceID := parts[0]

	report, ok := s.store.GetAgentReport(deviceID)
	if ok && report.Device.Connected {
		http.Error(w, "cannot delete connected device", http.StatusConflict)
		return
	}

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	if err := s.store.DeleteDevice(deviceID); err != nil {
		http.Error(w, "failed to delete device", http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "device_deleted",
		Actor:   claims.Subject,
		Target:  deviceID,
		Details: "source=reports_cleanup",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSendCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "command" {
		http.NotFound(w, r)
		return
	}
	deviceID := parts[0]
	claims, ok := claimsFromContext(r.Context())
	if !ok || !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		http.Error(w, "command required", http.StatusBadRequest)
		return
	}
	if err := s.hub.SendCommand(deviceID, req.Command); err != nil {
		http.Error(w, "device offline", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	agentKey := r.URL.Query().Get("agent_key")
	deviceID := r.URL.Query().Get("device_id")
	if agentKey == "" || deviceID == "" {
		http.Error(w, "missing agent credential", http.StatusUnauthorized)
		return
	}
	if !s.store.ValidateAgentCredential(deviceID, agentKey) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	s.hub.RegisterAgent(deviceID, conn)
	defer s.hub.UnregisterAgent(deviceID, conn)

	for {
		var msg hub.AgentEnvelope
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		s.hub.HandleAgentMessage(deviceID, msg)
	}
}

func (s *Server) handleDashboardWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := r.URL.Query().Get("token")
	claims, err := auth.ParseToken(token, s.cfg.JWTSecret)
	if err != nil || !authz.Can(claims.Role, authz.PermViewDevices) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	s.hub.RegisterDashboard(conn)
	defer s.hub.UnregisterDashboard(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("session_id")
	if token == "" || sessionID == "" {
		http.Error(w, "missing token or session_id", http.StatusUnauthorized)
		return
	}
	claims, err := auth.ParseToken(token, s.cfg.JWTSecret)
	if err != nil || !authz.Can(claims.Role, authz.PermSendCommand) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if err := s.hub.AttachTerminalClient(sessionID, claims.Subject, conn); err != nil {
		switch {
		case errors.Is(err, hub.ErrTerminalSessionNotFound):
			_ = conn.WriteJSON(hub.AgentEnvelope{Type: "terminal_error", SessionID: sessionID, Error: "terminal session not found"})
		case errors.Is(err, hub.ErrTerminalSessionDenied):
			_ = conn.WriteJSON(hub.AgentEnvelope{Type: "terminal_error", SessionID: sessionID, Error: "forbidden"})
		default:
			_ = conn.WriteJSON(hub.AgentEnvelope{Type: "terminal_error", SessionID: sessionID, Error: "failed to attach terminal"})
		}
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "terminal attach failed"))
		return
	}
	defer s.hub.DetachTerminalClient(sessionID, conn)

	for {
		var msg hub.AgentEnvelope
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "terminal_data", "terminal_resize", "terminal_close":
			if err := s.hub.ForwardTerminalClientMessage(sessionID, claims.Subject, msg); err != nil {
				return
			}
		default:
			continue
		}
	}
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			hdr := r.Header.Get("Authorization")
			parts := strings.SplitN(hdr, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}
		// Query-param token fallback exists so plain browser navigations (e.g. a
		// file download link opened in a new tab) can authenticate without being
		// able to set an Authorization header, same pattern already used by the
		// dashboard/terminal websocket endpoints.
		if token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		claims, err := auth.ParseToken(token, s.cfg.JWTSecret)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if strings.TrimSpace(claims.OrgID) == "" {
			claims.OrgID = storage.DefaultOrgID
		}
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) permissionMiddleware(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !authz.Can(claims.Role, permission) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func claimsFromContext(ctx context.Context) (auth.Claims, bool) {
	v := ctx.Value(claimsContextKey)
	if v == nil {
		return auth.Claims{}, false
	}
	claims, ok := v.(auth.Claims)
	return claims, ok
}

func (s *Server) validClientScope(w http.ResponseWriter, r *http.Request, orgID string) (string, bool) {
	clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
	if clientID == "" {
		return "", true
	}
	client, exists := s.store.GetClient(clientID)
	if !exists || client.OrgID != orgID {
		http.Error(w, "client not found", http.StatusNotFound)
		return "", false
	}
	return clientID, true
}

func (s *Server) authorizeDeviceScope(w http.ResponseWriter, r *http.Request, orgID, deviceID string) bool {
	device, exists := s.store.GetDevice(deviceID)
	if !exists || device.OrgID != orgID {
		http.NotFound(w, r)
		return false
	}
	clientID, valid := s.validClientScope(w, r, orgID)
	if !valid {
		return false
	}
	if clientID != "" && device.ClientID != clientID {
		http.NotFound(w, r)
		return false
	}
	return true
}

func (s *Server) authorizeClientResource(w http.ResponseWriter, r *http.Request, orgID, resourceOrgID, resourceClientID string) bool {
	if resourceOrgID != orgID {
		http.NotFound(w, r)
		return false
	}
	clientID, valid := s.validClientScope(w, r, orgID)
	if !valid {
		return false
	}
	if clientID != "" && resourceClientID != clientID {
		http.NotFound(w, r)
		return false
	}
	return true
}

func (s *Server) clientBelongsToOrg(clientID, orgID string) bool {
	client, exists := s.store.GetClient(strings.TrimSpace(clientID))
	return exists && client.OrgID == orgID
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func randomAgentKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func parseWorkQueueMarkdown(content string) workQueueResponse {
	result := workQueueResponse{
		CurrentMilestone: map[string]string{},
		Sections:         []workQueueSection{},
	}

	sectionOrder := []string{"Completed", "In Progress", "Next Implementation Slice", "Upcoming", "Risks", "Handoff Notes"}
	sections := map[string]*workQueueSection{}
	for _, title := range sectionOrder {
		sections[title] = &workQueueSection{Title: title, Items: []workQueueItem{}}
	}

	currentSection := ""
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "## ") {
			currentSection = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}

		if currentSection == "Current Milestone" && strings.HasPrefix(line, "- ") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			parts := strings.SplitN(payload, ":", 2)
			if len(parts) == 2 {
				result.CurrentMilestone[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			continue
		}

		if section, ok := sections[currentSection]; ok && strings.HasPrefix(line, "- ") {
			if item, parsed := parseChecklistItem(line); parsed {
				section.Items = append(section.Items, item)
			}
		}
	}

	for _, title := range sectionOrder {
		section := sections[title]
		if len(section.Items) > 0 {
			result.Sections = append(result.Sections, *section)
		}
	}

	return result
}

func parseChecklistItem(line string) (workQueueItem, bool) {
	payload := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if strings.HasPrefix(payload, "[x] ") {
		return workQueueItem{Text: strings.TrimSpace(strings.TrimPrefix(payload, "[x] ")), Done: true}, true
	}
	if strings.HasPrefix(payload, "[ ] ") {
		return workQueueItem{Text: strings.TrimSpace(strings.TrimPrefix(payload, "[ ] ")), Done: false}, true
	}
	return workQueueItem{Text: payload}, true
}

func (s *Server) appendAuditEvent(event storage.AuditEvent) {
	event.CreatedAt = time.Now().UTC()
	if event.OrgID == "" {
		event.OrgID = s.resolveAuditOrgID(event.Actor)
	}
	if err := s.store.AppendAuditEvent(event); err != nil {
		log.Printf("append audit event failed: %v", err)
	}
}

func (s *Server) resolveAuditOrgID(actor string) string {
	if strings.HasPrefix(actor, "agent:") {
		deviceID := strings.TrimPrefix(actor, "agent:")
		if d, ok := s.store.GetDevice(deviceID); ok {
			return d.OrgID
		}
		return storage.DefaultOrgID
	}
	if u, ok := s.store.GetUser(actor); ok {
		return u.OrgID
	}
	return storage.DefaultOrgID
}
