package httpapi

import (
	"bytes"
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

func TestCallOpenAICompatible(t *testing.T) {
	var received openAIChatRequest
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": `{"reply":"Two devices need attention.","actions":[]}`}}},
		})
	}))
	defer provider.Close()

	answer, err := callOpenAICompatible(aiSettings{Provider: "custom", APIKey: "test-key", BaseURL: provider.URL, Model: "hermes-test"}, "system", `{"devices":[]}`, "What needs attention?")
	if err != nil {
		t.Fatalf("call provider: %v", err)
	}
	if answer == "" || received.Model != "hermes-test" || len(received.Messages) != 3 {
		t.Fatalf("unexpected provider exchange: answer=%q request=%+v", answer, received)
	}
}

func TestFetchAIModels(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "z-model"}, {"id": "a-model"}},
		})
	}))
	defer provider.Close()

	models, err := fetchAIModels(aiSettings{Provider: "custom", APIKey: "test-key", BaseURL: provider.URL})
	if err != nil {
		t.Fatalf("fetch models: %v", err)
	}
	if len(models) != 2 || models[0] != "a-model" || models[1] != "z-model" {
		t.Fatalf("unexpected models: %v", models)
	}
}

func TestAIProviderBaseURL(t *testing.T) {
	if got := aiProviderBaseURL("openrouter", "https://wrong.example"); got != "https://openrouter.ai/api/v1" {
		t.Fatalf("unexpected OpenRouter URL %q", got)
	}
	if got := aiProviderBaseURL("hermes", "https://wrong.example"); got != "http://localhost:11434/v1" {
		t.Fatalf("unexpected Hermes URL %q", got)
	}
	if got := aiProviderBaseURL("custom", "https://custom.example/v1/"); got != "https://custom.example/v1" {
		t.Fatalf("unexpected custom URL %q", got)
	}
}

func TestAIModelsEndpoint(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer route-key" {
			t.Fatalf("unexpected provider authorization header %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "route-model"}}})
	}))
	defer provider.Close()

	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	cfg := config.Config{JWTSecret: "test-secret-12345"}
	server := NewServer(cfg, store)
	token, err := auth.IssueToken("admin", authz.RoleAdmin, storage.DefaultOrgID, cfg.JWTSecret, time.Hour)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	settingsBody := []byte(`{"theme":"default","mailForwarding":{"smtpPort":587},"ai":{"provider":"custom","apiKey":"route-key","baseUrl":"` + provider.URL + `","model":"route-model"}}`)
	settingsReq := httptest.NewRequest(http.MethodPut, "/api/settings/application", bytes.NewReader(settingsBody))
	settingsReq.Header.Set("Authorization", "Bearer "+token)
	settingsRec := httptest.NewRecorder()
	server.ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("save settings: status=%d body=%s", settingsRec.Code, settingsRec.Body.String())
	}

	body := []byte(`{"provider":"custom","baseUrl":"` + provider.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/models", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("load models: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Models) != 1 || response.Models[0] != "route-model" {
		t.Fatalf("unexpected models: %v", response.Models)
	}
}

func TestApplicationSettingsPreservesAndRedactsAIKey(t *testing.T) {
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	cfg := config.Config{JWTSecret: "test-secret-12345"}
	server := NewServer(cfg, store)
	server.setApplicationSettings(applicationSettings{AI: aiSettings{Provider: "openrouter", APIKey: "stored-secret", BaseURL: "https://openrouter.ai/api/v1", Model: "test-model"}})
	token, err := auth.IssueToken("admin", authz.RoleAdmin, storage.DefaultOrgID, cfg.JWTSecret, time.Hour)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	body := []byte(`{"theme":"default","mailForwarding":{"smtpPort":587},"ai":{"provider":"openrouter","baseUrl":"https://openrouter.ai/api/v1","model":"test-model"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/application", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update settings: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response applicationSettings
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.AI.APIKey != "" || !response.AI.APIKeyConfigured {
		t.Fatalf("expected redacted configured key, got %+v", response.AI)
	}
	if got := server.getApplicationSettings().AI.APIKey; got != "stored-secret" {
		t.Fatalf("expected stored key to be preserved, got %q", got)
	}
}
