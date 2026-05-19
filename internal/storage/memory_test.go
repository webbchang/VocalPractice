package storage

import (
	"testing"

	"vocal-practice-app/internal/domain"

	"github.com/google/uuid"
)

func TestCreateAndGetUser(t *testing.T) {
	s := New()
	user := domain.NewUser("testuser", "test@example.com", "hash123")
	if err := s.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", got.Username)
	}

	gotByEmail, err := s.GetUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if gotByEmail.ID != user.ID {
		t.Errorf("expected user ID %v, got %v", user.ID, gotByEmail.ID)
	}
}

func TestCreateDuplicateUserEmail(t *testing.T) {
	s := New()
	user1 := domain.NewUser("user1", "same@example.com", "hash1")
	user2 := domain.NewUser("user2", "same@example.com", "hash2")

	if err := s.CreateUser(user1); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if err := s.CreateUser(user2); err != ErrEmailExists {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
}

func TestListUsers(t *testing.T) {
	s := New()
	s.CreateUser(domain.NewUser("a", "a@test.com", "h"))
	s.CreateUser(domain.NewUser("b", "b@test.com", "h"))

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestCreateAndGetSong(t *testing.T) {
	s := New()
	tracks := []domain.MIDITrack{
		{
			ID:         uuid.New(),
			Name:       "Lead Vocal",
			Instrument: "Voice",
			Channel:    0,
			IsVocal:    true,
			Notes: []domain.MIDINote{
				{Pitch: 60, Velocity: 100, StartTime: 0.0, EndTime: 1.0},
			},
		},
	}
	song := domain.NewSong("Test Song", "Test Artist", "/path/to/file.mid", tracks)
	if err := s.CreateSong(song); err != nil {
		t.Fatalf("CreateSong failed: %v", err)
	}

	got, err := s.GetSongByID(song.ID)
	if err != nil {
		t.Fatalf("GetSongByID failed: %v", err)
	}
	if got.Title != "Test Song" {
		t.Errorf("expected title Test Song, got %s", got.Title)
	}
	if len(got.Tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(got.Tracks))
	}
}

func TestListSongs(t *testing.T) {
	s := New()
	s.CreateSong(domain.NewSong("S1", "A1", "/p1", nil))
	s.CreateSong(domain.NewSong("S2", "A2", "/p2", nil))

	songs, err := s.ListSongs()
	if err != nil {
		t.Fatalf("ListSongs failed: %v", err)
	}
	if len(songs) != 2 {
		t.Errorf("expected 2 songs, got %d", len(songs))
	}
}

func TestStructureCRUD(t *testing.T) {
	s := New()
	songID := uuid.New()

	section := domain.NewSongStructure(songID, nil, domain.StructureTypeSECTION, "Verse", 0, 30, 0, 480, 1)
	phrase := domain.NewSongStructure(songID, &section.ID, domain.StructureTypePHRASE, "Line 1", 0, 15, 0, 240, 1)

	if err := s.CreateStructure([]*domain.SongStructure{section, phrase}); err != nil {
		t.Fatalf("CreateStructure failed: %v", err)
	}

	// Get by ID
	got, err := s.GetStructureByID(section.ID)
	if err != nil {
		t.Fatalf("GetStructureByID failed: %v", err)
	}
	if got.Title != "Verse" {
		t.Errorf("expected title Verse, got %s", got.Title)
	}

	// Update
	got.Title = "Chorus"
	if err := s.UpdateStructure(got); err != nil {
		t.Fatalf("UpdateStructure failed: %v", err)
	}
	updated, _ := s.GetStructureByID(section.ID)
	if updated.Title != "Chorus" {
		t.Errorf("expected title Chorus, got %s", updated.Title)
	}

	// List by song
	structs, err := s.ListStructuresBySong(songID)
	if err != nil {
		t.Fatalf("ListStructuresBySong failed: %v", err)
	}
	if len(structs) != 2 {
		t.Errorf("expected 2 structures, got %d", len(structs))
	}

	// Build tree
	tree, err := s.BuildStructureTree(songID)
	if err != nil {
		t.Fatalf("BuildStructureTree failed: %v", err)
	}
	if len(tree) != 1 {
		t.Errorf("expected 1 root node, got %d", len(tree))
	}
	if len(tree[0].Phrases) != 1 {
		t.Errorf("expected 1 phrase, got %d", len(tree[0].Phrases))
	}

	// Delete (cascade)
	if err := s.DeleteStructure(section.ID); err != nil {
		t.Fatalf("DeleteStructure failed: %v", err)
	}
	structs, _ = s.ListStructuresBySong(songID)
	if len(structs) != 0 {
		t.Errorf("expected 0 structures after cascade delete, got %d", len(structs))
	}
}

func TestTrackLyricsCRUD(t *testing.T) {
	s := New()
	trackID := uuid.New()
	structID := uuid.New()

	// Upsert
	if err := s.UpsertTrackLyrics(trackID, structID, "第一句歌詞"); err != nil {
		t.Fatalf("UpsertTrackLyrics failed: %v", err)
	}

	// Get
	got, err := s.GetTrackLyrics(trackID, structID)
	if err != nil {
		t.Fatalf("GetTrackLyrics failed: %v", err)
	}
	if got.Lyrics != "第一句歌詞" {
		t.Errorf("expected lyrics '第一句歌詞', got '%s'", got.Lyrics)
	}
	if got.TrackID != trackID {
		t.Errorf("expected trackID %v, got %v", trackID, got.TrackID)
	}
	if got.StructureID != structID {
		t.Errorf("expected structureID %v, got %v", structID, got.StructureID)
	}

	// Upsert update
	if err := s.UpsertTrackLyrics(trackID, structID, "更新後的歌詞"); err != nil {
		t.Fatalf("UpsertTrackLyrics update failed: %v", err)
	}
	got, _ = s.GetTrackLyrics(trackID, structID)
	if got.Lyrics != "更新後的歌詞" {
		t.Errorf("expected updated lyrics, got '%s'", got.Lyrics)
	}

	// List by track
	trackID2 := uuid.New()
	s.UpsertTrackLyrics(trackID, structID, "track1 lyrics")
	structID2 := uuid.New()
	s.UpsertTrackLyrics(trackID, structID2, "track1 lyrics 2")
	s.UpsertTrackLyrics(trackID2, structID, "track2 lyrics")

	track1List, err := s.ListTrackLyricsByTrack(trackID)
	if err != nil {
		t.Fatalf("ListTrackLyricsByTrack failed: %v", err)
	}
	if len(track1List) != 2 {
		t.Errorf("expected 2 lyrics for track1, got %d", len(track1List))
	}

	// List by song
	songID := uuid.New()
	s.CreateStructure([]*domain.SongStructure{
		domain.NewSongStructure(songID, nil, domain.StructureTypeSECTION, "Verse", 0, 30, 0, 480, 1),
	})
	// lyrics was stored with structID which is not tied to songID, so song list should be empty
	songLyrics, err := s.ListTrackLyricsBySong(songID)
	if err != nil {
		t.Fatalf("ListTrackLyricsBySong failed: %v", err)
	}
	if len(songLyrics) != 0 {
		t.Errorf("expected 0 lyrics for song with no matching structures, got %d", len(songLyrics))
	}

	// Delete
	if err := s.DeleteTrackLyrics(trackID, structID); err != nil {
		t.Fatalf("DeleteTrackLyrics failed: %v", err)
	}
	_, err = s.GetTrackLyrics(trackID, structID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent
	if err := s.DeleteTrackLyrics(uuid.New(), uuid.New()); err != nil {
		t.Errorf("expected nil for deleting non-existent lyrics, got %v", err)
	}
}

func TestAssessmentCRUD(t *testing.T) {
	s := New()
	userID := uuid.New()
	songID := uuid.New()
	trackID := uuid.New()

	a := domain.NewUserAssessment(userID, songID, trackID, nil, 85.5, 50, 42, 15.3, 0.08, []float64{10, -5}, []float64{0.05}, nil)
	if err := s.CreateAssessment(a); err != nil {
		t.Fatalf("CreateAssessment failed: %v", err)
	}

	// Get by ID
	got, err := s.GetAssessmentByID(a.ID)
	if err != nil {
		t.Fatalf("GetAssessmentByID failed: %v", err)
	}
	if got.Score != 85.5 {
		t.Errorf("expected score 85.5, got %f", got.Score)
	}

	// List by user
	assessments, err := s.ListAssessmentsByUser(userID, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListAssessmentsByUser failed: %v", err)
	}
	if len(assessments) != 1 {
		t.Errorf("expected 1 assessment, got %d", len(assessments))
	}

	// List all
	all, err := s.ListAllAssessments(nil, nil)
	if err != nil {
		t.Fatalf("ListAllAssessments failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 assessment total, got %d", len(all))
	}
}
