package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/storage"
)

type aiChatRequest struct {
	Message  string `json:"message"`
	ClientID string `json:"clientId,omitempty"`
}

type aiAction struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	ClientID    string `json:"clientId,omitempty"`
	DeviceID    string `json:"deviceId,omitempty"`
	AlertID     string `json:"alertId,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Body        string `json:"body,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Command     string `json:"command,omitempty"`
	ScriptID    string `json:"scriptId,omitempty"`
	ScriptBody  string `json:"scriptBody,omitempty"`
	ScriptOS    string `json:"scriptOs,omitempty"`
	ScriptName  string `json:"scriptName,omitempty"`
}

type aiChatResponse struct {
	Reply   string     `json:"reply"`
	Actions []aiAction `json:"actions"`
}

type aiActionRequest struct {
	Action   aiAction `json:"action"`
	ClientID string   `json:"clientId,omitempty"`
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type aiModelsRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseUrl"`
}

type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *Server) handleAIModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req aiModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.APIKey == "" {
		req.APIKey = strings.TrimSpace(s.getApplicationSettings().AI.APIKey)
		if req.APIKey == "" {
			http.Error(w, "API key is required to load models", http.StatusBadRequest)
			return
		}
	}
	models, err := fetchAIModels(aiSettings{
		Provider: req.Provider,
		APIKey:   req.APIKey,
		BaseURL:  aiProviderBaseURL(req.Provider, req.BaseURL),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	respondJSON(w, map[string]any{"models": models})
}

func aiProviderBaseURL(provider, customURL string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter", "":
		return "https://openrouter.ai/api/v1"
	case "hermes":
		return "http://localhost:11434/v1"
	default:
		return strings.TrimRight(strings.TrimSpace(customURL), "/")
	}
}

func fetchAIModels(settings aiSettings) ([]string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("AI base URL is not configured")
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+settings.APIKey)
	if strings.EqualFold(settings.Provider, "openrouter") {
		req.Header.Set("X-Title", "GoMeshCentral")
	}
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI provider request failed: %w", err)
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	var response openAIModelsResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("AI provider returned an invalid model response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if response.Error != nil && response.Error.Message != "" {
			return nil, fmt.Errorf("AI provider: %s", response.Error.Message)
		}
		return nil, fmt.Errorf("AI provider returned status %d", res.StatusCode)
	}
	models := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		if id := strings.TrimSpace(model.ID); id != "" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	return models, nil
}

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req aiChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	if req.ClientID != "" {
		client, exists := s.store.GetClient(req.ClientID)
		if !exists || client.OrgID != claims.OrgID {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
	}

	settings := s.getApplicationSettings().AI
	if strings.EqualFold(settings.Provider, "openrouter") && strings.TrimSpace(settings.APIKey) == "" {
		http.Error(w, "OpenRouter API key is not configured", http.StatusServiceUnavailable)
		return
	}
	contextJSON, err := json.Marshal(s.aiSystemContext(claims.OrgID, req.ClientID))
	if err != nil {
		http.Error(w, "failed to build system context", http.StatusInternalServerError)
		return
	}

	systemPrompt := `You are the operations assistant inside GoMeshCentral. Use only the supplied system context. Reply as strict JSON with this shape: {"reply":"concise answer","actions":[]}. Each action MUST have these fields: type, description, and action-specific params. Supported action types:
- create_ticket: requires clientId, subject, body, priority. Example: {"type":"create_ticket","description":"Create ticket about X","clientId":"xyz","subject":"...",  "body":"...","priority":"high"}
- send_command: requires deviceId, command. Example: {"type":"send_command","description":"Run script on P520","deviceId":"xyz","command":"powershell..."}
- acknowledge_alert: requires alertId. Example: {"type":"acknowledge_alert","description":"Dismiss alert","alertId":"xyz"}
- run_script: requires deviceId, scriptId. Example: {"type":"run_script","description":"Run Chocolatey on P520","deviceId":"xyz","scriptId":"abc"}
- save_script: requires scriptName, scriptBody, scriptOs. Example: {"type":"save_script","description":"Save new script","scriptName":"Install Stuff","scriptOs":"windows","scriptBody":"..."}
Never claim actions have been executed. Only propose actions necessary to satisfy requests.`
	result, err := callOpenAICompatible(settings, systemPrompt, string(contextJSON), req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	parsed := aiChatResponse{Reply: strings.TrimSpace(result), Actions: []aiAction{}}
	cleaned := strings.TrimSpace(result)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(cleaned)), &parsed); err != nil || parsed.Reply == "" {
		parsed = aiChatResponse{Reply: strings.TrimSpace(result), Actions: []aiAction{}}
	}

	s.appendAuditEvent(storage.AuditEvent{Action: "ai_assistant_queried", Actor: claims.Subject, Target: settings.Model, OrgID: claims.OrgID, Details: fmt.Sprintf("client_id=%s;proposed_actions=%d", req.ClientID, len(parsed.Actions))})
	respondJSON(w, parsed)
}

