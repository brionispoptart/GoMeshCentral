package httpapi

// This file contains manifest generation for portable agent deployments.
// The manifest is a JSON file containing server endpoint, branding, and bootstrap config.
// It is fetched by the installer and placed on the system for the agent to read.

import (
	"encoding/json"
	"net/http"
)

type manifestRequest struct {
	Token string `json:"token,omitempty"` // Bootstrap or enrollment token for auth-free download
}

type agentManifest struct {
	ServerEndpoint string `json:"serverEndpoint"`
	OrgID          string `json:"orgId,omitempty"`
	BootstrapToken string `json:"bootstrapToken,omitempty"`
	CompanyName    string `json:"companyName,omitempty"`
	LogoDataURL    string `json:"logoDataUrl,omitempty"`
}

// handleAgentManifest generates a deployment-specific manifest for the agent to read.
// Can be authenticated (admin) or accessed with a one-time bootstrap token.
// GET /api/download/agent/manifest - requires auth, returns manifest for authenticated org
// POST /api/download/agent/manifest - accepts bootstrap token in body
func (s *Server) handleAgentManifest(w http.ResponseWriter, r *http.Request) {
	agentServer := s.resolveAgentServer(r)

	switch r.Method {
	case http.MethodGet:
		// Authenticated request: user's org manifest
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		org, ok := s.store.GetOrganization(claims.OrgID)
		if !ok {
			http.Error(w, "organization not found", http.StatusNotFound)
			return
		}

		branding, _ := s.store.GetBranding(org.ID)
		manifest := agentManifest{
			ServerEndpoint: agentServer,
			OrgID:          org.ID,
			CompanyName:    branding.CompanyName,
			LogoDataURL:    branding.Logo,
		}
		respondJSON(w, manifest)

	case http.MethodPost:
		// Bootstrap token request: one-time manifest download
		var req manifestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		if req.Token == "" {
			http.Error(w, "token required", http.StatusBadRequest)
			return
		}

		// TODO: Validate bootstrap token and extract org context
		// For now, return manifest with no org-specific branding
		manifest := agentManifest{
			ServerEndpoint: agentServer,
			BootstrapToken: req.Token,
		}
		respondJSON(w, manifest)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDownloadAgentManifestForInstaller returns the manifest for use in installer flows.
// Called after user selects deployment options in the UI.
// The returned manifest includes bootstrap credentials that allow the agent to auto-register
// without needing a pre-existing enrollment token.
func (s *Server) handleDownloadAgentManifestForInstaller(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	agentServer := s.resolveAgentServer(r)
	org, ok := s.store.GetOrganization(claims.OrgID)
	if !ok {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	branding, _ := s.store.GetBranding(org.ID)

	manifest := agentManifest{
		ServerEndpoint: agentServer,
		OrgID:          org.ID,
		CompanyName:    branding.CompanyName,
		LogoDataURL:    branding.Logo,
	}

	// Return as JSON that can be embedded in installer or fetched at install time
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="manifest.json"`)
	json.NewEncoder(w).Encode(manifest)
}
