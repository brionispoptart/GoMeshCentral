package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases the underlying database handle (used by tests and graceful shutdown).
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	userDDL := `
CREATE TABLE IF NOT EXISTS users (
  username TEXT PRIMARY KEY,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  created_at TEXT NOT NULL
);`
	deviceDDL := `
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  last_heartbeat TEXT,
  connected INTEGER NOT NULL DEFAULT 0
);`
	enrollmentDDL := `
CREATE TABLE IF NOT EXISTS enrollment_tokens (
	token TEXT PRIMARY KEY,
	created_by TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	used_by TEXT,
	used_at TEXT
);`
	agentCredDDL := `
CREATE TABLE IF NOT EXISTS agent_credentials (
	device_id TEXT PRIMARY KEY,
	secret_hash TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);`
	agentReportDDL := `
CREATE TABLE IF NOT EXISTS agent_reports (
	device_id TEXT PRIMARY KEY,
	hostname TEXT NOT NULL DEFAULT '',
	username TEXT NOT NULL DEFAULT '',
	os TEXT NOT NULL DEFAULT '',
	arch TEXT NOT NULL DEFAULT '',
	cpu_count INTEGER NOT NULL DEFAULT 0,
	cpu_usage_percent REAL NOT NULL DEFAULT 0,
	memory_usage_percent REAL NOT NULL DEFAULT 0,
	memory_used_bytes INTEGER NOT NULL DEFAULT 0,
	memory_total_bytes INTEGER NOT NULL DEFAULT 0,
	local_ips_json TEXT NOT NULL DEFAULT '[]',
	executable_path TEXT NOT NULL DEFAULT '',
	working_dir TEXT NOT NULL DEFAULT '',
	process_id INTEGER NOT NULL DEFAULT 0,
	agent_started_at TEXT,
	agent_uptime_seconds INTEGER NOT NULL DEFAULT 0,
	reported_at TEXT NOT NULL
);`
	agentMetricSampleDDL := `
CREATE TABLE IF NOT EXISTS agent_metric_samples (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL,
	sampled_at TEXT NOT NULL,
	cpu_usage_percent REAL NOT NULL DEFAULT 0,
	memory_usage_percent REAL NOT NULL DEFAULT 0,
	memory_used_bytes INTEGER NOT NULL DEFAULT 0,
	memory_total_bytes INTEGER NOT NULL DEFAULT 0
);`
	agentMetricSampleIndexDDL := `
CREATE INDEX IF NOT EXISTS idx_agent_metric_samples_device_time
ON agent_metric_samples(device_id, sampled_at);
`
	auditEventDDL := `
CREATE TABLE IF NOT EXISTS audit_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	action TEXT NOT NULL,
	actor TEXT NOT NULL,
	target TEXT NOT NULL,
	details TEXT NOT NULL,
	created_at TEXT NOT NULL
);`
	clientDDL := `
CREATE TABLE IF NOT EXISTS clients (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	contact_name TEXT NOT NULL DEFAULT '',
	contact_email TEXT NOT NULL DEFAULT '',
	contact_phone TEXT NOT NULL DEFAULT '',
	address TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	contractDDL := `
