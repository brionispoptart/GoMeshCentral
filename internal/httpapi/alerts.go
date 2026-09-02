package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/storage"
)

func (s *Server) handleAlertRulesCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		clientID, valid := s.validClientScope(w, r, claims.OrgID)
		if !valid {
			return
		}
		rules := s.store.ListAlertRules(claims.OrgID)
		if clientID != "" {
			filtered := make([]storage.AlertRule, 0, len(rules))
			for _, rule := range rules {
				if rule.ClientID == clientID {
					filtered = append(filtered, rule)
				}
			}
			rules = filtered
		}
		respondJSON(w, rules)
	case http.MethodPost:
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.AlertRule
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.MetricType) == "" {
			http.Error(w, "name and metricType are required", http.StatusBadRequest)
			return
		}
		if body.ClientID != "" && !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.Enabled = true
		body.CreatedBy = claims.Subject
		body.OrgID = claims.OrgID
		created, err := s.store.CreateAlertRule(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "alert_rule_created", Actor: claims.Subject, Target: created.ID, Details: "name=" + created.Name + ";metric=" + created.MetricType})
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAlertRuleItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/alert-rules/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !authz.Can(claims.Role, authz.PermSendCommand) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var existing storage.AlertRule
	found := false
	for _, rule := range s.store.ListAlertRules(claims.OrgID) {
		if rule.ID == id {
			existing = rule
			found = true
			break
		}
	}
	if !found || !s.authorizeClientResource(w, r, claims.OrgID, existing.OrgID, existing.ClientID) {
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body storage.AlertRule
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if body.ClientID != "" && !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.ID = id
		if err := s.store.UpdateAlertRule(body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "alert_rule_updated", Actor: claims.Subject, Target: id})
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := s.store.DeleteAlertRule(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "alert_rule_deleted", Actor: claims.Subject, Target: id})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAlertsCollection(w http.ResponseWriter, r *http.Request) {
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
	alerts := s.store.ListAlerts(claims.OrgID, r.URL.Query().Get("status"))
	if clientID != "" {
		allowedDevices := make(map[string]struct{})
		for _, device := range s.store.ListDevices(claims.OrgID) {
			if device.ClientID == clientID {
				allowedDevices[device.ID] = struct{}{}
			}
		}
		filtered := make([]storage.Alert, 0, len(alerts))
		for _, alert := range alerts {
			if _, allowed := allowedDevices[alert.DeviceID]; allowed {
				filtered = append(filtered, alert)
			}
		}
		alerts = filtered
	}
	respondJSON(w, alerts)
}

func (s *Server) handleAlertItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	claims, ok := claimsFromContext(r.Context())
	if !ok || !authz.Can(claims.Role, authz.PermSendCommand) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	alertID := strings.TrimSuffix(strings.TrimSuffix(path, "/acknowledge"), "/resolve")
	var existing storage.Alert
	found := false
	for _, alert := range s.store.ListAlerts(claims.OrgID, "") {
		if alert.ID == alertID {
			existing = alert
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	device, exists := s.store.GetDevice(existing.DeviceID)
	if !exists || !s.authorizeClientResource(w, r, claims.OrgID, existing.OrgID, device.ClientID) {
		return
	}
	switch {
	case strings.HasSuffix(path, "/acknowledge"):
		id := strings.TrimSuffix(path, "/acknowledge")
		if err := s.store.AcknowledgeAlert(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "alert_acknowledged", Actor: claims.Subject, Target: id})
		w.WriteHeader(http.StatusNoContent)
	case strings.HasSuffix(path, "/resolve"):
		id := strings.TrimSuffix(path, "/resolve")
		if err := s.store.ResolveAlert(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "alert_resolved", Actor: claims.Subject, Target: id})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}
