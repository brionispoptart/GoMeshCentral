package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gomeshcentral/internal/auth"
	"gomeshcentral/internal/authz"
	"gomeshcentral/internal/config"
	"gomeshcentral/internal/storage"
)

func TestClientScopeFiltersDevicesAndRejectsForeignClient(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	cfg := config.Config{
		ListenAddr: "localhost:8080",
		JWTSecret:  "test-secret-12345",
	}

	token, err := auth.IssueToken("admin", authz.RoleAdmin, storage.DefaultOrgID, cfg.JWTSecret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	client, err := store.CreateClient(storage.Client{Name: "Existing Client", OrgID: storage.DefaultOrgID})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	store.UpsertDevice(storage.Device{ID: "client-device", Name: "Client Device", ClientID: client.ID, OrgID: storage.DefaultOrgID})
	store.UpsertDevice(storage.Device{ID: "other-device", Name: "Other Device", OrgID: storage.DefaultOrgID})
	if err := store.AssignDeviceClient("client-device", client.ID); err != nil {
		t.Fatalf("failed to assign client device: %v", err)
	}

	server := NewServer(cfg, store)
	clientsReq := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	clientsReq.Header.Set("Authorization", "Bearer "+token)
	clientsRec := httptest.NewRecorder()
	server.ServeHTTP(clientsRec, clientsReq)
	if clientsRec.Code != http.StatusOK {
		t.Fatalf("expected clients status 200, got %d: %s", clientsRec.Code, clientsRec.Body.String())
	}
	var clients []storage.Client
	if err := json.NewDecoder(clientsRec.Body).Decode(&clients); err != nil {
		t.Fatalf("failed to decode clients response: %v", err)
	}
	if len(clients) != 1 || clients[0].ID != client.ID {
		t.Fatalf("expected existing client in selector source, got %+v", clients)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/devices?clientId="+client.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var devices []storage.Device
	if err := json.NewDecoder(rec.Body).Decode(&devices); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(devices) != 1 || devices[0].ID != "client-device" {
		t.Fatalf("expected only client-device, got %+v", devices)
	}

	foreignOrg, err := store.CreateOrganization(storage.Organization{Name: "Foreign Org"})
	if err != nil {
		t.Fatalf("failed to create foreign org: %v", err)
	}
	foreignClient, err := store.CreateClient(storage.Client{Name: "Foreign Client", OrgID: foreignOrg.ID})
	if err != nil {
		t.Fatalf("failed to create foreign client: %v", err)
	}
	foreignReq := httptest.NewRequest(http.MethodGet, "/api/devices?clientId="+foreignClient.ID, nil)
	foreignReq.Header.Set("Authorization", "Bearer "+token)
	foreignRec := httptest.NewRecorder()
	server.ServeHTTP(foreignRec, foreignReq)
	if foreignRec.Code != http.StatusNotFound {
		t.Fatalf("expected foreign client scope to return 404, got %d", foreignRec.Code)
	}
}
