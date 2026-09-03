package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	OrgID        string    `json:"orgId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// DefaultOrgID is the well-known organization every pre-existing (pre-multi-tenant)
// row is migrated into, so upgrading an existing single-tenant database keeps
// working exactly as before with zero manual steps.
const DefaultOrgID = "default-org"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Branding struct {
	OrgID       string    `json:"orgId"`
	CompanyName string    `json:"companyName"`
	Logo        string    `json:"logo"` // Base64 encoded logo image
	Icon        string    `json:"icon"` // Base64 encoded icon image
	PhoneNumber string    `json:"phoneNumber"`
	Website     string    `json:"website"`
	Email       string    `json:"email"`
	LogoPath    string    `json:"logoPath"` // File path to stored logo
	IconPath    string    `json:"iconPath"` // File path to stored icon
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Device struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
	Connected     bool              `json:"connected"`
	ClientID      string            `json:"clientId,omitempty"`
	GroupID       string            `json:"groupId,omitempty"`
	OrgID         string            `json:"orgId"`
	CustomFields  map[string]string `json:"customFields,omitempty"`
	MachineIDHash string            `json:"-"`
	SystemIDHash  string            `json:"-"`
	BoardIDHash   string            `json:"-"`
}

type DeviceGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	OrgID     string    `json:"orgId"`
	CreatedAt time.Time `json:"createdAt"`
}

type Script struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TargetOS    string    `json:"targetOs"`
	Body        string    `json:"body"`
	CreatedBy   string    `json:"createdBy"`
	OrgID       string    `json:"orgId"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AlertRule struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	MetricType     string    `json:"metricType"`
	Comparator     string    `json:"comparator"`
	ThresholdValue float64   `json:"thresholdValue"`
	ClientID       string    `json:"clientId,omitempty"`
	DeviceID       string    `json:"deviceId,omitempty"`
	Severity       string    `json:"severity"`
	Enabled        bool      `json:"enabled"`
	CreatedBy      string    `json:"createdBy"`
	OrgID          string    `json:"orgId"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Alert struct {
	ID             string    `json:"id"`
	RuleID         string    `json:"ruleId"`
	RuleName       string    `json:"ruleName"`
	DeviceID       string    `json:"deviceId"`
	MetricType     string    `json:"metricType"`
	Message        string    `json:"message"`
	Severity       string    `json:"severity"`
	Status         string    `json:"status"`
	Value          float64   `json:"value"`
	OrgID          string    `json:"orgId"`
	TriggeredAt    time.Time `json:"triggeredAt"`
	AcknowledgedAt time.Time `json:"acknowledgedAt,omitempty"`
	ResolvedAt     time.Time `json:"resolvedAt,omitempty"`
}

type Client struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	ContactName              string    `json:"contactName"`
	ContactEmail             string    `json:"contactEmail"`
	ContactPhone             string    `json:"contactPhone"`
	Address                  string    `json:"address"`
	Notes                    string    `json:"notes"`
	OrgID                    string    `json:"orgId"`
	CreatedAt                time.Time `json:"createdAt"`
	PortalEnabled            bool      `json:"portalEnabled"`
	PortalPasswordHash       string    `json:"-"`
	PortalPointOfContactID   string    `json:"portalPointOfContactId,omitempty"`
	PortalPointOfContactName string    `json:"portalPointOfContactName,omitempty"`
}

type Contract struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"clientId"`
	Name           string    `json:"name"`
	ContractType   string    `json:"contractType"`
	RateType       string    `json:"rateType"`
	RateAmount     float64   `json:"rateAmount"`
	BillingCycle   string    `json:"billingCycle"`
	StartDate      time.Time `json:"startDate"`
	EndDate        time.Time `json:"endDate,omitempty"`
	Status         string    `json:"status"`
	Notes          string    `json:"notes"`
	LastInvoicedAt time.Time `json:"lastInvoicedAt,omitempty"`
	OrgID          string    `json:"orgId"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Ticket struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"clientId,omitempty"`
	DeviceID    string    `json:"deviceId,omitempty"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Assignee    string    `json:"assignee"`
	CreatedBy   string    `json:"createdBy"`
	OrgID       string    `json:"orgId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ResolvedAt  time.Time `json:"resolvedAt,omitempty"`
	ApprovedBy  string    `json:"approvedBy,omitempty"`
	ApprovedAt  time.Time `json:"approvedAt,omitempty"`
}

