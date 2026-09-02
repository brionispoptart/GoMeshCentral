package httpapi

import (
	"encoding/json"
	"net/http"

	"gomeshcentral/internal/auth"
	"gomeshcentral/internal/storage"
)

// handleBrandingTest is a simple test handler
func (s *Server) handleBrandingTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"branding": "test", "method": "` + r.Method + `"}`))
}

// handleBrandingSimple handles GET and PUT for branding
func (s *Server) handleBrandingSimple(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"handler_called": true, "method": "` + r.Method + `"}`))
}

// handleBrandingCollection handles GET and PUT for branding
func (s *Server) handleBrandingCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "missing claims", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetBranding(w, r, claims)
	case http.MethodPut:
		s.handleSaveBranding(w, r, claims)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetBranding retrieves the organization's branding settings
func (s *Server) handleGetBranding(w http.ResponseWriter, r *http.Request, claims auth.Claims) {
	branding, err := s.store.GetBranding(claims.OrgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, branding)
}

// handleSaveBranding saves the organization's branding settings
func (s *Server) handleSaveBranding(w http.ResponseWriter, r *http.Request, claims auth.Claims) {
	if r.ContentLength > 10*1024*1024 { // 10MB limit
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	var branding storage.Branding
	if err := json.NewDecoder(r.Body).Decode(&branding); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Force the OrgID to match the authenticated org
	branding.OrgID = claims.OrgID

	if err := s.store.SaveBranding(branding); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "branding_updated",
		Actor:   claims.Subject,
		Target:  branding.OrgID,
		Details: "company_name=" + branding.CompanyName,
		OrgID:   claims.OrgID,
	})

	respondJSON(w, branding)
}
