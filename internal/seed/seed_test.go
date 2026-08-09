package seed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	if err := os.WriteFile(path, []byte(`[
  {"id":"00000000-0000-0000-0000-000000000001","username":"admin","email":"admin@localhost","password_hash":"h1","role":"admin","is_active":true},
  {"id":"00000000-0000-0000-0000-000000000002","username":"testuser","email":"test@localhost","password_hash":"h2","role":"user","is_active":true}
]`), 0644); err != nil {
		t.Fatal(err)
	}

	recs, err := LoadUsers(path)
	if err != nil {
		t.Fatalf("LoadUsers failed: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 users, got %d", len(recs))
	}
	if recs[0].ID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("unexpected id: %s", recs[0].ID)
	}
	if recs[0].Role != "admin" {
		t.Errorf("expected admin role, got %s", recs[0].Role)
	}
	if recs[1].Role != "user" {
		t.Errorf("expected user role, got %s", recs[1].Role)
	}
}

func TestLoadSongs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "songs.json")
	if err := os.WriteFile(path, []byte(`[
  {"title":"油桐花","artist":"Josu Elberdin","midi_path":"test_data/x.mid","structures_csv":"test_data/x.csv"}
]`), 0644); err != nil {
		t.Fatal(err)
	}
	recs, err := LoadSongs(path)
	if err != nil {
		t.Fatalf("LoadSongs failed: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 song, got %d", len(recs))
	}
	if recs[0].Title != "油桐花" {
		t.Errorf("unexpected title: %s", recs[0].Title)
	}
	if recs[0].StructuresCSV != "test_data/x.csv" {
		t.Errorf("unexpected csv path: %s", recs[0].StructuresCSV)
	}
}

func TestLoadUsersMissingFile(t *testing.T) {
	if _, err := LoadUsers(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
