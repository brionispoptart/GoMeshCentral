package hub

import (
	"path/filepath"
	"testing"

	"gomeshcentral/internal/storage"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store)
}

func TestEvaluateReportAlertsTriggersAndResolves(t *testing.T) {
	h := newTestHub(t)
	h.store.UpsertDevice(storage.Device{ID: "dev-1", Name: "Test Device"})

	rule, err := h.store.CreateAlertRule(storage.AlertRule{
		Name:           "High CPU",
		MetricType:     "cpu",
		Comparator:     "gt",
		ThresholdValue: 80,
		Severity:       "critical",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create alert rule: %v", err)
	}

	// Breach: should open exactly one alert.
	h.evaluateReportAlerts("dev-1", storage.AgentReport{CPUUsagePercent: 95})
	open := h.store.ListAlerts(storage.DefaultOrgID, "open")
	if len(open) != 1 {
		t.Fatalf("expected 1 open alert after breach, got %d", len(open))
	}
	if open[0].RuleID != rule.ID || open[0].DeviceID != "dev-1" {
		t.Fatalf("unexpected alert: %+v", open[0])
	}

	// Repeated breach should not create a duplicate alert.
	h.evaluateReportAlerts("dev-1", storage.AgentReport{CPUUsagePercent: 96})
	if got := len(h.store.ListAlerts(storage.DefaultOrgID, "open")); got != 1 {
		t.Fatalf("expected still 1 open alert after repeated breach, got %d", got)
	}

	// Recovery: should auto-resolve the open alert.
	h.evaluateReportAlerts("dev-1", storage.AgentReport{CPUUsagePercent: 10})
	if got := len(h.store.ListAlerts(storage.DefaultOrgID, "open")); got != 0 {
		t.Fatalf("expected 0 open alerts after resolve, got %d", got)
	}
	resolved := h.store.ListAlerts(storage.DefaultOrgID, "resolved")
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved alert, got %d", len(resolved))
	}
}

func TestOfflineAlertsTriggerAndResolveOnReconnect(t *testing.T) {
	h := newTestHub(t)
	h.store.UpsertDevice(storage.Device{ID: "dev-2", Name: "Test Device 2"})

	if _, err := h.store.CreateAlertRule(storage.AlertRule{
		Name:       "Offline",
		MetricType: "offline",
		Severity:   "warning",
		Enabled:    true,
	}); err != nil {
		t.Fatalf("create alert rule: %v", err)
	}

	h.triggerOfflineAlerts("dev-2")
	open := h.store.ListAlerts(storage.DefaultOrgID, "open")
	if len(open) != 1 {
		t.Fatalf("expected 1 open offline alert, got %d", len(open))
	}

	h.resolveOfflineAlerts("dev-2")
	if got := len(h.store.ListAlerts(storage.DefaultOrgID, "open")); got != 0 {
		t.Fatalf("expected 0 open alerts after reconnect, got %d", got)
	}
}
