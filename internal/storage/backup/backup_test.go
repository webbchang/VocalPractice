package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *storage.MemoryStore {
	t.Helper()
	return storage.NewMemoryStore()
}

func seedSong(store *storage.MemoryStore, t *testing.T) *domain.Song {
	t.Helper()
	trackID := uuid.New()
	tracks := []domain.MIDITrack{
		{ID: trackID, Name: "Lead Vocal", Instrument: "Voice", Channel: 0,
			IsVocal: true, Notes: []domain.MIDINote{{Pitch: 60, Velocity: 100, StartTime: 0, EndTime: 1}}},
	}
	song := domain.NewSong("Test Song", "Test Artist", "/midi/test.mid", tracks)
	if err := store.CreateSong(song); err != nil {
		t.Fatalf("CreateSong failed: %v", err)
	}
	section := domain.NewSongStructure(song.ID, &trackID, nil, domain.StructureTypeSECTION, "Verse", 0, 10, 0, 480, 1)
	if err := store.CreateStructure([]*domain.SongStructure{section}); err != nil {
		t.Fatalf("CreateStructure failed: %v", err)
	}
	if err := store.UpsertTrackLyrics(trackID, section.ID, "hello world"); err != nil {
		t.Fatalf("UpsertTrackLyrics failed: %v", err)
	}
	userID := uuid.New()
	a := domain.NewUserAssessment(userID, song.ID, trackID, &section.ID, 90.0, 10, 8, 5.0, 0.01,
		[]float64{0, 1, 2}, []float64{0.01, 0.02}, nil)
	if err := store.CreateAssessment(a); err != nil {
		t.Fatalf("CreateAssessment failed: %v", err)
	}
	user := domain.NewUser("someone", "someone@example.com", "hash")
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return song
}

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "backup.json")

	store := newTestStore(t)
	seedSong(store, t)

	mgr := NewManager(store, filePath)
	if err := mgr.Backup(); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	if !mgr.HasBackup() {
		t.Fatal("expected backup file to exist after Backup")
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	// Wipe the store and a fresh restored store.
	restored := storage.NewMemoryStore()
	restoredMgr := NewManager(restored, filePath)
	if err := restoredMgr.Restore(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Seeded admin + demo users should be replaced by backup contents. Backup
	// of the first store included 2 seeded users + the one we created = 3.
	// Seeded admin + testuser + webb + the one created in seedSong.
	users, _ := restored.ListUsers()
	if len(users) != 4 {
		t.Fatalf("expected 4 users after restore, got %d", len(users))
	}

	songs, _ := restored.ListSongs()
	if len(songs) != 1 {
		t.Fatalf("expected 1 song after restore, got %d", len(songs))
	}
	if songs[0].Title != "Test Song" {
		t.Fatalf("unexpected song title: %s", songs[0].Title)
	}

	lyrics, _ := restored.ListTrackLyricsByTrack(songs[0].Tracks[0].ID)
	if len(lyrics) != 1 || lyrics[0].Lyrics != "hello world" {
		t.Fatalf("unexpected lyrics after restore: %+v", lyrics)
	}

	assessments, _ := restored.ListAllAssessments(nil, nil)
	if len(assessments) != 1 {
		t.Fatalf("expected 1 assessment after restore, got %d", len(assessments))
	}
	if assessments[0].Score != 90.0 {
		t.Fatalf("unexpected score: %v", assessments[0].Score)
	}
}

func TestRestoreNoFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "does-not-exist.json")
	store := newTestStore(t)
	mgr := NewManager(store, filePath)
	err := mgr.Restore()
	if !errorsIs(err, ErrNoBackupFile) {
		t.Fatalf("expected ErrNoBackupFile, got %v", err)
	}
}

func TestAutoBackupOnWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "backup.json")

	store := newTestStore(t)
	mgr := NewManager(store, filePath)
	mgr.StartAutoBackup(time.Second)
	defer mgr.Close()

	// Mutating write should eventually be flushed to disk.
	user := domain.NewUser("writer", "writer@example.com", "h")
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filePath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected backup file to be created automatically, got: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	restored := storage.NewMemoryStore()
	if err := restored.RestoreFromJSON(data); err != nil {
		t.Fatalf("RestoreFromJSON failed: %v", err)
	}
	users, _ := restored.ListUsers()
	found := false
	for _, u := range users {
		if u.Email == "writer@example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("newly created user not found in restored data: %+v", users)
	}
}

func TestFlushPersistsChanges(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "backup.json")
	store := newTestStore(t)
	mgr := NewManager(store, filePath)
	defer mgr.Close()

	_ = store.CreateUser(domain.NewUser("flusher", "flusher@example.com", "h"))
	if err := mgr.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	if !mgr.HasBackup() {
		t.Fatal("expected backup file after Flush")
	}
}

func TestCloseFlushesPendingChanges(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "backup.json")
	store := newTestStore(t)
	mgr := NewManager(store, filePath)
	mgr.StartAutoBackup(time.Hour) // long interval so it depends on Close

	_ = store.CreateUser(domain.NewUser("graceful", "graceful@example.com", "h"))
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !mgr.HasBackup() {
		t.Fatal("expected backup file after Close with dirty state")
	}
}

// TestBackupPreservesPasswordHash ensures the password hash (marked json:"-"
// on the domain type to avoid leaking via the API) is still persisted and
// restored by the backup mechanism.
func TestBackupPreservesPasswordHash(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "backup.json")
	store := newTestStore(t)

	knownID := uuid.New()
	u := &domain.User{
		ID:           knownID,
		Username:     "secure",
		Email:        "secure@example.com",
		PasswordHash: "super-secret-hash",
		Role:         "user",
		IsActive:     true,
	}
	if err := store.CreateUser(u); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	mgr := NewManager(store, filePath)
	if err := mgr.Backup(); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	restored := storage.NewMemoryStore()
	if err := restored.RestoreFromJSON(mustReadFile(t, filePath)); err != nil {
		t.Fatalf("RestoreFromJSON failed: %v", err)
	}

	got, err := restored.GetUserByID(knownID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.PasswordHash != "super-secret-hash" {
		t.Fatalf("password hash not preserved: got %q", got.PasswordHash)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// errorsIs is a tiny helper to avoid pulling in errors just for one check.
func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		err = unwrap(err)
	}
	return false
}

func unwrap(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}