CREATE TABLE IF NOT EXISTS contracts (
	id TEXT PRIMARY KEY,
	client_id TEXT NOT NULL,
	name TEXT NOT NULL,
	contract_type TEXT NOT NULL DEFAULT '',
	rate_type TEXT NOT NULL DEFAULT '',
	rate_amount REAL NOT NULL DEFAULT 0,
	billing_cycle TEXT NOT NULL DEFAULT '',
	start_date TEXT,
	end_date TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	ticketDDL := `
CREATE TABLE IF NOT EXISTS tickets (
	id TEXT PRIMARY KEY,
	client_id TEXT NOT NULL DEFAULT '',
	device_id TEXT NOT NULL DEFAULT '',
	subject TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	priority TEXT NOT NULL DEFAULT 'medium',
	assignee TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	resolved_at TEXT
);`
	invoiceDDL := `
CREATE TABLE IF NOT EXISTS invoices (
	id TEXT PRIMARY KEY,
	client_id TEXT NOT NULL,
	contract_id TEXT NOT NULL DEFAULT '',
	invoice_number TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	issue_date TEXT,
	due_date TEXT,
	line_items_json TEXT NOT NULL DEFAULT '[]',
	subtotal REAL NOT NULL DEFAULT 0,
	tax REAL NOT NULL DEFAULT 0,
	total REAL NOT NULL DEFAULT 0,
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	if _, err := s.db.Exec(userDDL); err != nil {
		return fmt.Errorf("migrate users: %w", err)
	}
	if _, err := s.db.Exec(deviceDDL); err != nil {
		return fmt.Errorf("migrate devices: %w", err)
	}
	if _, err := s.db.Exec(enrollmentDDL); err != nil {
		return fmt.Errorf("migrate enrollment tokens: %w", err)
	}
	if _, err := s.db.Exec(agentCredDDL); err != nil {
		return fmt.Errorf("migrate agent credentials: %w", err)
	}
	if _, err := s.db.Exec(agentReportDDL); err != nil {
		return fmt.Errorf("migrate agent reports: %w", err)
	}
	if err := s.addColumnIfMissing("agent_reports", "cpu_usage_percent", "REAL NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("migrate agent reports cpu_usage_percent: %w", err)
	}
	if err := s.addColumnIfMissing("agent_reports", "memory_usage_percent", "REAL NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("migrate agent reports memory_usage_percent: %w", err)
	}
	if err := s.addColumnIfMissing("agent_reports", "memory_used_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("migrate agent reports memory_used_bytes: %w", err)
	}
	if err := s.addColumnIfMissing("agent_reports", "memory_total_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("migrate agent reports memory_total_bytes: %w", err)
	}
	if _, err := s.db.Exec(agentMetricSampleDDL); err != nil {
		return fmt.Errorf("migrate agent metric samples: %w", err)
	}
	if _, err := s.db.Exec(agentMetricSampleIndexDDL); err != nil {
		return fmt.Errorf("migrate agent metric samples index: %w", err)
	}
	if _, err := s.db.Exec(auditEventDDL); err != nil {
		return fmt.Errorf("migrate audit events: %w", err)
	}
	if err := s.addColumnIfMissing("devices", "client_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate devices client_id: %w", err)
	}
	if _, err := s.db.Exec(clientDDL); err != nil {
		return fmt.Errorf("migrate clients: %w", err)
	}
	if _, err := s.db.Exec(contractDDL); err != nil {
		return fmt.Errorf("migrate contracts: %w", err)
	}
	if _, err := s.db.Exec(ticketDDL); err != nil {
		return fmt.Errorf("migrate tickets: %w", err)
	}
	if _, err := s.db.Exec(invoiceDDL); err != nil {
		return fmt.Errorf("migrate invoices: %w", err)
	}
	if err := s.addColumnIfMissing("devices", "group_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate devices group_id: %w", err)
	}
	for _, column := range []string{"machine_id_hash", "system_id_hash", "board_id_hash"} {
		if err := s.addColumnIfMissing("devices", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("migrate devices %s: %w", column, err)
		}
	}
	deviceGroupDDL := `
CREATE TABLE IF NOT EXISTS device_groups (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	scriptDDL := `
CREATE TABLE IF NOT EXISTS scripts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	target_os TEXT NOT NULL DEFAULT 'any',
	body TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	if _, err := s.db.Exec(deviceGroupDDL); err != nil {
		return fmt.Errorf("migrate device groups: %w", err)
	}
	if _, err := s.db.Exec(scriptDDL); err != nil {
		return fmt.Errorf("migrate scripts: %w", err)
	}
	alertRuleDDL := `
CREATE TABLE IF NOT EXISTS alert_rules (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	metric_type TEXT NOT NULL,
	comparator TEXT NOT NULL DEFAULT 'gt',
	threshold_value REAL NOT NULL DEFAULT 0,
	client_id TEXT NOT NULL DEFAULT '',
	device_id TEXT NOT NULL DEFAULT '',
	severity TEXT NOT NULL DEFAULT 'warning',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	alertDDL := `
CREATE TABLE IF NOT EXISTS alerts (
	id TEXT PRIMARY KEY,
	rule_id TEXT NOT NULL,
	rule_name TEXT NOT NULL DEFAULT '',
	device_id TEXT NOT NULL,
	metric_type TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	severity TEXT NOT NULL DEFAULT 'warning',
	status TEXT NOT NULL DEFAULT 'open',
	value REAL NOT NULL DEFAULT 0,
	triggered_at TEXT NOT NULL,
	acknowledged_at TEXT NOT NULL DEFAULT '',
	resolved_at TEXT NOT NULL DEFAULT ''
);`
	alertIndexDDL := `
CREATE INDEX IF NOT EXISTS idx_alerts_rule_device_status
ON alerts(rule_id, device_id, status);
`
	if _, err := s.db.Exec(alertRuleDDL); err != nil {
		return fmt.Errorf("migrate alert rules: %w", err)
	}
	if _, err := s.db.Exec(alertDDL); err != nil {
		return fmt.Errorf("migrate alerts: %w", err)
	}
	if _, err := s.db.Exec(alertIndexDDL); err != nil {
		return fmt.Errorf("migrate alerts index: %w", err)
	}
	if err := s.addColumnIfMissing("contracts", "last_invoiced_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate contracts last_invoiced_at: %w", err)
	}
	timeEntryDDL := `
CREATE TABLE IF NOT EXISTS time_entries (
	id TEXT PRIMARY KEY,
	client_id TEXT NOT NULL,
	ticket_id TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	minutes INTEGER NOT NULL DEFAULT 0,
	billable INTEGER NOT NULL DEFAULT 1,
	invoice_id TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);`
	if _, err := s.db.Exec(timeEntryDDL); err != nil {
		return fmt.Errorf("migrate time entries: %w", err)
	}

	// --- Multi-tenancy: organizations table + org_id backfilled onto every
	// tenant-scoped table. Existing rows (from before multi-tenancy existed)
	// default to DefaultOrgID so an upgraded single-tenant database keeps
	// working exactly as before with zero manual migration steps.
	orgDDL := `
CREATE TABLE IF NOT EXISTS organizations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL
);`
	if _, err := s.db.Exec(orgDDL); err != nil {
		return fmt.Errorf("migrate organizations: %w", err)
	}
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO organizations(id, name, created_at) VALUES(?, ?, ?)`,
		DefaultOrgID, "Default Organization", formatTimePtr(time.Now().UTC()),
	); err != nil {
		return fmt.Errorf("seed default organization: %w", err)
	}

	orgScopedTables := []string{
		"users", "devices", "clients", "contracts", "tickets", "invoices",
		"device_groups", "scripts", "alert_rules", "alerts", "time_entries",
		"audit_events",
	}
	for _, table := range orgScopedTables {
		if err := s.addColumnIfMissing(table, "org_id", fmt.Sprintf("TEXT NOT NULL DEFAULT '%s'", DefaultOrgID)); err != nil {
			return fmt.Errorf("migrate %s org_id: %w", table, err)
		}
	}
	if err := s.addColumnIfMissing("enrollment_tokens", "org_id", fmt.Sprintf("TEXT NOT NULL DEFAULT '%s'", DefaultOrgID)); err != nil {
		return fmt.Errorf("migrate enrollment_tokens org_id: %w", err)
	}

	// Client Portal Fields
	if err := s.addColumnIfMissing("clients", "portal_enabled", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("migrate clients portal_enabled: %w", err)
	}
	if err := s.addColumnIfMissing("clients", "portal_password_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate clients portal_password_hash: %w", err)
	}
	if err := s.addColumnIfMissing("clients", "portal_point_of_contact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate clients portal_point_of_contact_id: %w", err)
	}
	if err := s.addColumnIfMissing("clients", "portal_point_of_contact_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate clients portal_point_of_contact_name: %w", err)
	}

	// Ticket Approval Fields
	if err := s.addColumnIfMissing("tickets", "approved_by", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate tickets approved_by: %w", err)
	}
	if err := s.addColumnIfMissing("tickets", "approved_at", "TEXT"); err != nil {
		return fmt.Errorf("migrate tickets approved_at: %w", err)
	}

	appSettingsDDL := `
CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL
);`
	if _, err := s.db.Exec(appSettingsDDL); err != nil {
		return fmt.Errorf("migrate app_settings: %w", err)
	}

	// User email field for credential delivery
	if err := s.addColumnIfMissing("users", "email", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate users email: %w", err)
	}

	// Branding table for custom company branding per organization
	brandingDDL := `
CREATE TABLE IF NOT EXISTS branding (
	org_id TEXT PRIMARY KEY,
	company_name TEXT NOT NULL DEFAULT '',
	logo TEXT NOT NULL DEFAULT '',
	icon TEXT NOT NULL DEFAULT '',
	phone_number TEXT NOT NULL DEFAULT '',
	website TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	logo_path TEXT NOT NULL DEFAULT '',
	icon_path TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);
`
	if _, err := s.db.Exec(brandingDDL); err != nil {
		return fmt.Errorf("migrate branding: %w", err)
	}

	// Custom field definitions per org
	customFieldDefDDL := `
CREATE TABLE IF NOT EXISTS custom_field_definitions (
	org_id TEXT NOT NULL,
	field_name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (org_id, field_name)
);`
	if _, err := s.db.Exec(customFieldDefDDL); err != nil {
		return fmt.Errorf("migrate custom_field_definitions: %w", err)
	}

	// Custom field values per device
	customFieldValuesDDL := `
CREATE TABLE IF NOT EXISTS device_custom_fields (
	device_id TEXT NOT NULL,
	field_name TEXT NOT NULL,
	field_value TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (device_id, field_name)
);`
	if _, err := s.db.Exec(customFieldValuesDDL); err != nil {
		return fmt.Errorf("migrate device_custom_fields: %w", err)
	}

	return nil
}

// addColumnIfMissing checks if a column exists in a table and adds it if missing.
func (s *SQLiteStore) addColumnIfMissing(table, column, definition string) error {
	// Query PRAGMA table_info to get all columns for the table
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("check table info: %w", err)
	}
	defer rows.Close()

	// Check if the column already exists
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}

		if name == column {
			// Column already exists, no error
			return nil
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate table info: %w", err)
	}

	// Column doesn't exist, add it
	alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := s.db.Exec(alterSQL); err != nil {
		return fmt.Errorf("add column %s to %s: %w", column, table, err)
	}

	return nil
}

func (s *SQLiteStore) CreateOrganization(o Organization) (Organization, error) {
	id, err := randomToken()
	if err != nil {
		return Organization{}, err
	}
	o.ID = id
	o.CreatedAt = time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO organizations(id, name, created_at) VALUES(?, ?, ?)`,
		o.ID, o.Name, formatTimePtr(o.CreatedAt),
	)
	if err != nil {
		return Organization{}, err
	}
	return o, nil
}

func (s *SQLiteStore) GetOrganization(id string) (Organization, bool) {
	row := s.db.QueryRow(`SELECT id, name, created_at FROM organizations WHERE id = ?`, id)
	var o Organization
	var createdAt string
	if err := row.Scan(&o.ID, &o.Name, &createdAt); err != nil {
		return Organization{}, false
	}
	o.CreatedAt = parseTimePtr(createdAt)
	return o, true
}

func (s *SQLiteStore) ListOrganizations() []Organization {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM organizations ORDER BY created_at ASC`)
	if err != nil {
		return []Organization{}
	}
	defer rows.Close()
	out := []Organization{}
	for rows.Next() {
		var o Organization
		var createdAt string
		if err := rows.Scan(&o.ID, &o.Name, &createdAt); err != nil {
			continue
		}
		o.CreatedAt = parseTimePtr(createdAt)
		out = append(out, o)
	}
	return out
}

func (s *SQLiteStore) CreateUser(username, email, passwordHash, role, orgID string) error {
	_, err := s.db.Exec(
		`INSERT INTO users(username, email, password_hash, role, org_id, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
		username,
		email,
		passwordHash,
		role,
		orgID,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if sqliteUniqueViolation(err) {
			return errors.New("user already exists")
		}
		return err
	}
	return nil
}

