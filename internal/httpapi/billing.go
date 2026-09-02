package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/billing"
	"gomeshcentral/internal/storage"
)

// POST /api/contracts/{id}/generate-invoice manually triggers billing for one
// contract right now (bypassing the due-date check), rolling in any unbilled
// billable time entries for that contract's client.
func (s *Server) handleGenerateContractInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/contracts/"), "/generate-invoice")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !authz.Can(claims.Role, authz.PermManagePSA) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	contract, ok := s.store.GetContract(id)
	if !ok || !s.authorizeClientResource(w, r, claims.OrgID, contract.OrgID, contract.ClientID) {
		return
	}
	invoice, err := billing.GenerateInvoiceForContract(s.store, contract, time.Now().UTC(), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "invoice_generated",
		Actor:   claims.Subject,
		Target:  invoice.ID,
		Details: "contract_id=" + contract.ID + ";total=" + formatFloat(invoice.Total),
	})
	respondJSON(w, invoice)
}

// Time entries

func (s *Server) handleTimeEntriesCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		respondJSON(w, s.store.ListTimeEntries(claims.OrgID, r.URL.Query().Get("clientId")))
	case http.MethodPost:
		claims, ok := claimsFromContext(r.Context())
		if !ok || !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.TimeEntry
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.ClientID) == "" || body.Minutes <= 0 {
			http.Error(w, "clientId and a positive minutes value are required", http.StatusBadRequest)
			return
		}
		if !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.CreatedBy = claims.Subject
		body.OrgID = claims.OrgID
		created, err := s.store.CreateTimeEntry(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "time_entry_logged",
			Actor:   claims.Subject,
			Target:  created.ID,
			Details: "client_id=" + created.ClientID + ";minutes=" + formatFloat(float64(created.Minutes)),
		})
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTimeEntryItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/time-entries/")
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
	var entry storage.TimeEntry
	found := false
	for _, candidate := range s.store.ListTimeEntries(claims.OrgID, "") {
		if candidate.ID == id {
			entry = candidate
			found = true
			break
		}
	}
	if !found || !s.authorizeClientResource(w, r, claims.OrgID, entry.OrgID, entry.ClientID) {
		return
	}
	if err := s.store.DeleteTimeEntry(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "time_entry_deleted",
		Actor:   claims.Subject,
		Target:  id,
		Details: "client_id=" + entry.ClientID,
	})
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/invoices/{id}/generate-pdf generates a PDF for an invoice
func (s *Server) handleGenerateInvoicePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/invoices/"), "/generate-pdf")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get invoice
	invoice, ok := s.store.GetInvoice(id)
	if !ok || !s.authorizeClientResource(w, r, claims.OrgID, invoice.OrgID, invoice.ClientID) {
		return
	}

	// Get organization and client details
	org := &storage.Organization{
		Name: "GoMeshCentral",
	}
	orgs := s.store.ListOrganizations()
	for _, o := range orgs {
		if o.ID == claims.OrgID {
			org = &o
			break
		}
	}

	client := &storage.Client{}
	if c, ok := s.store.GetClient(invoice.ClientID); ok {
		client = &c
	}

	// Get branding information
	branding, _ := s.store.GetBranding(claims.OrgID)

	// Generate PDF
	pdfBuf, err := billing.GenerateInvoicePDF(org, client, &invoice, &branding)
	if err != nil {
		http.Error(w, "failed to generate PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure invoices directory exists
	invoicesDir := filepath.Join("data", "invoices")
	os.MkdirAll(invoicesDir, 0755)

	// Generate filename
	filename := billing.GenerateInvoiceFileName(&invoice)
	filepath := filepath.Join(invoicesDir, filename)

	// Write PDF to file
	pdfFile, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "failed to save PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdfFile.Close()

	if _, err := io.Copy(pdfFile, pdfBuf); err != nil {
		http.Error(w, "failed to write PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit event
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "invoice_pdf_generated",
		Actor:   claims.Subject,
		Target:  id,
		Details: "filename=" + filename,
	})

	// Return filename
	respondJSON(w, map[string]string{
		"filename": filename,
		"url":      "/api/invoices/" + id + "/download-pdf",
	})
}

// GET /api/invoices/{id}/download-pdf downloads the PDF for an invoice
func (s *Server) handleDownloadInvoicePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/invoices/"), "/download-pdf")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get invoice
	invoice, ok := s.store.GetInvoice(id)
	if !ok || !s.authorizeClientResource(w, r, claims.OrgID, invoice.OrgID, invoice.ClientID) {
		return
	}

	// Try to find the PDF file
	invoicesDir := filepath.Join("data", "invoices")
	filename := billing.GenerateInvoiceFileName(&invoice)
	filepath := filepath.Join(invoicesDir, filename)

	// Check if file exists; if not, generate it
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		// Generate PDF first
		org := &storage.Organization{
			Name: "GoMeshCentral",
		}
		orgs := s.store.ListOrganizations()
		for _, o := range orgs {
			if o.ID == claims.OrgID {
				org = &o
				break
			}
		}

		client := &storage.Client{}
		if c, ok := s.store.GetClient(invoice.ClientID); ok {
			client = &c
		}

		// Get branding information
		branding, _ := s.store.GetBranding(claims.OrgID)

		pdfBuf, err := billing.GenerateInvoicePDF(org, client, &invoice, &branding)
		if err != nil {
			http.Error(w, "failed to generate PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}

		os.MkdirAll(invoicesDir, 0755)
		pdfFile, err := os.Create(filepath)
		if err != nil {
			http.Error(w, "failed to save PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer pdfFile.Close()

		if _, err := io.Copy(pdfFile, pdfBuf); err != nil {
			http.Error(w, "failed to write PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	// Serve the file
	http.ServeFile(w, r, filepath)
}

// POST /api/invoices/{id}/send-email sends an invoice to the client via email
func (s *Server) handleSendInvoiceEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/invoices/"), "/send-email")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get invoice
	invoice, ok := s.store.GetInvoice(id)
	if !ok || !s.authorizeClientResource(w, r, claims.OrgID, invoice.OrgID, invoice.ClientID) {
		return
	}

	// Get client
	client, ok := s.store.GetClient(invoice.ClientID)
	if !ok || client.ContactEmail == "" {
		http.Error(w, "client not found or email not configured", http.StatusBadRequest)
		return
	}

	// Get organization info for branding
	org, _ := s.store.GetOrganization(claims.OrgID)
	branding, _ := s.store.GetBranding(claims.OrgID)
	companyName := org.Name
	if branding.CompanyName != "" {
		companyName = branding.CompanyName
	}

	// Build download URL - use the server's agent public address if available
	baseURL := "http://localhost:8080"
	if s.agentPublicAddr != "" {
		baseURL = s.agentPublicAddr
	}
	downloadURL := baseURL + "/api/invoices/" + invoice.ID + "/download-pdf"

	// Send email
	err := s.emailService.SendInvoiceEmail(
		client.ContactEmail,
		companyName,
		invoice.InvoiceNumber,
		client.Name,
		invoice.Total,
		downloadURL,
	)
	if err != nil {
		http.Error(w, "failed to send email: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record audit event
	s.appendAuditEvent(storage.AuditEvent{
		Action:  "invoice_emailed",
		Actor:   claims.Subject,
		Target:  invoice.ID,
		Details: "to=" + client.ContactEmail + ";invoice_number=" + invoice.InvoiceNumber,
		OrgID:   claims.OrgID,
	})

	respondJSON(w, map[string]string{"message": "Invoice sent to " + client.ContactEmail})
}
