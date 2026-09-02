package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gomeshcentral/internal/auth"
	"gomeshcentral/internal/storage"

	"golang.org/x/crypto/bcrypt"
)

type clientLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type clientLoginResponse struct {
	Token      string `json:"token"`
	ClientID   string `json:"clientId"`
	ClientName string `json:"clientName"`
	Email      string `json:"email"`
}

type clientMeResponse struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	ContactName              string    `json:"contactName"`
	ContactEmail             string    `json:"contactEmail"`
	ContactPhone             string    `json:"contactPhone"`
	PortalPointOfContactName string    `json:"portalPointOfContactName,omitempty"`
	CreatedAt                time.Time `json:"createdAt"`
}

type clientTicketSubmitRequest struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	DeviceID    string `json:"deviceId,omitempty"`
}

type clientTicketCommentRequest struct {
	Comment  string `json:"comment"`
	IsPublic bool   `json:"isPublic"`
}

// handleClientLogin authenticates a client or POC by email/password
func (s *Server) handleClientLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req clientLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	// First check if this is a POC user trying to log in
	allOrgs := s.store.ListOrganizations()
	for _, org := range allOrgs {
		users := s.store.ListUsers(org.ID)
		for i, u := range users {
			if strings.EqualFold(u.Email, email) && u.Role == "poc" {
				// Verify password
				if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
					http.Error(w, "invalid credentials", http.StatusUnauthorized)
					return
				}

				// Issue token with POC role
				token, err := auth.IssueToken(users[i].Username, "poc", u.OrgID, s.cfg.JWTSecret, 24*time.Hour)
				if err != nil {
					http.Error(w, "token generation failed", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(clientLoginResponse{
					Token:      token,
					ClientID:   users[i].Username,
					ClientName: users[i].Username,
					Email:      users[i].Email,
				})
				return
			}
		}
	}

	// Otherwise, check if this is a client trying to log in
	var client *storage.Client
	for _, org := range allOrgs {
		clients := s.store.ListClients(org.ID)
		for i, c := range clients {
			if strings.EqualFold(c.ContactEmail, email) && c.PortalEnabled {
				client = &clients[i]
				break
			}
		}
		if client != nil {
			break
		}
	}

	if client == nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(client.PortalPasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Issue token with client role
	token, err := auth.IssueToken(client.ID, "client", client.OrgID, s.cfg.JWTSecret, 24*time.Hour)
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientLoginResponse{
		Token:      token,
		ClientID:   client.ID,
		ClientName: client.Name,
		Email:      client.ContactEmail,
	})
}

// handleClientMe returns the authenticated client's or POC's own data
func (s *Server) handleClientMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok || (claims.Role != "client" && claims.Role != "poc") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claims.Role == "poc" {
		// Return POC user data
		users := s.store.ListUsers(claims.OrgID)
		for _, u := range users {
			if u.Username == claims.Subject {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":        u.Username,
					"name":      u.Username,
					"email":     u.Email,
					"role":      "poc",
					"createdAt": u.CreatedAt,
				})
				return
			}
		}
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Regular client
	client, ok := s.store.GetClient(claims.Subject)
	if !ok {
		http.Error(w, "client not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientMeResponse{
		ID:                       client.ID,
		Name:                     client.Name,
		ContactName:              client.ContactName,
		ContactEmail:             client.ContactEmail,
		ContactPhone:             client.ContactPhone,
		PortalPointOfContactName: client.PortalPointOfContactName,
		CreatedAt:                client.CreatedAt,
	})
}