func (s *SQLiteStore) GetUser(username string) (User, bool) {
	row := s.db.QueryRow(`SELECT username, email, password_hash, role, org_id, created_at FROM users WHERE username = ?`, username)
	var u User
	var createdAt string
	if err := row.Scan(&u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.OrgID, &createdAt); err != nil {
		return User{}, false
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return u, true
}

func (s *SQLiteStore) ListUsers(orgID string) []User {
	rows, err := s.db.Query(`SELECT username, email, role, org_id, created_at FROM users WHERE org_id = ? ORDER BY username ASC`, orgID)
	if err != nil {
		return []User{}
	}
	defer rows.Close()

	out := []User{}
	for rows.Next() {
		var u User
		var createdAt string
		if err := rows.Scan(&u.Username, &u.Email, &u.Role, &u.OrgID, &createdAt); err != nil {
			continue
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, u)
	}
	return out
}

func (s *SQLiteStore) UpsertBootstrapAdmin(username, passwordHash, orgID string) error {
	_, err := s.db.Exec(
		`INSERT INTO users(username, password_hash, role, org_id, created_at)
		 VALUES(?, ?, 'admin', ?, ?)
		 ON CONFLICT(username) DO UPDATE SET
		   password_hash = excluded.password_hash,
		   role = 'admin'`,
		username,
		passwordHash,
		orgID,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *SQLiteStore) CreateEnrollmentToken(createdBy, orgID string, expiresAt time.Time) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO enrollment_tokens(token, created_by, org_id, expires_at) VALUES(?, ?, ?, ?)`,
		token,
		createdBy,
		orgID,
		expiresAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *SQLiteStore) ConsumeEnrollmentToken(token, deviceID string) (string, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT expires_at, used_at, org_id FROM enrollment_tokens WHERE token = ?`, token)
	var expiresAt, orgID string
	var usedAt sql.NullString
	if err := row.Scan(&expiresAt, &usedAt, &orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}

	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return "", false, err
	}
	if usedAt.Valid || time.Now().UTC().After(expiry) {
		return "", false, nil
	}

	_, err = tx.Exec(
		`UPDATE enrollment_tokens SET used_by = ?, used_at = ? WHERE token = ?`,
		deviceID,
		time.Now().UTC().Format(time.RFC3339Nano),
		token,
	)
	if err != nil {
		return "", false, err
	}

	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return orgID, true, nil
}