func (s *Server) handleAIActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req aiActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ClientID != "" && req.Action.ClientID != "" && req.ClientID != req.Action.ClientID {
		http.Error(w, "action is outside the selected client", http.StatusForbidden)
		return
	}

	switch req.Action.Type {
	case "create_ticket":
		if !authz.Can(claims.Role, authz.PermManagePSA) || !s.clientBelongsToOrg(req.Action.ClientID, claims.OrgID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		ticket, err := s.store.CreateTicket(storage.Ticket{ClientID: req.Action.ClientID, Subject: strings.TrimSpace(req.Action.Subject), Description: req.Action.Body, Priority: req.Action.Priority, Status: "open", CreatedBy: claims.Subject, OrgID: claims.OrgID})
		if err != nil || ticket.Subject == "" {
			http.Error(w, "failed to create ticket", http.StatusBadRequest)
			return
		}
		// Invoke notification callback if set
		if callback := s.hub.GetOnTicketCreated(); callback != nil {
			_ = callback(ticket)
		}
		s.auditAIAction(claims.Subject, claims.OrgID, req.Action.Type, ticket.ID)
		respondJSON(w, ticket)
	case "send_command":
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		device, exists := s.store.GetDevice(req.Action.DeviceID)
		if !exists || device.OrgID != claims.OrgID || (req.ClientID != "" && device.ClientID != req.ClientID) {
			http.NotFound(w, r)
			return
		}
		if strings.TrimSpace(req.Action.Command) == "" || s.hub.SendCommand(device.ID, req.Action.Command) != nil {
			http.Error(w, "device offline or command is empty", http.StatusConflict)
			return
		}
		s.auditAIAction(claims.Subject, claims.OrgID, req.Action.Type, device.ID)
		w.WriteHeader(http.StatusAccepted)
	case "acknowledge_alert":
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var alert storage.Alert
		found := false
		for _, candidate := range s.store.ListAlerts(claims.OrgID, "") {
			if candidate.ID == req.Action.AlertID {
				alert, found = candidate, true
				break
			}
		}
		device, deviceExists := s.store.GetDevice(alert.DeviceID)
		if !found || !deviceExists || (req.ClientID != "" && device.ClientID != req.ClientID) {
			http.NotFound(w, r)
			return
		}
		if err := s.store.AcknowledgeAlert(alert.ID); err != nil {
			http.Error(w, "failed to acknowledge alert", http.StatusInternalServerError)
			return
		}
		s.auditAIAction(claims.Subject, claims.OrgID, req.Action.Type, alert.ID)
		w.WriteHeader(http.StatusNoContent)
	case "run_script":
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if req.Action.ScriptID == "" {
			http.Error(w, "scriptId is required", http.StatusBadRequest)
			return
		}
		script, exists := s.store.GetScript(req.Action.ScriptID)
		if !exists || script.OrgID != claims.OrgID {
			http.NotFound(w, r)
			return
		}
		device, exists := s.store.GetDevice(req.Action.DeviceID)
		if !exists || device.OrgID != claims.OrgID || (req.ClientID != "" && device.ClientID != req.ClientID) {
			http.NotFound(w, r)
			return
		}
		if s.hub.SendCommand(device.ID, script.Body) != nil {
			http.Error(w, "device offline", http.StatusConflict)
			return
		}
		s.auditAIAction(claims.Subject, claims.OrgID, req.Action.Type, req.Action.ScriptID)
		w.WriteHeader(http.StatusAccepted)
	case "save_script":
		if !authz.Can(claims.Role, authz.PermSendCommand) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if strings.TrimSpace(req.Action.ScriptName) == "" || strings.TrimSpace(req.Action.ScriptBody) == "" {
			http.Error(w, "scriptName and scriptBody are required", http.StatusBadRequest)
			return
		}
		script, err := s.store.CreateScript(storage.Script{Name: strings.TrimSpace(req.Action.ScriptName), TargetOS: strings.TrimSpace(req.Action.ScriptOS), Body: req.Action.ScriptBody, CreatedBy: claims.Subject, OrgID: claims.OrgID})
		if err != nil {
			http.Error(w, "failed to save script", http.StatusInternalServerError)
			return
		}
		s.auditAIAction(claims.Subject, claims.OrgID, req.Action.Type, script.ID)
		respondJSON(w, script)
	default:
		http.Error(w, "unsupported AI action", http.StatusBadRequest)
	}
}