type TicketComment struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticketId"`
	Author    string    `json:"author"`
	AuthorID  string    `json:"authorId"`
	Comment   string    `json:"comment"`
	IsPublic  bool      `json:"isPublic"`
	OrgID     string    `json:"orgId"`
	CreatedAt time.Time `json:"createdAt"`
}

type InvoiceLineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	Amount      float64 `json:"amount"`
}

type Invoice struct {
	ID            string            `json:"id"`
	ClientID      string            `json:"clientId"`
	ContractID    string            `json:"contractId,omitempty"`
	InvoiceNumber string            `json:"invoiceNumber"`
	Status        string            `json:"status"`
	IssueDate     time.Time         `json:"issueDate"`
	DueDate       time.Time         `json:"dueDate"`
	LineItems     []InvoiceLineItem `json:"lineItems"`
	Subtotal      float64           `json:"subtotal"`
	Tax           float64           `json:"tax"`
	Total         float64           `json:"total"`
	Notes         string            `json:"notes"`
	OrgID         string            `json:"orgId"`
	CreatedAt     time.Time         `json:"createdAt"`
}

type TimeEntry struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"clientId"`
	TicketID    string    `json:"ticketId,omitempty"`
	Description string    `json:"description"`
	Minutes     int       `json:"minutes"`
	Billable    bool      `json:"billable"`
	InvoiceID   string    `json:"invoiceId,omitempty"`
	CreatedBy   string    `json:"createdBy"`
	OrgID       string    `json:"orgId"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AuditEvent struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
	OrgID     string    `json:"orgId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type AgentReport struct {
	DeviceID           string    `json:"deviceId"`
	Hostname           string    `json:"hostname"`
	Username           string    `json:"username"`
	OS                 string    `json:"os"`
	Arch               string    `json:"arch"`
	CPUCount           int       `json:"cpuCount"`
	CPUUsagePercent    float64   `json:"cpuUsagePercent"`
	MemoryUsagePercent float64   `json:"memoryUsagePercent"`
	MemoryUsedBytes    uint64    `json:"memoryUsedBytes"`
	MemoryTotalBytes   uint64    `json:"memoryTotalBytes"`
	LocalIPs           []string  `json:"localIps"`
	ExecutablePath     string    `json:"executablePath"`
	WorkingDir         string    `json:"workingDir"`
	ProcessID          int       `json:"processId"`
	AgentStartedAt     time.Time `json:"agentStartedAt"`
	AgentUptimeSeconds int64     `json:"agentUptimeSeconds"`
	ReportedAt         time.Time `json:"reportedAt"`
}

type AgentReportView struct {
	Device   Device      `json:"device"`
	Enrolled bool        `json:"enrolled"`
	Report   AgentReport `json:"report"`
}

type AgentMetricSample struct {
	DeviceID           string    `json:"deviceId"`
	SampledAt          time.Time `json:"sampledAt"`
	CPUUsagePercent    float64   `json:"cpuUsagePercent"`
	MemoryUsagePercent float64   `json:"memoryUsagePercent"`
	MemoryUsedBytes    uint64    `json:"memoryUsedBytes"`
	MemoryTotalBytes   uint64    `json:"memoryTotalBytes"`
}

type Store interface {
	CreateOrganization(o Organization) (Organization, error)
	GetOrganization(id string) (Organization, bool)
	ListOrganizations() []Organization

	CreateUser(username, email, passwordHash, role, orgID string) error
	GetUser(username string) (User, bool)
	ListUsers(orgID string) []User
	UpsertBootstrapAdmin(username, passwordHash, orgID string) error
	CreateEnrollmentToken(createdBy, orgID string, expiresAt time.Time) (string, error)
	ConsumeEnrollmentToken(token, deviceID string) (orgID string, ok bool, err error)
	UpsertAgentCredential(deviceID, secretHash string) error
	ValidateAgentCredential(deviceID, secret string) bool
	HasAgentCredential(deviceID string) bool
	UpsertDevice(device Device)
	GetDevice(deviceID string) (Device, bool)
	ResolveDeviceIdentity(machineIDHash, systemIDHash, boardIDHash, name string) (Device, error)
	SetDeviceConnection(deviceID string, connected bool)
	ResetDeviceConnections() error
	ListDevices(orgID string) []Device
	DeleteDevice(deviceID string) error
	UpsertAgentReport(report AgentReport) error
	ListAgentReports(orgID string) []AgentReportView
	GetAgentReport(deviceID string) (AgentReportView, bool)
	ListAgentMetricSamples(deviceID string, since time.Time, limit int) []AgentMetricSample
	AppendAuditEvent(event AuditEvent) error
	ListAuditEvents(orgID string, limit int) []AuditEvent

	CreateClient(c Client) (Client, error)
	UpdateClient(c Client) error
	GetClient(id string) (Client, bool)
	ListClients(orgID string) []Client
	DeleteClient(id string) error
	AssignDeviceClient(deviceID, clientID string) error

	CreateContract(c Contract) (Contract, error)
	UpdateContract(c Contract) error
	GetContract(id string) (Contract, bool)
	ListContracts(orgID, clientID string) []Contract
	DeleteContract(id string) error
	SetContractLastInvoiced(id string, when time.Time) error

	CreateTicket(t Ticket) (Ticket, error)
	UpdateTicket(t Ticket) error
	GetTicket(id string) (Ticket, bool)
	ListTickets(orgID, clientID string) []Ticket
	DeleteTicket(id string) error

	CreateInvoice(inv Invoice) (Invoice, error)
	UpdateInvoice(inv Invoice) error
	GetInvoice(id string) (Invoice, bool)
	ListInvoices(orgID, clientID string) []Invoice
	DeleteInvoice(id string) error

	CreateDeviceGroup(g DeviceGroup) (DeviceGroup, error)
	ListDeviceGroups(orgID string) []DeviceGroup
	DeleteDeviceGroup(id string) error
	AssignDeviceGroup(deviceID, groupID string) error

	CreateScript(sc Script) (Script, error)
	GetScript(id string) (Script, bool)
	ListScripts(orgID string) []Script
	DeleteScript(id string) error

	CreateAlertRule(rule AlertRule) (AlertRule, error)
	UpdateAlertRule(rule AlertRule) error
	ListAlertRules(orgID string) []AlertRule
	DeleteAlertRule(id string) error

	CreateAlert(a Alert) (Alert, error)
	GetOpenAlert(ruleID, deviceID string) (Alert, bool)
	ListAlerts(orgID, status string) []Alert
	AcknowledgeAlert(id string) error
	ResolveAlert(id string) error
	ResolveOpenAlertsForDevice(deviceID string) error

	CreateTimeEntry(t TimeEntry) (TimeEntry, error)
	ListTimeEntries(orgID, clientID string) []TimeEntry
	ListUnbilledTimeEntries(orgID, clientID string) []TimeEntry
	MarkTimeEntriesInvoiced(ids []string, invoiceID string) error
	DeleteTimeEntry(id string) error

	SaveSetting(key, value string) error
	GetSetting(key string) (string, error)

	SaveBranding(b Branding) error
	GetBranding(orgID string) (Branding, error)

	// Device custom fields: define which fields exist for an org and update device field values
	ListCustomFieldDefinitions(orgID string) []string
	SaveCustomFieldDefinition(orgID, fieldName string) error
	DeleteCustomFieldDefinition(orgID, fieldName string) error
	GetDeviceCustomFields(deviceID string) map[string]string
	UpdateDeviceCustomFields(deviceID string, fields map[string]string) error
}

type EnrollmentToken struct {
	Token     string
	CreatedBy string
	OrgID     string
	ExpiresAt time.Time
	UsedBy    string
	UsedAt    time.Time
}

type MemoryStore struct {
	mu               sync.RWMutex
	users            map[string]User
	devices          map[string]Device
	enrollmentTokens map[string]EnrollmentToken
	agentCreds       map[string]string
	agentReports     map[string]AgentReport
	agentMetrics     map[string][]AgentMetricSample
	auditEvents      []AuditEvent
	nextAuditEventID int64
	branding         map[string]Branding
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:            map[string]User{},
		devices:          map[string]Device{},
		enrollmentTokens: map[string]EnrollmentToken{},
		agentCreds:       map[string]string{},
		agentReports:     map[string]AgentReport{},
		agentMetrics:     map[string][]AgentMetricSample{},
		auditEvents:      []AuditEvent{},
		nextAuditEventID: 1,
		branding:         map[string]Branding{},
	}
}

func (s *MemoryStore) CreateUser(username, email, passwordHash, role, orgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; ok {
		return errors.New("user already exists")
	}
	s.users[username] = User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		OrgID:        orgID,
		CreatedAt:    time.Now().UTC(),
	}
	return nil
}

func (s *MemoryStore) GetUser(username string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	return u, ok
}

func (s *MemoryStore) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		u.PasswordHash = ""
		out = append(out, u)
	}
	return out
}

func (s *MemoryStore) UpsertBootstrapAdmin(username, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		u = User{Username: username, CreatedAt: time.Now().UTC()}
	}
	u.PasswordHash = passwordHash
	u.Role = "admin"
	s.users[username] = u
	return nil
}

func (s *MemoryStore) CreateEnrollmentToken(createdBy string, expiresAt time.Time) (string, error) {
	token, err := randomMemoryToken()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.enrollmentTokens[token] = EnrollmentToken{
		Token:     token,
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}
	return token, nil
}

func (s *MemoryStore) ConsumeEnrollmentToken(token, deviceID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.enrollmentTokens[token]
	if !ok {
		return false, nil
	}
	if !rec.UsedAt.IsZero() {
		return false, nil
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		return false, nil
	}
	rec.UsedBy = deviceID
	rec.UsedAt = time.Now().UTC()
	s.enrollmentTokens[token] = rec
	return true, nil
}

func (s *MemoryStore) UpsertAgentCredential(deviceID, secretHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentCreds[deviceID] = secretHash
	return nil
}

func (s *MemoryStore) ValidateAgentCredential(deviceID, secret string) bool {
	s.mu.RLock()
	hash, ok := s.agentCreds[deviceID]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

func (s *MemoryStore) HasAgentCredential(deviceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.agentCreds[deviceID]
	return ok
}

func (s *MemoryStore) UpsertDevice(device Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.devices[device.ID]
	if ok {
		if device.Name == "" {
			device.Name = existing.Name
		}
	}
	s.devices[device.ID] = device
}

func (s *MemoryStore) ResolveDeviceIdentity(machineIDHash, systemIDHash, boardIDHash, name string) (Device, error) {
	if countMemoryNonEmpty(machineIDHash, systemIDHash, boardIDHash) < 2 {
		return Device{}, errors.New("at least two hardware identifiers are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var match Device
	bestScore := 0
	for _, candidate := range s.devices {
		score := countMemoryMatches(candidate, machineIDHash, systemIDHash, boardIDHash)
		if score > bestScore {
			match, bestScore = candidate, score
		}
	}
	if bestScore < 2 {
		id, err := randomMemoryToken()
		if err != nil {
			return Device{}, err
		}
		match = Device{ID: "agent-" + id, OrgID: DefaultOrgID}
	}
	if name != "" {
		match.Name = name
	}
	match.MachineIDHash, match.SystemIDHash, match.BoardIDHash = machineIDHash, systemIDHash, boardIDHash
	s.devices[match.ID] = match
	return match, nil
}

func countMemoryNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func countMemoryMatches(device Device, machineIDHash, systemIDHash, boardIDHash string) int {
	score := 0
	if device.MachineIDHash != "" && device.MachineIDHash == machineIDHash {
		score++
	}
	if device.SystemIDHash != "" && device.SystemIDHash == systemIDHash {
		score++
	}
	if device.BoardIDHash != "" && device.BoardIDHash == boardIDHash {
		score++
	}
	return score
}

func (s *MemoryStore) SetDeviceConnection(deviceID string, connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.devices[deviceID]
	d.ID = deviceID
	d.Connected = connected
	if connected {
		d.LastHeartbeat = time.Now().UTC()
	}
	s.devices[deviceID] = d
}

func (s *MemoryStore) ResetDeviceConnections() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for deviceID, device := range s.devices {
		device.Connected = false
		s.devices[deviceID] = device
	}
	return nil
}

func (s *MemoryStore) ListDevices() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Device, 0, len(s.devices))
	for _, d := range s.devices {
		out = append(out, d)
	}
	return out
}

func (s *MemoryStore) DeleteDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.devices, deviceID)
	delete(s.agentCreds, deviceID)
	delete(s.agentReports, deviceID)
	delete(s.agentMetrics, deviceID)
	return nil
}

func (s *MemoryStore) UpsertAgentReport(report AgentReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if report.ReportedAt.IsZero() {
		report.ReportedAt = time.Now().UTC()
	}
	s.agentReports[report.DeviceID] = report
	metric := AgentMetricSample{
		DeviceID:           report.DeviceID,
		SampledAt:          report.ReportedAt,
		CPUUsagePercent:    report.CPUUsagePercent,
		MemoryUsagePercent: report.MemoryUsagePercent,
		MemoryUsedBytes:    report.MemoryUsedBytes,
		MemoryTotalBytes:   report.MemoryTotalBytes,
	}
	s.agentMetrics[report.DeviceID] = append(s.agentMetrics[report.DeviceID], metric)
	if len(s.agentMetrics[report.DeviceID]) > 2000 {
		s.agentMetrics[report.DeviceID] = s.agentMetrics[report.DeviceID][len(s.agentMetrics[report.DeviceID])-2000:]
	}
	return nil
}

func (s *MemoryStore) ListAgentReports() []AgentReportView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AgentReportView, 0, len(s.devices))
	for _, d := range s.devices {
		report := s.agentReports[d.ID]
		_, enrolled := s.agentCreds[d.ID]
		out = append(out, AgentReportView{Device: d, Enrolled: enrolled, Report: report})
	}
	return out
}

func (s *MemoryStore) GetAgentReport(deviceID string) (AgentReportView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[deviceID]
	if !ok {
		return AgentReportView{}, false
	}
	report := s.agentReports[deviceID]
	_, enrolled := s.agentCreds[deviceID]
	return AgentReportView{Device: d, Enrolled: enrolled, Report: report}, true
}

func (s *MemoryStore) ListAgentMetricSamples(deviceID string, since time.Time, limit int) []AgentMetricSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	samples := s.agentMetrics[deviceID]
	if limit <= 0 {
		limit = 500
	}
	out := make([]AgentMetricSample, 0, min(limit, len(samples)))
	for i := 0; i < len(samples); i++ {
		if !since.IsZero() && samples[i].SampledAt.Before(since) {
			continue
		}
		out = append(out, samples[i])
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *MemoryStore) AppendAuditEvent(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event.ID = s.nextAuditEventID
	s.nextAuditEventID++
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	s.auditEvents = append(s.auditEvents, event)
	return nil
}

func (s *MemoryStore) ListAuditEvents(limit int) []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	if limit > len(s.auditEvents) {
		limit = len(s.auditEvents)
	}
	out := make([]AuditEvent, 0, limit)
	for i := len(s.auditEvents) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.auditEvents[i])
	}
	return out
}

func randomMemoryToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *MemoryStore) SaveBranding(b Branding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b.UpdatedAt = time.Now().UTC()
	s.branding[b.OrgID] = b
	return nil
}

func (s *MemoryStore) GetBranding(orgID string) (Branding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if b, ok := s.branding[orgID]; ok {
		return b, nil
	}
	return Branding{OrgID: orgID}, nil
}