func (s *SQLiteStore) UpsertAgentCredential(deviceID, secretHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO agent_credentials(device_id, secret_hash, created_at, updated_at)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET
		   secret_hash = excluded.secret_hash,
		   updated_at = excluded.updated_at`,
		deviceID,
		secretHash,
		time.Now().UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *SQLiteStore) ValidateAgentCredential(deviceID, secret string) bool {
	row := s.db.QueryRow(`SELECT secret_hash FROM agent_credentials WHERE device_id = ?`, deviceID)
	var hash string
	if err := row.Scan(&hash); err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

func (s *SQLiteStore) HasAgentCredential(deviceID string) bool {
	row := s.db.QueryRow(`SELECT 1 FROM agent_credentials WHERE device_id = ? LIMIT 1`, deviceID)
	var v int
	return row.Scan(&v) == nil
}

func (s *SQLiteStore) UpsertDevice(device Device) {
	lastHeartbeat := ""
	if !device.LastHeartbeat.IsZero() {
		lastHeartbeat = device.LastHeartbeat.UTC().Format(time.RFC3339Nano)
	}
	orgID := device.OrgID
	if orgID == "" {
		orgID = DefaultOrgID
	}
	_, _ = s.db.Exec(
		`INSERT INTO devices(id, name, last_heartbeat, connected, org_id)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = CASE WHEN excluded.name = '' THEN devices.name ELSE excluded.name END,
		   last_heartbeat = CASE WHEN excluded.last_heartbeat = '' THEN devices.last_heartbeat ELSE excluded.last_heartbeat END,
		   connected = excluded.connected,
		   org_id = CASE WHEN excluded.org_id = '' THEN devices.org_id ELSE excluded.org_id END`,
		device.ID,
		device.Name,
		lastHeartbeat,
		boolToInt(device.Connected),
		orgID,
	)
}

func (s *SQLiteStore) GetDevice(deviceID string) (Device, bool) {
	row := s.db.QueryRow(`SELECT id, name, last_heartbeat, connected, client_id, group_id, org_id, machine_id_hash, system_id_hash, board_id_hash FROM devices WHERE id = ?`, deviceID)
	var d Device
	var lastHeartbeat string
	var connected int
	if err := row.Scan(&d.ID, &d.Name, &lastHeartbeat, &connected, &d.ClientID, &d.GroupID, &d.OrgID, &d.MachineIDHash, &d.SystemIDHash, &d.BoardIDHash); err != nil {
		return Device{}, false
	}
	if lastHeartbeat != "" {
		d.LastHeartbeat, _ = time.Parse(time.RFC3339Nano, lastHeartbeat)
	}
	d.Connected = connected == 1
	d.CustomFields = s.GetDeviceCustomFields(deviceID)
	return d, true
}

func (s *SQLiteStore) ResolveDeviceIdentity(machineIDHash, systemIDHash, boardIDHash, name string) (Device, error) {
	if countNonEmpty(machineIDHash, systemIDHash, boardIDHash) < 2 {
		return Device{}, errors.New("at least two hardware identifiers are required")
	}
	rows, err := s.db.Query(`SELECT id, name, machine_id_hash, system_id_hash, board_id_hash FROM devices WHERE machine_id_hash = ? OR system_id_hash = ? OR board_id_hash = ?`, machineIDHash, systemIDHash, boardIDHash)
	if err != nil {
		return Device{}, err
	}
	defer rows.Close()

	var match Device
	bestScore := 0
	ambiguous := false
	for rows.Next() {
		var candidate Device
		if err := rows.Scan(&candidate.ID, &candidate.Name, &candidate.MachineIDHash, &candidate.SystemIDHash, &candidate.BoardIDHash); err != nil {
			return Device{}, err
		}
		score := countMatches(candidate.MachineIDHash, machineIDHash, candidate.SystemIDHash, systemIDHash, candidate.BoardIDHash, boardIDHash)
		if score > bestScore {
			match, bestScore, ambiguous = candidate, score, false
		} else if score == bestScore && score >= 2 {
			ambiguous = true
		}
	}
	if err := rows.Err(); err != nil {
		return Device{}, err
	}
	if ambiguous {
		return Device{}, errors.New("hardware identity matches multiple devices")
	}
	if bestScore < 2 {
		id, err := randomToken()
		if err != nil {
			return Device{}, err
		}
		match = Device{ID: "agent-" + id, OrgID: DefaultOrgID}
	}
	if name != "" {
		match.Name = name
	}
	match.MachineIDHash, match.SystemIDHash, match.BoardIDHash = machineIDHash, systemIDHash, boardIDHash
	_, err = s.db.Exec(`INSERT INTO devices(id, name, last_heartbeat, connected, org_id, machine_id_hash, system_id_hash, board_id_hash)
		VALUES(?, ?, '', 0, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = CASE WHEN excluded.name = '' THEN devices.name ELSE excluded.name END, machine_id_hash = excluded.machine_id_hash, system_id_hash = excluded.system_id_hash, board_id_hash = excluded.board_id_hash`, match.ID, match.Name, match.OrgID, match.MachineIDHash, match.SystemIDHash, match.BoardIDHash)
	if err != nil {
		return Device{}, err
	}
	return s.deviceByID(match.ID)
}

func (s *SQLiteStore) deviceByID(deviceID string) (Device, error) {
	d, ok := s.GetDevice(deviceID)
	if !ok {
		return Device{}, errors.New("resolved device was not found")
	}
	return d, nil
}

func countNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func countMatches(leftMachine, rightMachine, leftSystem, rightSystem, leftBoard, rightBoard string) int {
	return boolToInt(leftMachine != "" && leftMachine == rightMachine) + boolToInt(leftSystem != "" && leftSystem == rightSystem) + boolToInt(leftBoard != "" && leftBoard == rightBoard)
}

func (s *SQLiteStore) AssignDeviceClient(deviceID, clientID string) error {
	_, err := s.db.Exec(`UPDATE devices SET client_id = ? WHERE id = ?`, clientID, deviceID)
	return err
}

func (s *SQLiteStore) AssignDeviceGroup(deviceID, groupID string) error {
	_, err := s.db.Exec(`UPDATE devices SET group_id = ? WHERE id = ?`, groupID, deviceID)
	return err
}

func (s *SQLiteStore) SetDeviceConnection(deviceID string, connected bool) {
	heartbeat := ""
	if connected {
		heartbeat = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, _ = s.db.Exec(
		`INSERT INTO devices(id, name, last_heartbeat, connected, org_id)
		 VALUES(?, '', ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   connected = excluded.connected,
		   last_heartbeat = CASE WHEN excluded.last_heartbeat = '' THEN devices.last_heartbeat ELSE excluded.last_heartbeat END`,
		deviceID,
		heartbeat,
		boolToInt(connected),
		DefaultOrgID,
	)
}

func (s *SQLiteStore) ResetDeviceConnections() error {
	_, err := s.db.Exec(`UPDATE devices SET connected = 0`)
	return err
}

func (s *SQLiteStore) ListDevices(orgID string) []Device {
	rows, err := s.db.Query(`SELECT id, name, last_heartbeat, connected, client_id, group_id, org_id FROM devices WHERE org_id = ? ORDER BY id ASC`, orgID)
	if err != nil {
		return []Device{}
	}
	defer rows.Close()

	out := []Device{}
	for rows.Next() {
		var d Device
		var lastHeartbeat string
		var connected int
		if err := rows.Scan(&d.ID, &d.Name, &lastHeartbeat, &connected, &d.ClientID, &d.GroupID, &d.OrgID); err != nil {
			continue
		}
		if lastHeartbeat != "" {
			d.LastHeartbeat, _ = time.Parse(time.RFC3339Nano, lastHeartbeat)
		}
		d.Connected = connected == 1
		out = append(out, d)
	}
	return out
}

func (s *SQLiteStore) DeleteDevice(deviceID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []struct {
		query string
		args  []any
	}{
		{query: `DELETE FROM agent_metric_samples WHERE device_id = ?`, args: []any{deviceID}},
		{query: `DELETE FROM agent_reports WHERE device_id = ?`, args: []any{deviceID}},
		{query: `DELETE FROM agent_credentials WHERE device_id = ?`, args: []any{deviceID}},
		{query: `DELETE FROM devices WHERE id = ?`, args: []any{deviceID}},
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt.query, stmt.args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) UpsertAgentReport(report AgentReport) error {
	if report.ReportedAt.IsZero() {
		report.ReportedAt = time.Now().UTC()
	}
	localIPsJSON, err := json.Marshal(report.LocalIPs)
	if err != nil {
		return err
	}
	startedAt := ""
	if !report.AgentStartedAt.IsZero() {
		startedAt = report.AgentStartedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.Exec(
		`INSERT INTO agent_reports(
			device_id, hostname, username, os, arch, cpu_count, cpu_usage_percent, memory_usage_percent,
			memory_used_bytes, memory_total_bytes, local_ips_json,
			executable_path, working_dir, process_id, agent_started_at, agent_uptime_seconds, reported_at
		 ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET
			hostname = excluded.hostname,
			username = excluded.username,
			os = excluded.os,
			arch = excluded.arch,
			cpu_count = excluded.cpu_count,
			cpu_usage_percent = excluded.cpu_usage_percent,
			memory_usage_percent = excluded.memory_usage_percent,
			memory_used_bytes = excluded.memory_used_bytes,
			memory_total_bytes = excluded.memory_total_bytes,
			local_ips_json = excluded.local_ips_json,
			executable_path = excluded.executable_path,
			working_dir = excluded.working_dir,
			process_id = excluded.process_id,
			agent_started_at = excluded.agent_started_at,
			agent_uptime_seconds = excluded.agent_uptime_seconds,
			reported_at = excluded.reported_at`,
		report.DeviceID,
		report.Hostname,
		report.Username,
		report.OS,
		report.Arch,
		report.CPUCount,
		report.CPUUsagePercent,
		report.MemoryUsagePercent,
		int64(report.MemoryUsedBytes),
		int64(report.MemoryTotalBytes),
		string(localIPsJSON),
		report.ExecutablePath,
		report.WorkingDir,
		report.ProcessID,
		startedAt,
		report.AgentUptimeSeconds,
		report.ReportedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO agent_metric_samples(device_id, sampled_at, cpu_usage_percent, memory_usage_percent, memory_used_bytes, memory_total_bytes)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		report.DeviceID,
		report.ReportedAt.UTC().Format(time.RFC3339Nano),
		report.CPUUsagePercent,
		report.MemoryUsagePercent,
		int64(report.MemoryUsedBytes),
		int64(report.MemoryTotalBytes),
	)
	return err
}

func (s *SQLiteStore) ListAgentReports(orgID string) []AgentReportView {
	rows, err := s.db.Query(
		`SELECT
			d.id, d.name, d.last_heartbeat, d.connected,
			COALESCE(ar.hostname, ''), COALESCE(ar.username, ''), COALESCE(ar.os, ''), COALESCE(ar.arch, ''),
			COALESCE(ar.cpu_count, 0), COALESCE(ar.cpu_usage_percent, 0), COALESCE(ar.memory_usage_percent, 0),
			COALESCE(ar.memory_used_bytes, 0), COALESCE(ar.memory_total_bytes, 0), COALESCE(ar.local_ips_json, '[]'), COALESCE(ar.executable_path, ''),
			COALESCE(ar.working_dir, ''), COALESCE(ar.process_id, 0), COALESCE(ar.agent_started_at, ''),
			COALESCE(ar.agent_uptime_seconds, 0), COALESCE(ar.reported_at, ''),
			CASE WHEN ac.device_id IS NULL THEN 0 ELSE 1 END
		FROM devices d
		LEFT JOIN agent_reports ar ON ar.device_id = d.id
		LEFT JOIN agent_credentials ac ON ac.device_id = d.id
		WHERE d.org_id = ?
		ORDER BY d.id ASC`,
		orgID,
	)
	if err != nil {
		return []AgentReportView{}
	}
	defer rows.Close()

	out := []AgentReportView{}
	for rows.Next() {
		entry := AgentReportView{}
		var lastHeartbeat, localIPsJSON, startedAt, reportedAt string
		var connected, enrolled int
		var memUsedBytes, memTotalBytes int64
		if err := rows.Scan(
			&entry.Device.ID,
			&entry.Device.Name,
			&lastHeartbeat,
			&connected,
			&entry.Report.Hostname,
			&entry.Report.Username,
			&entry.Report.OS,
			&entry.Report.Arch,
			&entry.Report.CPUCount,
			&entry.Report.CPUUsagePercent,
			&entry.Report.MemoryUsagePercent,
			&memUsedBytes,
			&memTotalBytes,
			&localIPsJSON,
			&entry.Report.ExecutablePath,
			&entry.Report.WorkingDir,
			&entry.Report.ProcessID,
			&startedAt,
			&entry.Report.AgentUptimeSeconds,
			&reportedAt,
			&enrolled,
		); err != nil {
			continue
		}
		if lastHeartbeat != "" {
			entry.Device.LastHeartbeat, _ = time.Parse(time.RFC3339Nano, lastHeartbeat)
		}
		entry.Device.Connected = connected == 1
		entry.Enrolled = enrolled == 1
		entry.Report.DeviceID = entry.Device.ID
		entry.Report.MemoryUsedBytes = uint64(memUsedBytes)
		entry.Report.MemoryTotalBytes = uint64(memTotalBytes)
		if startedAt != "" {
			entry.Report.AgentStartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		}
		if reportedAt != "" {
			entry.Report.ReportedAt, _ = time.Parse(time.RFC3339Nano, reportedAt)
		}
		_ = json.Unmarshal([]byte(localIPsJSON), &entry.Report.LocalIPs)
		out = append(out, entry)
	}
	return out
}

func (s *SQLiteStore) GetAgentReport(deviceID string) (AgentReportView, bool) {
	row := s.db.QueryRow(
		`SELECT
			d.id, d.name, d.last_heartbeat, d.connected, d.org_id,
			COALESCE(ar.hostname, ''), COALESCE(ar.username, ''), COALESCE(ar.os, ''), COALESCE(ar.arch, ''),
			COALESCE(ar.cpu_count, 0), COALESCE(ar.cpu_usage_percent, 0), COALESCE(ar.memory_usage_percent, 0),
			COALESCE(ar.memory_used_bytes, 0), COALESCE(ar.memory_total_bytes, 0), COALESCE(ar.local_ips_json, '[]'), COALESCE(ar.executable_path, ''),
			COALESCE(ar.working_dir, ''), COALESCE(ar.process_id, 0), COALESCE(ar.agent_started_at, ''),
			COALESCE(ar.agent_uptime_seconds, 0), COALESCE(ar.reported_at, ''),
			CASE WHEN ac.device_id IS NULL THEN 0 ELSE 1 END
		FROM devices d
		LEFT JOIN agent_reports ar ON ar.device_id = d.id
		LEFT JOIN agent_credentials ac ON ac.device_id = d.id
		WHERE d.id = ?`,
		deviceID,
	)

	entry := AgentReportView{}
	var lastHeartbeat, localIPsJSON, startedAt, reportedAt string
	var connected, enrolled int
	var memUsedBytes, memTotalBytes int64
	if err := row.Scan(
		&entry.Device.ID,
		&entry.Device.Name,
		&lastHeartbeat,
		&connected,
		&entry.Device.OrgID,
		&entry.Report.Hostname,
		&entry.Report.Username,
		&entry.Report.OS,
		&entry.Report.Arch,
		&entry.Report.CPUCount,
		&entry.Report.CPUUsagePercent,
		&entry.Report.MemoryUsagePercent,
		&memUsedBytes,
		&memTotalBytes,
		&localIPsJSON,
		&entry.Report.ExecutablePath,
		&entry.Report.WorkingDir,
		&entry.Report.ProcessID,
		&startedAt,
		&entry.Report.AgentUptimeSeconds,
		&reportedAt,
		&enrolled,
	); err != nil {
		return AgentReportView{}, false
	}
	if lastHeartbeat != "" {
		entry.Device.LastHeartbeat, _ = time.Parse(time.RFC3339Nano, lastHeartbeat)
	}
	entry.Device.Connected = connected == 1
	entry.Enrolled = enrolled == 1
	entry.Report.DeviceID = entry.Device.ID
	entry.Report.MemoryUsedBytes = uint64(memUsedBytes)
	entry.Report.MemoryTotalBytes = uint64(memTotalBytes)
	if startedAt != "" {
		entry.Report.AgentStartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	}
	if reportedAt != "" {
		entry.Report.ReportedAt, _ = time.Parse(time.RFC3339Nano, reportedAt)
	}
	_ = json.Unmarshal([]byte(localIPsJSON), &entry.Report.LocalIPs)
	return entry, true
}

func (s *SQLiteStore) ListAgentMetricSamples(deviceID string, since time.Time, limit int) []AgentMetricSample {
	if limit <= 0 {
		limit = 500
	}
	sinceText := "0001-01-01T00:00:00Z"
	if !since.IsZero() {
		sinceText = since.UTC().Format(time.RFC3339Nano)
	}
	rows, err := s.db.Query(
		`SELECT device_id, sampled_at, cpu_usage_percent, memory_usage_percent, memory_used_bytes, memory_total_bytes
		 FROM agent_metric_samples
		 WHERE device_id = ? AND sampled_at >= ?
		 ORDER BY sampled_at ASC
		 LIMIT ?`,
		deviceID,
		sinceText,
		limit,
	)
	if err != nil {
		return []AgentMetricSample{}
	}
	defer rows.Close()

	out := []AgentMetricSample{}
	for rows.Next() {
		var sample AgentMetricSample
		var sampledAt string
		var memUsed, memTotal int64
		if err := rows.Scan(
			&sample.DeviceID,
			&sampledAt,
			&sample.CPUUsagePercent,
			&sample.MemoryUsagePercent,
			&memUsed,
			&memTotal,
		); err != nil {
			continue
		}
		sample.SampledAt, _ = time.Parse(time.RFC3339Nano, sampledAt)
		sample.MemoryUsedBytes = uint64(memUsed)
		sample.MemoryTotalBytes = uint64(memTotal)
		out = append(out, sample)
	}
	return out
}

func (s *SQLiteStore) AppendAuditEvent(event AuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	orgID := event.OrgID
	if orgID == "" {
		orgID = DefaultOrgID
	}
	_, err := s.db.Exec(
		`INSERT INTO audit_events(action, actor, target, details, created_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		event.Action,
		event.Actor,
		event.Target,
		event.Details,
		event.CreatedAt.UTC().Format(time.RFC3339Nano),
		orgID,
	)
	return err
}

func (s *SQLiteStore) ListAuditEvents(orgID string, limit int) []AuditEvent {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, action, actor, target, details, created_at, org_id
		 FROM audit_events
		 WHERE org_id = ?
		 ORDER BY id DESC
		 LIMIT ?`,
		orgID,
		limit,
	)
	if err != nil {
		return []AuditEvent{}
	}
	defer rows.Close()

	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Target, &e.Details, &createdAt, &e.OrgID); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, e)
	}
	return out
}

func formatTimePtr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTimePtr(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func (s *SQLiteStore) CreateClient(c Client) (Client, error) {
	id, err := randomToken()
	if err != nil {
		return Client{}, err
	}
	c.ID = id
	c.CreatedAt = time.Now().UTC()
	if c.OrgID == "" {
		c.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO clients(id, name, contact_name, contact_email, contact_phone, address, notes, created_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.ContactName, c.ContactEmail, c.ContactPhone, c.Address, c.Notes, formatTimePtr(c.CreatedAt), c.OrgID,
	)
	if err != nil {
		return Client{}, err
	}
	return c, nil
}

func (s *SQLiteStore) UpdateClient(c Client) error {
	_, err := s.db.Exec(
		`UPDATE clients SET name = ?, contact_name = ?, contact_email = ?, contact_phone = ?, address = ?, notes = ? WHERE id = ?`,
		c.Name, c.ContactName, c.ContactEmail, c.ContactPhone, c.Address, c.Notes, c.ID,
	)
	return err
}

func (s *SQLiteStore) GetClient(id string) (Client, bool) {
	row := s.db.QueryRow(
		`SELECT id, name, contact_name, contact_email, contact_phone, address, notes, created_at, org_id FROM clients WHERE id = ?`,
		id,
	)
	var c Client
	var createdAt string
	if err := row.Scan(&c.ID, &c.Name, &c.ContactName, &c.ContactEmail, &c.ContactPhone, &c.Address, &c.Notes, &createdAt, &c.OrgID); err != nil {
		return Client{}, false
	}
	c.CreatedAt = parseTimePtr(createdAt)
	return c, true
}

func (s *SQLiteStore) ListClients(orgID string) []Client {
	rows, err := s.db.Query(`SELECT id, name, contact_name, contact_email, contact_phone, address, notes, created_at, org_id FROM clients WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return []Client{}
	}
	defer rows.Close()
	out := []Client{}
	for rows.Next() {
		var c Client
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Name, &c.ContactName, &c.ContactEmail, &c.ContactPhone, &c.Address, &c.Notes, &createdAt, &c.OrgID); err != nil {
			continue
		}
		c.CreatedAt = parseTimePtr(createdAt)
		out = append(out, c)
	}
	return out
}

func (s *SQLiteStore) DeleteClient(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE devices SET client_id = '' WHERE client_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM clients WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) CreateContract(c Contract) (Contract, error) {
	id, err := randomToken()
	if err != nil {
		return Contract{}, err
	}
	c.ID = id
	c.CreatedAt = time.Now().UTC()
	if c.Status == "" {
		c.Status = "active"
	}
	if c.OrgID == "" {
		c.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO contracts(id, client_id, name, contract_type, rate_type, rate_amount, billing_cycle, start_date, end_date, status, notes, last_invoiced_at, created_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ClientID, c.Name, c.ContractType, c.RateType, c.RateAmount, c.BillingCycle,
		formatTimePtr(c.StartDate), formatTimePtr(c.EndDate), c.Status, c.Notes, formatTimePtr(c.LastInvoicedAt), formatTimePtr(c.CreatedAt), c.OrgID,
	)
	if err != nil {
		return Contract{}, err
	}
	return c, nil
}

func (s *SQLiteStore) UpdateContract(c Contract) error {
	_, err := s.db.Exec(
		`UPDATE contracts SET name = ?, contract_type = ?, rate_type = ?, rate_amount = ?, billing_cycle = ?, start_date = ?, end_date = ?, status = ?, notes = ? WHERE id = ?`,
		c.Name, c.ContractType, c.RateType, c.RateAmount, c.BillingCycle, formatTimePtr(c.StartDate), formatTimePtr(c.EndDate), c.Status, c.Notes, c.ID,
	)
	return err
}

func (s *SQLiteStore) SetContractLastInvoiced(id string, when time.Time) error {
	_, err := s.db.Exec(`UPDATE contracts SET last_invoiced_at = ? WHERE id = ?`, formatTimePtr(when), id)
	return err
}

func scanContract(row interface{ Scan(...any) error }) (Contract, error) {
	var c Contract
	var startDate, endDate, createdAt, lastInvoicedAt string
	if err := row.Scan(&c.ID, &c.ClientID, &c.Name, &c.ContractType, &c.RateType, &c.RateAmount, &c.BillingCycle, &startDate, &endDate, &c.Status, &c.Notes, &lastInvoicedAt, &createdAt, &c.OrgID); err != nil {
		return Contract{}, err
	}
	c.StartDate = parseTimePtr(startDate)
	c.EndDate = parseTimePtr(endDate)
	c.LastInvoicedAt = parseTimePtr(lastInvoicedAt)
	c.CreatedAt = parseTimePtr(createdAt)
	return c, nil
}

func (s *SQLiteStore) GetContract(id string) (Contract, bool) {
	row := s.db.QueryRow(
		`SELECT id, client_id, name, contract_type, rate_type, rate_amount, billing_cycle, start_date, end_date, status, notes, last_invoiced_at, created_at, org_id FROM contracts WHERE id = ?`,
		id,
	)
	c, err := scanContract(row)
	if err != nil {
		return Contract{}, false
	}
	return c, true
}

func (s *SQLiteStore) ListContracts(orgID, clientID string) []Contract {
	query := `SELECT id, client_id, name, contract_type, rate_type, rate_amount, billing_cycle, start_date, end_date, status, notes, last_invoiced_at, created_at, org_id FROM contracts WHERE org_id = ?`
	args := []any{orgID}
	if clientID != "" {
		query += ` AND client_id = ?`
		args = append(args, clientID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []Contract{}
	}
	defer rows.Close()
	out := []Contract{}
	for rows.Next() {
		c, err := scanContract(rows)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (s *SQLiteStore) DeleteContract(id string) error {
	_, err := s.db.Exec(`DELETE FROM contracts WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateTicket(t Ticket) (Ticket, error) {
	id, err := randomToken()
	if err != nil {
		return Ticket{}, err
	}
	t.ID = id
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = "open"
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	if t.OrgID == "" {
		t.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO tickets(id, client_id, device_id, subject, description, status, priority, assignee, created_by, created_at, updated_at, resolved_at, approved_by, approved_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ClientID, t.DeviceID, t.Subject, t.Description, t.Status, t.Priority, t.Assignee, t.CreatedBy,
		formatTimePtr(t.CreatedAt), formatTimePtr(t.UpdatedAt), formatTimePtr(t.ResolvedAt), t.ApprovedBy, formatTimePtr(t.ApprovedAt), t.OrgID,
	)
	if err != nil {
		return Ticket{}, err
	}
	return t, nil
}

func (s *SQLiteStore) UpdateTicket(t Ticket) error {
	t.UpdatedAt = time.Now().UTC()
	if (t.Status == "resolved" || t.Status == "closed") && t.ResolvedAt.IsZero() {
		t.ResolvedAt = t.UpdatedAt
	}
	_, err := s.db.Exec(
		`UPDATE tickets SET client_id = ?, device_id = ?, subject = ?, description = ?, status = ?, priority = ?, assignee = ?, updated_at = ?, resolved_at = ?, approved_by = ?, approved_at = ? WHERE id = ?`,
		t.ClientID, t.DeviceID, t.Subject, t.Description, t.Status, t.Priority, t.Assignee, formatTimePtr(t.UpdatedAt), formatTimePtr(t.ResolvedAt), t.ApprovedBy, formatTimePtr(t.ApprovedAt), t.ID,
	)
	return err
}

func scanTicket(row interface{ Scan(...any) error }) (Ticket, error) {
	var t Ticket
	var createdAt, updatedAt, resolvedAt, approvedAt string
	if err := row.Scan(&t.ID, &t.ClientID, &t.DeviceID, &t.Subject, &t.Description, &t.Status, &t.Priority, &t.Assignee, &t.CreatedBy, &createdAt, &updatedAt, &resolvedAt, &t.ApprovedBy, &approvedAt, &t.OrgID); err != nil {
		return Ticket{}, err
	}
	t.CreatedAt = parseTimePtr(createdAt)
	t.UpdatedAt = parseTimePtr(updatedAt)
	t.ResolvedAt = parseTimePtr(resolvedAt)
	t.ApprovedAt = parseTimePtr(approvedAt)
	return t, nil
}

func (s *SQLiteStore) GetTicket(id string) (Ticket, bool) {
	row := s.db.QueryRow(
		`SELECT id, client_id, device_id, subject, description, status, priority, assignee, created_by, created_at, updated_at, resolved_at, approved_by, approved_at, org_id FROM tickets WHERE id = ?`,
		id,
	)
	t, err := scanTicket(row)
	if err != nil {
		return Ticket{}, false
	}
	return t, true
}

func (s *SQLiteStore) ListTickets(orgID, clientID string) []Ticket {
	query := `SELECT id, client_id, device_id, subject, description, status, priority, assignee, created_by, created_at, updated_at, resolved_at, approved_by, approved_at, org_id FROM tickets WHERE org_id = ?`
	args := []any{orgID}
	if clientID != "" {
		query += ` AND client_id = ?`
		args = append(args, clientID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []Ticket{}
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *SQLiteStore) DeleteTicket(id string) error {
	_, err := s.db.Exec(`DELETE FROM tickets WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateInvoice(inv Invoice) (Invoice, error) {
	id, err := randomToken()
	if err != nil {
		return Invoice{}, err
	}
	inv.ID = id
	inv.CreatedAt = time.Now().UTC()
	if inv.Status == "" {
		inv.Status = "draft"
	}
	if inv.InvoiceNumber == "" {
		inv.InvoiceNumber = "INV-" + inv.ID[:8]
	}
	computeInvoiceTotals(&inv)
	lineItemsJSON, err := json.Marshal(inv.LineItems)
	if err != nil {
		return Invoice{}, err
	}
	if inv.OrgID == "" {
		inv.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO invoices(id, client_id, contract_id, invoice_number, status, issue_date, due_date, line_items_json, subtotal, tax, total, notes, created_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.ClientID, inv.ContractID, inv.InvoiceNumber, inv.Status, formatTimePtr(inv.IssueDate), formatTimePtr(inv.DueDate),
		string(lineItemsJSON), inv.Subtotal, inv.Tax, inv.Total, inv.Notes, formatTimePtr(inv.CreatedAt), inv.OrgID,
	)
	if err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

func computeInvoiceTotals(inv *Invoice) {
	subtotal := 0.0
	for i := range inv.LineItems {
		inv.LineItems[i].Amount = inv.LineItems[i].Quantity * inv.LineItems[i].UnitPrice
		subtotal += inv.LineItems[i].Amount
	}
	inv.Subtotal = subtotal
	inv.Total = subtotal + inv.Tax
}

func (s *SQLiteStore) UpdateInvoice(inv Invoice) error {
	computeInvoiceTotals(&inv)
	lineItemsJSON, err := json.Marshal(inv.LineItems)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE invoices SET status = ?, issue_date = ?, due_date = ?, line_items_json = ?, subtotal = ?, tax = ?, total = ?, notes = ? WHERE id = ?`,
		inv.Status, formatTimePtr(inv.IssueDate), formatTimePtr(inv.DueDate), string(lineItemsJSON), inv.Subtotal, inv.Tax, inv.Total, inv.Notes, inv.ID,
	)
	return err
}

func scanInvoice(row interface{ Scan(...any) error }) (Invoice, error) {
	var inv Invoice
	var issueDate, dueDate, createdAt, lineItemsJSON string
	if err := row.Scan(&inv.ID, &inv.ClientID, &inv.ContractID, &inv.InvoiceNumber, &inv.Status, &issueDate, &dueDate, &lineItemsJSON, &inv.Subtotal, &inv.Tax, &inv.Total, &inv.Notes, &createdAt, &inv.OrgID); err != nil {
		return Invoice{}, err
	}
	inv.IssueDate = parseTimePtr(issueDate)
	inv.DueDate = parseTimePtr(dueDate)
	inv.CreatedAt = parseTimePtr(createdAt)
	_ = json.Unmarshal([]byte(lineItemsJSON), &inv.LineItems)
	return inv, nil
}

func (s *SQLiteStore) GetInvoice(id string) (Invoice, bool) {
	row := s.db.QueryRow(
		`SELECT id, client_id, contract_id, invoice_number, status, issue_date, due_date, line_items_json, subtotal, tax, total, notes, created_at, org_id FROM invoices WHERE id = ?`,
		id,
	)
	inv, err := scanInvoice(row)
	if err != nil {
		return Invoice{}, false
	}
	return inv, true
}

func (s *SQLiteStore) ListInvoices(orgID, clientID string) []Invoice {
	query := `SELECT id, client_id, contract_id, invoice_number, status, issue_date, due_date, line_items_json, subtotal, tax, total, notes, created_at, org_id FROM invoices WHERE org_id = ?`
	args := []any{orgID}
	if clientID != "" {
		query += ` AND client_id = ?`
		args = append(args, clientID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []Invoice{}
	}
	defer rows.Close()
	out := []Invoice{}
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			continue
		}
		out = append(out, inv)
	}
	return out
}

func (s *SQLiteStore) DeleteInvoice(id string) error {
	_, err := s.db.Exec(`DELETE FROM invoices WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateDeviceGroup(g DeviceGroup) (DeviceGroup, error) {
	id, err := randomToken()
	if err != nil {
		return DeviceGroup{}, err
	}
	g.ID = id
	g.CreatedAt = time.Now().UTC()
	if g.OrgID == "" {
		g.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO device_groups(id, name, notes, created_at, org_id) VALUES(?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Notes, formatTimePtr(g.CreatedAt), g.OrgID,
	)
	if err != nil {
		return DeviceGroup{}, err
	}
	return g, nil
}

func (s *SQLiteStore) ListDeviceGroups(orgID string) []DeviceGroup {
	rows, err := s.db.Query(`SELECT id, name, notes, created_at, org_id FROM device_groups WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return []DeviceGroup{}
	}
	defer rows.Close()
	out := []DeviceGroup{}
	for rows.Next() {
		var g DeviceGroup
		var createdAt string
		if err := rows.Scan(&g.ID, &g.Name, &g.Notes, &createdAt, &g.OrgID); err != nil {
			continue
		}
		g.CreatedAt = parseTimePtr(createdAt)
		out = append(out, g)
	}
	return out
}

func (s *SQLiteStore) DeleteDeviceGroup(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE devices SET group_id = '' WHERE group_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM device_groups WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) CreateScript(sc Script) (Script, error) {
	id, err := randomToken()
	if err != nil {
		return Script{}, err
	}
	sc.ID = id
	sc.CreatedAt = time.Now().UTC()
	if sc.TargetOS == "" {
		sc.TargetOS = "any"
	}
	if sc.OrgID == "" {
		sc.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO scripts(id, name, description, target_os, body, created_by, created_at, org_id) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.Name, sc.Description, sc.TargetOS, sc.Body, sc.CreatedBy, formatTimePtr(sc.CreatedAt), sc.OrgID,
	)
	if err != nil {
		return Script{}, err
	}
	return sc, nil
}

func (s *SQLiteStore) GetScript(id string) (Script, bool) {
	row := s.db.QueryRow(`SELECT id, name, description, target_os, body, created_by, created_at, org_id FROM scripts WHERE id = ?`, id)
	var sc Script
	var createdAt string
	if err := row.Scan(&sc.ID, &sc.Name, &sc.Description, &sc.TargetOS, &sc.Body, &sc.CreatedBy, &createdAt, &sc.OrgID); err != nil {
		return Script{}, false
	}
	sc.CreatedAt = parseTimePtr(createdAt)
	return sc, true
}

func (s *SQLiteStore) ListScripts(orgID string) []Script {
	rows, err := s.db.Query(`SELECT id, name, description, target_os, body, created_by, created_at, org_id FROM scripts WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return []Script{}
	}
	defer rows.Close()
	out := []Script{}
	for rows.Next() {
		var sc Script
		var createdAt string
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Description, &sc.TargetOS, &sc.Body, &sc.CreatedBy, &createdAt, &sc.OrgID); err != nil {
			continue
		}
		sc.CreatedAt = parseTimePtr(createdAt)
		out = append(out, sc)
	}
	return out
}

func (s *SQLiteStore) DeleteScript(id string) error {
	_, err := s.db.Exec(`DELETE FROM scripts WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateAlertRule(rule AlertRule) (AlertRule, error) {
	id, err := randomToken()
	if err != nil {
		return AlertRule{}, err
	}
	rule.ID = id
	rule.CreatedAt = time.Now().UTC()
	if rule.Comparator == "" {
		rule.Comparator = "gt"
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.OrgID == "" {
		rule.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO alert_rules(id, name, metric_type, comparator, threshold_value, client_id, device_id, severity, enabled, created_by, created_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.MetricType, rule.Comparator, rule.ThresholdValue, rule.ClientID, rule.DeviceID, rule.Severity, boolToInt(rule.Enabled), rule.CreatedBy, formatTimePtr(rule.CreatedAt), rule.OrgID,
	)
	if err != nil {
		return AlertRule{}, err
	}
	return rule, nil
}

func (s *SQLiteStore) UpdateAlertRule(rule AlertRule) error {
	_, err := s.db.Exec(
		`UPDATE alert_rules SET name = ?, metric_type = ?, comparator = ?, threshold_value = ?, client_id = ?, device_id = ?, severity = ?, enabled = ? WHERE id = ?`,
		rule.Name, rule.MetricType, rule.Comparator, rule.ThresholdValue, rule.ClientID, rule.DeviceID, rule.Severity, boolToInt(rule.Enabled), rule.ID,
	)
	return err
}

func (s *SQLiteStore) ListAlertRules(orgID string) []AlertRule {
	rows, err := s.db.Query(`SELECT id, name, metric_type, comparator, threshold_value, client_id, device_id, severity, enabled, created_by, created_at, org_id FROM alert_rules WHERE org_id = ? ORDER BY created_at DESC`, orgID)
	if err != nil {
		return []AlertRule{}
	}
	defer rows.Close()
	out := []AlertRule{}
	for rows.Next() {
		var rule AlertRule
		var enabled int
		var createdAt string
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.MetricType, &rule.Comparator, &rule.ThresholdValue, &rule.ClientID, &rule.DeviceID, &rule.Severity, &enabled, &rule.CreatedBy, &createdAt, &rule.OrgID); err != nil {
			continue
		}
		rule.Enabled = enabled == 1
		rule.CreatedAt = parseTimePtr(createdAt)
		out = append(out, rule)
	}
	return out
}

func (s *SQLiteStore) DeleteAlertRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM alert_rules WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateAlert(a Alert) (Alert, error) {
	id, err := randomToken()
	if err != nil {
		return Alert{}, err
	}
	a.ID = id
	if a.TriggeredAt.IsZero() {
		a.TriggeredAt = time.Now().UTC()
	}
	if a.Status == "" {
		a.Status = "open"
	}
	if a.OrgID == "" {
		a.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO alerts(id, rule_id, rule_name, device_id, metric_type, message, severity, status, value, triggered_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RuleID, a.RuleName, a.DeviceID, a.MetricType, a.Message, a.Severity, a.Status, a.Value, formatTimePtr(a.TriggeredAt), a.OrgID,
	)
	if err != nil {
		return Alert{}, err
	}
	return a, nil
}

func scanAlert(row interface{ Scan(...any) error }) (Alert, error) {
	var a Alert
	var triggeredAt string
	var acknowledgedAt, resolvedAt sql.NullString
	if err := row.Scan(&a.ID, &a.RuleID, &a.RuleName, &a.DeviceID, &a.MetricType, &a.Message, &a.Severity, &a.Status, &a.Value, &triggeredAt, &acknowledgedAt, &resolvedAt, &a.OrgID); err != nil {
		return Alert{}, err
	}
	a.TriggeredAt = parseTimePtr(triggeredAt)
	a.AcknowledgedAt = parseTimePtr(acknowledgedAt.String)
	a.ResolvedAt = parseTimePtr(resolvedAt.String)
	return a, nil
}

func (s *SQLiteStore) GetOpenAlert(ruleID, deviceID string) (Alert, bool) {
	row := s.db.QueryRow(
		`SELECT id, rule_id, rule_name, device_id, metric_type, message, severity, status, value, triggered_at, acknowledged_at, resolved_at, org_id
		 FROM alerts WHERE rule_id = ? AND device_id = ? AND status != 'resolved' ORDER BY triggered_at DESC LIMIT 1`,
		ruleID, deviceID,
	)
	a, err := scanAlert(row)
	if err != nil {
		return Alert{}, false
	}
	return a, true
}

func (s *SQLiteStore) ListAlerts(orgID, status string) []Alert {
	query := `SELECT id, rule_id, rule_name, device_id, metric_type, message, severity, status, value, triggered_at, acknowledged_at, resolved_at, org_id FROM alerts WHERE org_id = ?`
	args := []any{orgID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY triggered_at DESC LIMIT 500`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []Alert{}
	}
	defer rows.Close()
	out := []Alert{}
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			continue
		}
		out = append(out, a)
	}
	return out
}

func (s *SQLiteStore) AcknowledgeAlert(id string) error {
	_, err := s.db.Exec(
		`UPDATE alerts SET status = 'acknowledged', acknowledged_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	)
	return err
}

func (s *SQLiteStore) ResolveAlert(id string) error {
	_, err := s.db.Exec(
		`UPDATE alerts SET status = 'resolved', resolved_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	)
	return err
}

func (s *SQLiteStore) DeleteAlert(id string) error {
	_, err := s.db.Exec(`DELETE FROM alerts WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateTimeEntry(te TimeEntry) (TimeEntry, error) {
	id, err := randomToken()
	if err != nil {
		return TimeEntry{}, err
	}
	te.ID = id
	te.CreatedAt = time.Now().UTC()
	if te.OrgID == "" {
		te.OrgID = DefaultOrgID
	}
	_, err = s.db.Exec(
		`INSERT INTO time_entries(id, client_id, ticket_id, description, minutes, billable, invoice_id, created_by, created_at, org_id)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		te.ID, te.ClientID, te.TicketID, te.Description, te.Minutes, boolToInt(te.Billable), te.InvoiceID, te.CreatedBy, formatTimePtr(te.CreatedAt), te.OrgID,
	)
	if err != nil {
		return TimeEntry{}, err
	}
	return te, nil
}

func (s *SQLiteStore) ListTimeEntries(orgID, clientID string) []TimeEntry {
	query := `SELECT id, client_id, ticket_id, description, minutes, billable, invoice_id, created_by, created_at, org_id FROM time_entries WHERE org_id = ?`
	args := []any{orgID}
	if clientID != "" {
		query += ` AND client_id = ?`
		args = append(args, clientID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []TimeEntry{}
	}
	defer rows.Close()
	out := []TimeEntry{}
	for rows.Next() {
		var te TimeEntry
		var createdAt string
		var billable int
		if err := rows.Scan(&te.ID, &te.ClientID, &te.TicketID, &te.Description, &te.Minutes, &billable, &te.InvoiceID, &te.CreatedBy, &createdAt, &te.OrgID); err != nil {
			continue
		}
		te.Billable = billable == 1
		te.CreatedAt = parseTimePtr(createdAt)
		out = append(out, te)
	}
	return out
}

func (s *SQLiteStore) DeleteTimeEntry(id string) error {
	_, err := s.db.Exec(`DELETE FROM time_entries WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) SaveBranding(b Branding) error {
	b.UpdatedAt = time.Now().UTC()
	if b.OrgID == "" {
		b.OrgID = DefaultOrgID
	}
	_, err := s.db.Exec(
		`INSERT INTO branding(org_id, company_name, logo, icon, phone_number, website, email, logo_path, icon_path, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(org_id) DO UPDATE SET
		   company_name = excluded.company_name,
		   logo = excluded.logo,
		   icon = excluded.icon,
		   phone_number = excluded.phone_number,
		   website = excluded.website,
		   email = excluded.email,
		   logo_path = excluded.logo_path,
		   icon_path = excluded.icon_path,
		   updated_at = excluded.updated_at`,
		b.OrgID, b.CompanyName, b.Logo, b.Icon, b.PhoneNumber, b.Website, b.Email, b.LogoPath, b.IconPath, formatTimePtr(b.UpdatedAt),
	)
	return err
}

func (s *SQLiteStore) GetBranding(orgID string) (Branding, error) {
	if orgID == "" {
		orgID = DefaultOrgID
	}
	row := s.db.QueryRow(
		`SELECT org_id, company_name, logo, icon, phone_number, website, email, logo_path, icon_path, updated_at FROM branding WHERE org_id = ?`,
		orgID,
	)
	var b Branding
	var updatedAt string
	if err := row.Scan(&b.OrgID, &b.CompanyName, &b.Logo, &b.Icon, &b.PhoneNumber, &b.Website, &b.Email, &b.LogoPath, &b.IconPath, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Branding{OrgID: orgID}, nil
		}
		return Branding{}, err
	}
	b.UpdatedAt = parseTimePtr(updatedAt)
	return b, nil
}

func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query app_settings: %w", err)
	}
	return value, nil
}

func (s *SQLiteStore) SaveSetting(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)",
		key, value, now,
	)
	if err != nil {
		return fmt.Errorf("insert/update app_settings: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SetSetting(key, value string) error {
	return s.SaveSetting(key, value)
}

func (s *SQLiteStore) ListUnbilledTimeEntries(orgID, clientID string) []TimeEntry {
	query := `SELECT id, client_id, ticket_id, description, minutes, billable, invoice_id, created_by, created_at, org_id 
	          FROM time_entries 
	          WHERE org_id = ? AND client_id = ? AND invoice_id = '' AND billable = 1
	          ORDER BY created_at DESC`

	rows, err := s.db.Query(query, orgID, clientID)
	if err != nil {
		return []TimeEntry{}
	}
	defer rows.Close()

	var entries []TimeEntry
	for rows.Next() {
		var te TimeEntry
		var createdAt string
		if err := rows.Scan(&te.ID, &te.ClientID, &te.TicketID, &te.Description, &te.Minutes, &te.Billable, &te.InvoiceID, &te.CreatedBy, &createdAt, &te.OrgID); err != nil {
			continue
		}
		te.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		entries = append(entries, te)
	}
	return entries
}

func (s *SQLiteStore) MarkTimeEntriesInvoiced(ids []string, invoiceID string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = invoiceID

	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := "UPDATE time_entries SET invoice_id = ? WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("mark time entries invoiced: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ResolveOpenAlertsForDevice(deviceID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		"UPDATE alerts SET status = 'resolved', resolved_at = ? WHERE device_id = ? AND status != 'resolved'",
		now, deviceID,
	)
	return err
}

// Custom fields management
func (s *SQLiteStore) ListCustomFieldDefinitions(orgID string) []string {
	rows, err := s.db.Query("SELECT field_name FROM custom_field_definitions WHERE org_id = ? ORDER BY field_name", orgID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var fields []string
	for rows.Next() {
		var fieldName string
		if err := rows.Scan(&fieldName); err != nil {
			continue
		}
		fields = append(fields, fieldName)
	}
	return fields
}

func (s *SQLiteStore) SaveCustomFieldDefinition(orgID, fieldName string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO custom_field_definitions (org_id, field_name, created_at) VALUES (?, ?, ?)",
		orgID, fieldName, now,
	)
	if err != nil {
		return fmt.Errorf("save custom field definition: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteCustomFieldDefinition(orgID, fieldName string) error {
	// Delete the definition and all values for this field across all devices
	_, err := s.db.Exec("DELETE FROM custom_field_definitions WHERE org_id = ? AND field_name = ?", orgID, fieldName)
	if err != nil {
		return fmt.Errorf("delete custom field definition: %w", err)
	}
	// Also clean up values, but need to ensure only this org's devices
	_, err = s.db.Exec(`
		DELETE FROM device_custom_fields 
		WHERE field_name = ? AND device_id IN (
			SELECT id FROM devices WHERE org_id = ?
		)`,
		fieldName, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete custom field values: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetDeviceCustomFields(deviceID string) map[string]string {
	rows, err := s.db.Query("SELECT field_name, field_value FROM device_custom_fields WHERE device_id = ?", deviceID)
	if err != nil {
		return make(map[string]string)
	}
	defer rows.Close()

	fields := make(map[string]string)
	for rows.Next() {
		var fieldName, fieldValue string
		if err := rows.Scan(&fieldName, &fieldValue); err != nil {
			continue
		}
		fields[fieldName] = fieldValue
	}
	return fields
}

func (s *SQLiteStore) UpdateDeviceCustomFields(deviceID string, fields map[string]string) error {
	// Delete existing values
	_, err := s.db.Exec("DELETE FROM device_custom_fields WHERE device_id = ?", deviceID)
	if err != nil {
		return fmt.Errorf("clear device custom fields: %w", err)
	}

	// Insert new values
	for fieldName, fieldValue := range fields {
		_, err := s.db.Exec(
			"INSERT INTO device_custom_fields (device_id, field_name, field_value) VALUES (?, ?, ?)",
			deviceID, fieldName, fieldValue,
		)
		if err != nil {
			return fmt.Errorf("insert device custom field: %w", err)
		}
	}
	return nil
}

func sqliteUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
