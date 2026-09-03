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

func TestResolveDeviceIdentityRequiresTwoMatchingSignals(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	first, err := store.ResolveDeviceIdentity("machine-a", "system-a", "board-a", "Endpoint")
	if err != nil {
		t.Fatalf("register first identity: %v", err)
	}
	matched, err := store.ResolveDeviceIdentity("machine-a", "system-a", "board-b", "Renamed Endpoint")
	if err != nil {
		t.Fatalf("resolve changed identity: %v", err)
	}
	if matched.ID != first.ID {
		t.Fatalf("two matching signals should retain device %q, got %q", first.ID, matched.ID)
	}
	if matched.Name != "Renamed Endpoint" || matched.BoardIDHash != "board-b" {
		t.Fatalf("matched device was not refreshed: %+v", matched)
	}
	newDevice, err := store.ResolveDeviceIdentity("machine-a", "system-c", "board-c", "Different Endpoint")
	if err != nil {
		t.Fatalf("resolve one-signal overlap: %v", err)
	}
	if newDevice.ID == first.ID {
		t.Fatal("one matching signal must create a separate device")
	}
}
