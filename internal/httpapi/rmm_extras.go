package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/storage"
)

// Device Groups

func (s *Server) handleDeviceGroupsCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.store.ListDeviceGroups(claims.OrgID))
	case http.MethodPost:
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.DeviceGroup
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		body.OrgID = claims.OrgID
		created, err := s.store.CreateDeviceGroup(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "device_group_created", Actor: claims.Subject, Target: created.ID, Details: "name=" + created.Name})
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDeviceGroupItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/device-groups/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !authz.Can(claims.Role, authz.PermSendCommand) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteDeviceGroup(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{Action: "device_group_deleted", Actor: claims.Subject, Target: id})
	w.WriteHeader(http.StatusNoContent)
}

// Scripts

func (s *Server) handleScriptsCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.store.ListScripts(claims.OrgID))
	case http.MethodPost:
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Script
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Body) == "" {
			http.Error(w, "name and body are required", http.StatusBadRequest)
			return
		}
		body.CreatedBy = claims.Subject
		body.OrgID = claims.OrgID
		created, err := s.store.CreateScript(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "script_created", Actor: claims.Subject, Target: created.ID, Details: "name=" + created.Name})
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleScriptItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/scripts/")
	if strings.HasSuffix(path, "/run") {
		s.handleRunScript(w, r, strings.TrimSuffix(path, "/run"))
		return
	}
	id := path
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		script, ok := s.store.GetScript(id)
		if !ok || script.OrgID != claims.OrgID {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, script)
	case http.MethodDelete:
		claims, ok := claimsFromContext(r.Context())
		if !ok || !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteScript(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{Action: "script_deleted", Actor: claims.Subject, Target: id})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRunScript dispatches a saved script's body as a single command to a
// device over the existing agent command channel (same path as ad-hoc "ping").
func (s *Server) handleRunScript(w http.ResponseWriter, r *http.Request, scriptID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !authz.Can(claims.Role, authz.PermSendCommand) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	script, ok := s.store.GetScript(scriptID)
	if !ok || script.OrgID != claims.OrgID {
		http.NotFound(w, r)
		return
	}
	var req struct {
		DeviceID string `json:"deviceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" {
		http.Error(w, "deviceId is required", http.StatusBadRequest)
		return
	}
	if err := s.hub.SendCommand(req.DeviceID, script.Body); err != nil {
		http.Error(w, "device offline", http.StatusConflict)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "script_run",
		Actor:   claims.Subject,
		Target:  req.DeviceID,
		Details: "script_id=" + scriptID + ";script_name=" + script.Name,
	})
	w.WriteHeader(http.StatusAccepted)
}

// Custom Device Fields

func (s *Server) handleCustomFieldsCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		fields := s.store.ListCustomFieldDefinitions(claims.OrgID)
		respondJSON(w, fields)
	case http.MethodPost:
		var body struct {
			FieldName string `json:"fieldName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.FieldName) == "" {
			http.Error(w, "fieldName is required", http.StatusBadRequest)
			return
		}
		if err := s.store.SaveCustomFieldDefinition(claims.OrgID, body.FieldName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "custom_field_created",
			Actor:   claims.Subject,
			Target:  body.FieldName,
			Details: "org_id=" + claims.OrgID,
		})
		respondJSON(w, map[string]string{"fieldName": body.FieldName})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCustomFieldItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract field name from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/devices/custom-fields/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "field name required", http.StatusBadRequest)
		return
	}
	fieldName := parts[0]

	switch r.Method {
	case http.MethodDelete:
		if err := s.store.DeleteCustomFieldDefinition(claims.OrgID, fieldName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "custom_field_deleted",
			Actor:   claims.Subject,
			Target:  fieldName,
			Details: "org_id=" + claims.OrgID,
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetDeviceCustomFields(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract device ID from URL path (/api/devices/{deviceID}/custom-fields)
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	deviceID := strings.TrimSuffix(path, "/custom-fields")
	if deviceID == "" {
		http.Error(w, "device id required", http.StatusBadRequest)
		return
	}

	// Verify device belongs to org
	device, ok := s.store.GetDevice(deviceID)
	if !ok || device.OrgID != claims.OrgID {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	respondJSON(w, device.CustomFields)
}

func (s *Server) handleUpdateDeviceCustomFields(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract device ID from URL path (/api/devices/{deviceID}/custom-fields)
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	deviceID := strings.TrimSuffix(path, "/custom-fields")
	if deviceID == "" {
		http.Error(w, "device id required", http.StatusBadRequest)
		return
	}

	// Verify device belongs to org
	device, ok := s.store.GetDevice(deviceID)
	if !ok || device.OrgID != claims.OrgID {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	var fields map[string]string
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateDeviceCustomFields(deviceID, fields); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "device_custom_fields_updated",
		Actor:   claims.Subject,
		Target:  deviceID,
		Details: "device_name=" + device.Name,
	})

	respondJSON(w, fields)
}
