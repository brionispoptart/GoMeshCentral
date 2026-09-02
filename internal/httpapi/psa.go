package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/storage"
)

func writePSAErr(w http.ResponseWriter, err error, status int) {
	http.Error(w, err.Error(), status)
}

func requirePSAManage(r *http.Request) (storage.AuditEvent, bool) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		return storage.AuditEvent{}, false
	}
	return storage.AuditEvent{Actor: claims.Subject, OrgID: claims.OrgID}, authz.Can(claims.Role, authz.PermManagePSA)
}

// Clients

func (s *Server) handleClientsCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.store.ListClients(claims.OrgID))
	case http.MethodPost:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Client
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		body.OrgID = claims.OrgID
		created, err := s.store.CreateClient(body)
		if err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "client_created"
		audit.Target = created.ID
		audit.Details = "name=" + created.Name
		s.appendAuditEvent(audit)
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClientItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	client, exists := s.store.GetClient(id)
	if !exists || !s.authorizeClientResource(w, r, claims.OrgID, client.OrgID, client.ID) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, client)
	case http.MethodPut:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Client
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		body.ID = id
		if err := s.store.UpdateClient(body); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "client_updated"
		audit.Target = id
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteClient(id); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "client_deleted"
		audit.Target = id
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Contracts

func (s *Server) handleContractsCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.store.ListContracts(claims.OrgID, r.URL.Query().Get("clientId")))
	case http.MethodPost:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Contract
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.ClientID) == "" || strings.TrimSpace(body.Name) == "" {
			http.Error(w, "clientId and name are required", http.StatusBadRequest)
			return
		}
		if !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.OrgID = claims.OrgID
		created, err := s.store.CreateContract(body)
		if err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "contract_created"
		audit.Target = created.ID
		audit.Details = "client_id=" + created.ClientID + ";name=" + created.Name
		s.appendAuditEvent(audit)
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleContractItem(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/generate-invoice") {
		s.handleGenerateContractInvoice(w, r)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/contracts/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	contract, exists := s.store.GetContract(id)
	if !exists || !s.authorizeClientResource(w, r, claims.OrgID, contract.OrgID, contract.ClientID) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, contract)
	case http.MethodPut:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Contract
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.ID = id
		if err := s.store.UpdateContract(body); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "contract_updated"
		audit.Target = id
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteContract(id); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "contract_deleted"
		audit.Target = id
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Tickets

func (s *Server) handleTicketsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		respondJSON(w, s.store.ListTickets(claims.OrgID, r.URL.Query().Get("clientId")))
	case http.MethodPost:
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body storage.Ticket
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Subject) == "" {
			http.Error(w, "subject is required", http.StatusBadRequest)
			return
		}
		if body.ClientID != "" && !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.CreatedBy = claims.Subject
		body.OrgID = claims.OrgID
		created, err := s.store.CreateTicket(body)
		if err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		// Invoke notification callback if set (email notifications, etc.)
		if callback := s.hub.GetOnTicketCreated(); callback != nil {
			_ = callback(created)
		}
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "ticket_created",
			Actor:   claims.Subject,
			Target:  created.ID,
			Details: "subject=" + created.Subject,
		})
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTicketItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tickets/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ticket, exists := s.store.GetTicket(id)
	if !exists || !s.authorizeClientResource(w, r, claims.OrgID, ticket.OrgID, ticket.ClientID) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, ticket)
	case http.MethodPut:
		var body storage.Ticket
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if body.ClientID != "" && !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.ID = id
		if err := s.store.UpdateTicket(body); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		s.appendAuditEvent(storage.AuditEvent{
			Action:  "ticket_updated",
			Actor:   claims.Subject,
			Target:  id,
			Details: "status=" + body.Status,
		})
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteTicket(id); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "ticket_deleted"
		audit.Target = id
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Invoices

func (s *Server) handleInvoicesCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, s.store.ListInvoices(claims.OrgID, r.URL.Query().Get("clientId")))
	case http.MethodPost:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Invoice
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.ClientID) == "" {
			http.Error(w, "clientId is required", http.StatusBadRequest)
			return
		}
		if !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.OrgID = claims.OrgID
		created, err := s.store.CreateInvoice(body)
		if err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "invoice_created"
		audit.Target = created.ID
		audit.Details = "client_id=" + created.ClientID + ";total=" + formatFloat(created.Total)
		s.appendAuditEvent(audit)
		respondJSON(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInvoiceItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/invoices/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	// Handle PDF routes
	if strings.HasSuffix(id, "/generate-pdf") {
		actualID := strings.TrimSuffix(id, "/generate-pdf")
		modifiedReq := r.Clone(r.Context())
		modifiedReq.URL.Path = "/api/invoices/" + actualID + "/generate-pdf"
		s.handleGenerateInvoicePDF(w, modifiedReq)
		return
	}
	if strings.HasSuffix(id, "/download-pdf") {
		actualID := strings.TrimSuffix(id, "/download-pdf")
		modifiedReq := r.Clone(r.Context())
		modifiedReq.URL.Path = "/api/invoices/" + actualID + "/download-pdf"
		s.handleDownloadInvoicePDF(w, modifiedReq)
		return
	}
	if strings.HasSuffix(id, "/send-email") {
		actualID := strings.TrimSuffix(id, "/send-email")
		modifiedReq := r.Clone(r.Context())
		modifiedReq.URL.Path = "/api/invoices/" + actualID + "/send-email"
		s.handleSendInvoiceEmail(w, modifiedReq)
		return
	}

	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	invoice, exists := s.store.GetInvoice(id)
	if !exists || !s.authorizeClientResource(w, r, claims.OrgID, invoice.OrgID, invoice.ClientID) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, invoice)
	case http.MethodPut:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body storage.Invoice
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if !s.clientBelongsToOrg(body.ClientID, claims.OrgID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		body.ID = id
		if err := s.store.UpdateInvoice(body); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "invoice_updated"
		audit.Target = id
		audit.Details = "status=" + body.Status
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		audit, allowed := requirePSAManage(r)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteInvoice(id); err != nil {
			writePSAErr(w, err, http.StatusInternalServerError)
			return
		}
		audit.Action = "invoice_deleted"
		audit.Target = id
		s.appendAuditEvent(audit)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func formatFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(jsonNumber(v), "0"), ".")
}

func jsonNumber(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