func (s *Server) aiSystemContext(orgID, clientID string) map[string]any {
	clients := s.store.ListClients(orgID)
	devices := s.store.ListDevices(orgID)
	if clientID != "" {
		filteredClients := make([]storage.Client, 0, 1)
		for _, client := range clients {
			if client.ID == clientID {
				filteredClients = append(filteredClients, client)
			}
		}
		clients = filteredClients
		filteredDevices := make([]storage.Device, 0)
		for _, device := range devices {
			if device.ClientID == clientID {
				filteredDevices = append(filteredDevices, device)
			}
		}
		devices = filteredDevices
	}
	return map[string]any{
		"selectedClientId": clientID,
		"clients":          clients,
		"devices":          devices,
		"contracts":        s.store.ListContracts(orgID, clientID),
		"tickets":          s.store.ListTickets(orgID, clientID),
		"invoices":         s.store.ListInvoices(orgID, clientID),
		"timeEntries":      s.store.ListTimeEntries(orgID, clientID),
		"alerts":           filterAIAlerts(s.store.ListAlerts(orgID, ""), devices, clientID),
	}
}

func filterAIAlerts(alerts []storage.Alert, devices []storage.Device, clientID string) []storage.Alert {
	if clientID == "" {
		return alerts
	}
	allowed := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		allowed[device.ID] = struct{}{}
	}
	filtered := make([]storage.Alert, 0)
	for _, alert := range alerts {
		if _, ok := allowed[alert.DeviceID]; ok {
			filtered = append(filtered, alert)
		}
	}
	return filtered
}

func callOpenAICompatible(settings aiSettings, systemPrompt, systemContext, userMessage string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("AI base URL is not configured")
	}
	payload := openAIChatRequest{Model: settings.Model, Temperature: 0.2, Messages: []openAIMessage{{Role: "system", Content: systemPrompt}, {Role: "system", Content: "Current system context JSON:\n" + systemContext}, {Role: "user", Content: userMessage}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+settings.APIKey)
	}
	if strings.EqualFold(settings.Provider, "openrouter") {
		req.Header.Set("X-Title", "GoMeshCentral")
	}
	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI provider request failed: %w", err)
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		return "", err
	}
	var response openAIChatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("AI provider returned an invalid response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if response.Error != nil && response.Error.Message != "" {
			return "", fmt.Errorf("AI provider: %s", response.Error.Message)
		}
		return "", fmt.Errorf("AI provider returned status %d", res.StatusCode)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI provider returned no answer")
	}
	return response.Choices[0].Message.Content, nil
}

func (s *Server) auditAIAction(actor, orgID, actionType, target string) {
	s.appendAuditEvent(storage.AuditEvent{Action: "ai_action_executed", Actor: actor, Target: target, OrgID: orgID, Details: "action_type=" + actionType})
}
