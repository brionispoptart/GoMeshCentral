package storage

import (
	"path/filepath"
	"testing"
)

func TestAppSettingPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	key := "application_settings"
	val := `{"theme":"midnight","customDomain":"portal.example.com"}`

	if err := store.SaveSetting(key, val); err != nil {
		t.Fatalf("failed to save setting: %v", err)
	}

	got, err := store.GetSetting(key)
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}

	if got != val {
		t.Errorf("expected %q, got %q", val, got)
	}
}
