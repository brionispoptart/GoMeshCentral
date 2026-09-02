package hub

import (
	"fmt"

	"gomeshcentral/internal/storage"
)

// ruleMatchesDevice reports whether an alert rule's scope (device/client) applies
// to the given device. Empty scope fields mean "all devices".
func ruleMatchesDevice(rule storage.AlertRule, device storage.Device) bool {
	if rule.DeviceID != "" && rule.DeviceID != device.ID {
		return false
	}
	if rule.ClientID != "" && rule.ClientID != device.ClientID {
		return false
	}
	return true
}

func breachesThreshold(rule storage.AlertRule, value float64) bool {
	if rule.Comparator == "lt" {
		return value < rule.ThresholdValue
	}
	return value > rule.ThresholdValue
}

// evaluateReportAlerts checks CPU/memory metric rules against a freshly ingested
// agent report, opening or auto-resolving alerts as thresholds are crossed.
func (h *Hub) evaluateReportAlerts(deviceID string, report storage.AgentReport) {
	device, ok := h.findDevice(deviceID)
	if !ok {
		return
	}
	rules := h.store.ListAlertRules(device.OrgID)
	for _, rule := range rules {
		if !rule.Enabled || rule.MetricType == "offline" {
			continue
		}
		if !ruleMatchesDevice(rule, device) {
			continue
		}
		var value float64
		switch rule.MetricType {
		case "cpu":
			value = report.CPUUsagePercent
		case "memory":
			value = report.MemoryUsagePercent
		default:
			continue
		}
		h.applyMetricRule(rule, deviceID, value)
	}
}

func (h *Hub) applyMetricRule(rule storage.AlertRule, deviceID string, value float64) {
	breached := breachesThreshold(rule, value)
	existing, hasOpen := h.store.GetOpenAlert(rule.ID, deviceID)

	if breached {
		if hasOpen {
			return
		}
		alert, _ := h.store.CreateAlert(storage.Alert{
			OrgID:      rule.OrgID,
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			DeviceID:   deviceID,
			MetricType: rule.MetricType,
			Severity:   rule.Severity,
			Value:      value,
			Message:    fmt.Sprintf("%s %s %.1f (threshold %.1f)", rule.MetricType, comparatorWord(rule.Comparator), value, rule.ThresholdValue),
		})
		// Invoke notification callback if set
		if callback := h.GetOnAlertCreated(); callback != nil {
			_ = callback(alert)
		}
		h.broadcastAlertsChanged()
		return
	}

	if hasOpen && existing.Status != "resolved" {
		_ = h.store.ResolveAlert(existing.ID)
		h.broadcastAlertsChanged()
	}
}

func comparatorWord(comparator string) string {
	if comparator == "lt" {
		return "below"
	}
	return "above"
}

// triggerOfflineAlerts opens alerts for any enabled "offline" rule matching this
// device. Called when a device's connection is torn down.
func (h *Hub) triggerOfflineAlerts(deviceID string) {
	device, ok := h.findDevice(deviceID)
	if !ok {
		return
	}
	rules := h.store.ListAlertRules(device.OrgID)
	changed := false
	for _, rule := range rules {
		if !rule.Enabled || rule.MetricType != "offline" {
			continue
		}
		if !ruleMatchesDevice(rule, device) {
			continue
		}
		if _, hasOpen := h.store.GetOpenAlert(rule.ID, deviceID); hasOpen {
			continue
		}
		_, _ = h.store.CreateAlert(storage.Alert{
			OrgID:      rule.OrgID,
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			DeviceID:   deviceID,
			MetricType: "offline",
			Severity:   rule.Severity,
			Message:    "device went offline",
		})
		changed = true
	}
	if changed {
		h.broadcastAlertsChanged()
	}
}

// resolveOfflineAlerts auto-resolves any open "offline" alerts once a device
// reconnects.
func (h *Hub) resolveOfflineAlerts(deviceID string) {
	if err := h.store.ResolveOpenAlertsForDevice(deviceID); err == nil {
		h.broadcastAlertsChanged()
	}
}

func (h *Hub) findDevice(deviceID string) (storage.Device, bool) {
	return h.store.GetDevice(deviceID)
}