// handleClientTickets returns tickets for the authenticated client, or pending approvals for POC
func (s *Server) handleClientTickets(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok || (claims.Role != "client" && claims.Role != "poc") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claims.Role == "poc" {
		// POCs can only GET pending approvals
		if r.Method == http.MethodGet {
			s.handlePOCPendingApprovals(w, claims.Subject, claims.OrgID)
		} else {
			http.Error(w, "POCs can only view tickets", http.StatusMethodNotAllowed)
		}
		return
	}

	// Regular client handling
	if r.Method == http.MethodGet {
		s.handleClientTicketsGet(w, claims.Subject, claims.OrgID)
	} else if r.Method == http.MethodPost {
		s.handleClientTicketsPost(w, r, claims.Subject, claims.OrgID)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePOCPendingApprovals returns tickets pending approval for a specific POC
func (s *Server) handlePOCPendingApprovals(w http.ResponseWriter, pocUsername string, orgID string) {
	// Get all tickets for this org with pending_approval status
	allTickets := s.store.ListTickets(orgID, "")

	var pendingTickets []storage.Ticket
	for _, ticket := range allTickets {
		if ticket.Status == "pending_approval" {
			// Verify this POC is the point of contact for this ticket's client
			if client, ok := s.store.GetClient(ticket.ClientID); ok {
				if client.PortalPointOfContactID == pocUsername {
					pendingTickets = append(pendingTickets, ticket)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(pendingTickets) == 0 {
		w.Write([]byte("[]"))
	} else {
		json.NewEncoder(w).Encode(pendingTickets)
	}
}

func (s *Server) handleClientTicketsGet(w http.ResponseWriter, clientID string, orgID string) {
	tickets := s.store.ListTickets(orgID, clientID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickets)
}

func (s *Server) handleClientTicketsPost(w http.ResponseWriter, r *http.Request, clientID, orgID string) {
	var req clientTicketSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Subject == "" || req.Description == "" {
		http.Error(w, "subject and description required", http.StatusBadRequest)
		return
	}

	// Create ticket with "pending_approval" status
	ticket := storage.Ticket{
		ClientID:    clientID,
		DeviceID:    req.DeviceID,
		Subject:     req.Subject,
		Description: req.Description,
		Status:      "pending_approval",
		Priority:    req.Priority,
		CreatedBy:   clientID,
		OrgID:       orgID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	created, err := s.store.CreateTicket(ticket)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create ticket: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// handleClientTicketDetail returns a single ticket with comments
func (s *Server) handleClientTicketDetail(w http.ResponseWriter, r *http.Request, ticketID string, clientID string) {
	ticket, ok := s.store.GetTicket(ticketID)
	if !ok || ticket.ClientID != clientID {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ticket)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClientInvoices returns invoices for the authenticated client
func (s *Server) handleClientInvoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok || claims.Role != "client" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	invoices := s.store.ListInvoices(claims.OrgID, claims.Subject)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoices)
}

// handleClientContracts returns contracts for the authenticated client
func (s *Server) handleClientContracts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok || claims.Role != "client" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contracts := s.store.ListContracts(claims.OrgID, claims.Subject)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contracts)
}

// handleClientDevices returns devices for the authenticated client (read-only)
func (s *Server) handleClientDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok || claims.Role != "client" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	devices := s.store.ListDevices(claims.OrgID)

	// Filter to only this client's devices
	var clientDevices []storage.Device
	for _, d := range devices {
		if d.ClientID == claims.Subject {
			clientDevices = append(clientDevices, d)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientDevices)
}

// handleApproveClientTicket allows point-of-contact to approve pending tickets
func (s *Server) handleApproveClientTicket(w http.ResponseWriter, r *http.Request, ticketID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ticket, ok := s.store.GetTicket(ticketID)
	if !ok {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	if ticket.Status != "pending_approval" {
		http.Error(w, "ticket is not pending approval", http.StatusBadRequest)
		return
	}

	// Verify user is the point of contact for this client
	client, ok := s.store.GetClient(ticket.ClientID)
	if !ok || client.PortalPointOfContactID != claims.Subject {
		http.Error(w, "unauthorized to approve this ticket", http.StatusForbidden)
		return
	}

	// Approve the ticket (move to "open")
	ticket.Status = "open"
	ticket.ApprovedBy = claims.Subject
	ticket.ApprovedAt = time.Now()
	ticket.UpdatedAt = time.Now()

	if err := s.store.UpdateTicket(ticket); err != nil {
		http.Error(w, "failed to approve ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

// handlePendingApprovals returns tickets pending approval for POCs in the org
func (s *Server) handlePendingApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use the context helper function to extract claims
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get all tickets for this org with pending_approval status
	allTickets := s.store.ListTickets(claims.OrgID, "")

	var pendingTickets []storage.Ticket
	for _, ticket := range allTickets {
		if ticket.Status == "pending_approval" {
			// Verify user is POC for this ticket's client
			if client, ok := s.store.GetClient(ticket.ClientID); ok {
				if client.PortalPointOfContactID == claims.Subject {
					pendingTickets = append(pendingTickets, ticket)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(pendingTickets) == 0 {
		w.Write([]byte("[]"))
	} else {
		json.NewEncoder(w).Encode(pendingTickets)
	}
}

// handleClientTicketAction handles /api/client/tickets/{id}/approve and /api/client/tickets/{id}/reject
func (s *Server) handleClientTicketAction(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(claimsContextKey).(auth.Claims)
	if !ok || (claims.Role != "client" && claims.Role != "poc") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Only POCs can approve/reject
	if claims.Role != "poc" {
		http.Error(w, "only POCs can manage approvals", http.StatusForbidden)
		return
	}

	// Parse URL path: /api/client/tickets/{id}/approve or /api/client/tickets/{id}/reject
	path := r.URL.Path
	const prefix = "/api/client/tickets/"

	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	rest := path[len(prefix):]
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	ticketID := parts[0]
	action := parts[1]

	if action == "approve" {
		s.handlePOCApproveTicket(w, r, ticketID, claims.Subject, claims.OrgID)
	} else if action == "reject" {
		s.handlePOCRejectTicket(w, r, ticketID, claims.Subject, claims.OrgID)
	} else {
		http.NotFound(w, r)
	}
}

// handlePOCApproveTicket allows a POC to approve a pending client ticket
func (s *Server) handlePOCApproveTicket(w http.ResponseWriter, r *http.Request, ticketID string, pocUsername string, orgID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ticket, ok := s.store.GetTicket(ticketID)
	if !ok {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	if ticket.Status != "pending_approval" {
		http.Error(w, "ticket is not pending approval", http.StatusBadRequest)
		return
	}

	// Verify this POC is the point of contact for the client
	client, ok := s.store.GetClient(ticket.ClientID)
	if !ok || client.PortalPointOfContactID != pocUsername {
		http.Error(w, "unauthorized to approve this ticket", http.StatusForbidden)
		return
	}

	// Approve the ticket
	ticket.Status = "open"
	ticket.ApprovedBy = pocUsername
	ticket.ApprovedAt = time.Now()
	ticket.UpdatedAt = time.Now()

	if err := s.store.UpdateTicket(ticket); err != nil {
		http.Error(w, "failed to approve ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

// handlePOCRejectTicket allows a POC to reject a pending client ticket
func (s *Server) handlePOCRejectTicket(w http.ResponseWriter, r *http.Request, ticketID string, pocUsername string, orgID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ticket, ok := s.store.GetTicket(ticketID)
	if !ok {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	if ticket.Status != "pending_approval" {
		http.Error(w, "ticket is not pending approval", http.StatusBadRequest)
		return
	}

	// Verify this POC is the point of contact for the client
	client, ok := s.store.GetClient(ticket.ClientID)
	if !ok || client.PortalPointOfContactID != pocUsername {
		http.Error(w, "unauthorized to reject this ticket", http.StatusForbidden)
		return
	}

	// Reject the ticket
	ticket.Status = "rejected"
	ticket.UpdatedAt = time.Now()

	if err := s.store.UpdateTicket(ticket); err != nil {
		http.Error(w, "failed to reject ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

// clientAuthMiddleware validates client or POC auth tokens
func (s *Server) clientAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		claims, err := auth.ParseToken(token, s.cfg.JWTSecret)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		if claims.Role != "client" && claims.Role != "poc" {
			http.Error(w, "not authorized for client portal", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, claimsContextKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
