package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func setupStructuresTestRouter(h *AdminStructuresHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/admin/songs/{song_id}/structures", func(r chi.Router) {
		r.Get("/export", h.ExportStructures)
		r.Post("/import", h.ImportStructures)
		r.Post("/{section_id}/copy-phrases", h.CopySectionPhrases)
	})
	r.Put("/api/v1/admin/songs/{song_id}/structures/{structure_id}", h.Update)
	return r
}

func createSongForStructureTest(t *testing.T, store storage.Store) *domain.Song {
	t.Helper()
	song := domain.NewSong("t", "a", "", nil)
	if err := store.CreateSong(song); err != nil {
		t.Fatalf("create song failed: %v", err)
	}
	return song
}

func createSongWithTracks(t *testing.T, store storage.Store) *domain.Song {
	t.Helper()
	track1 := domain.MIDITrack{ID: uuid.New(), Name: "Lead Vocal", IsVocal: true, MIDIIndex: 0}
	track2 := domain.MIDITrack{ID: uuid.New(), Name: "Piano", IsVocal: false, MIDIIndex: 1}
	song := domain.NewSong("Test Song", "Test Artist", "", []domain.MIDITrack{track1, track2})
	if err := store.CreateSong(song); err != nil {
		t.Fatalf("create song failed: %v", err)
	}
	return song
}

func TestUpdatePhraseRejectWhenTickOutOfParentRange(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	section := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Verse", 1, 2, 100, 200, 1)
	phrase := domain.NewSongStructure(song.ID, nil, &section.ID, domain.StructureTypePHRASE, "Line", 1.2, 1.8, 120, 180, 1)
	if err := store.CreateStructure([]*domain.SongStructure{section, phrase}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}

	payload := map[string]interface{}{
		"type":       "PHRASE",
		"parent_id":  section.ID.String(),
		"start_tick": 50,
		"end_tick":   120,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/songs/"+song.ID.String()+"/structures/"+phrase.ID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSectionToPhraseWithParentSuccess(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	parent := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Parent", 0, 3, 0, 300, 1)
	target := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Target", 1, 2, 100, 200, 2)
	if err := store.CreateStructure([]*domain.SongStructure{parent, target}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}

	payload := map[string]interface{}{
		"type":       "PHRASE",
		"parent_id":  parent.ID.String(),
		"start_tick": 100,
		"end_tick":   200,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/songs/"+song.ID.String()+"/structures/"+target.ID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := store.GetStructureByID(target.ID)
	if err != nil {
		t.Fatalf("failed to get updated structure: %v", err)
	}
	if updated.Type != domain.StructureTypePHRASE {
		t.Fatalf("expected type PHRASE, got %s", updated.Type)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("expected parent_id=%s, got %+v", parent.ID.String(), updated.ParentID)
	}
}

func TestUpdatePhraseAcceptWhenTickEqualsParentBoundaries(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	parent := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Parent", 0, 3, 100, 200, 1)
	target := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Target", 0, 3, 100, 200, 2)
	if err := store.CreateStructure([]*domain.SongStructure{parent, target}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}

	payload := map[string]interface{}{
		"type":       "PHRASE",
		"parent_id":  parent.ID.String(),
		"start_tick": 100,
		"end_tick":   200,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/songs/"+song.ID.String()+"/structures/"+target.ID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePhraseToSectionClearsParent(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	parent := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Parent", 0, 3, 0, 300, 1)
	phrase := domain.NewSongStructure(song.ID, nil, &parent.ID, domain.StructureTypePHRASE, "Line", 1, 2, 100, 200, 1)
	if err := store.CreateStructure([]*domain.SongStructure{parent, phrase}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}

	payload := map[string]interface{}{
		"type":      "SECTION",
		"parent_id": "",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/songs/"+song.ID.String()+"/structures/"+phrase.ID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := store.GetStructureByID(phrase.ID)
	if err != nil {
		t.Fatalf("failed to get updated structure: %v", err)
	}
	if updated.Type != domain.StructureTypeSECTION {
		t.Fatalf("expected type SECTION, got %s", updated.Type)
	}
	if updated.ParentID != nil {
		t.Fatalf("expected parent_id nil, got %+v", updated.ParentID)
	}
}

func TestCopySectionPhrasesSuccess(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	trackID := uuid.New()
	source := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Source", 0, 10, 0, 1000, 1)
	target := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Target", 10, 20, 1000, 2000, 2)
	p1 := domain.NewSongStructure(song.ID, nil, &source.ID, domain.StructureTypePHRASE, "P1", 1, 2, 100, 200, 1)
	p2 := domain.NewSongStructure(song.ID, nil, &source.ID, domain.StructureTypePHRASE, "P2", 3, 4, 300, 400, 2)
	if err := store.CreateStructure([]*domain.SongStructure{source, target, p1, p2}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}
	if err := store.UpsertTrackLyrics(trackID, p1.ID, "lyrics-1"); err != nil {
		t.Fatalf("seed lyrics-1 failed: %v", err)
	}
	if err := store.UpsertTrackLyrics(trackID, p2.ID, "lyrics-2"); err != nil {
		t.Fatalf("seed lyrics-2 failed: %v", err)
	}

	payload := map[string]string{"target_section_id": target.ID.String()}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/songs/"+song.ID.String()+"/structures/"+source.ID.String()+"/copy-phrases", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rec.Code, rec.Body.String())
	}

	structures, err := store.ListStructuresBySong(song.ID)
	if err != nil {
		t.Fatalf("list structures failed: %v", err)
	}
	targetCount := 0
	for _, st := range structures {
		if st.Type == domain.StructureTypePHRASE && st.ParentID != nil && *st.ParentID == target.ID {
			targetCount++
		}
	}
	if targetCount != 2 {
		t.Fatalf("expected 2 copied phrases under target, got %d", targetCount)
	}

	lyrics, err := store.ListTrackLyricsByTrack(trackID)
	if err != nil {
		t.Fatalf("list lyrics failed: %v", err)
	}
	if len(lyrics) != 4 {
		t.Fatalf("expected 4 lyrics entries after copy, got %d", len(lyrics))
	}
}

// --- Export tests ---

func TestExportStructures_Empty(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/songs/%s/structures/export", song.ID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	csv := resp["csv"].(string)
	if !strings.HasPrefix(csv, "type,title,start,end,order,track_name,lyrics\n") {
		t.Fatalf("expected CSV header, got %q", csv)
	}
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected only header, got %d lines", len(lines))
	}
}

func TestExportStructures_WithSectionsAndPhrases(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	section := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Verse A", 0, 10, 0, 0, 1)
	phrase := domain.NewSongStructure(song.ID, nil, &section.ID, domain.StructureTypePHRASE, "Line 1", 1, 3, 0, 0, 1)
	if err := store.CreateStructure([]*domain.SongStructure{section, phrase}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/songs/%s/structures/export", song.ID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	csv := resp["csv"].(string)
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	// Header + comment + 2 data lines = 4 lines
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (header + comment + 2 data), got %d: %s", len(lines), csv)
	}
	// Line 2 should be the comment
	if !strings.Contains(lines[1], "# Track: (no track)") {
		t.Fatalf("expected comment '# Track: (no track)', got %s", lines[1])
	}
	// Line 3 should be S,Verse A
	if !strings.Contains(lines[2], "S") || !strings.Contains(lines[2], "Verse A") {
		t.Fatalf("expected S type + Verse A, got %s", lines[2])
	}
	// Line 4 should be P,Line 1
	if !strings.Contains(lines[3], "P") || !strings.Contains(lines[3], "Line 1") {
		t.Fatalf("expected P type + Line 1, got %s", lines[3])
	}
}

func TestExportStructures_WithTrackName(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongWithTracks(t, store)

	trackID := song.Tracks[0].ID
	section := domain.NewSongStructure(song.ID, &trackID, nil, domain.StructureTypeSECTION, "Section 1", 0, 5, 0, 0, 1)
	if err := store.CreateStructure([]*domain.SongStructure{section}); err != nil {
		t.Fatalf("create structures failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/songs/%s/structures/export", song.ID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	csv := resp["csv"].(string)
	// Should have "Lead Vocal" as track_name in CSV
	if !strings.Contains(csv, "Lead Vocal") {
		t.Fatalf("expected track_name 'Lead Vocal' in CSV, got: %s", csv)
	}
	// Verify song info in response
	songMap := resp["song"].(map[string]interface{})
	if songMap["title"] != "Test Song" {
		t.Fatalf("expected song title 'Test Song', got %v", songMap["title"])
	}
}

func TestExportStructures_SongNotFound(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)

	r.Get("/api/v1/admin/songs/"+uuid.New().String()+"/structures/export", h.ExportStructures)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/songs/"+uuid.New().String()+"/structures/export", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- Import tests ---

func TestImportStructures_Basic(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	csv := "type,title,start,end,order,track_name,lyrics\nS,Verse A,0.000000,10.000000,1,,\nP,Line 1,1.000000,3.000000,1,,"
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["imported"].(float64) != 2 {
		t.Fatalf("expected imported=2, got %v", resp["imported"])
	}

	// Verify structures were created
	structures, err := store.ListStructuresBySong(song.ID)
	if err != nil {
		t.Fatalf("list structures failed: %v", err)
	}
	if len(structures) != 2 {
		t.Fatalf("expected 2 structures, got %d", len(structures))
	}
}

func TestImportStructures_WithTrackName(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongWithTracks(t, store)

	csv := fmt.Sprintf("type,title,start,end,order,track_name,lyrics\nS,Verse A,0.000000,10.000000,1,%s,", song.Tracks[0].Name)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Verify the structure has the track ID
	structures, err := store.ListStructuresBySong(song.ID)
	if err != nil {
		t.Fatalf("list structures failed: %v", err)
	}
	if len(structures) != 1 {
		t.Fatalf("expected 1 structure, got %d", len(structures))
	}
	if structures[0].TrackID == nil || *structures[0].TrackID != song.Tracks[0].ID {
		t.Fatalf("expected track_id=%s, got %v", song.Tracks[0].ID, structures[0].TrackID)
	}
}

func TestImportStructures_WithLyrics(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongWithTracks(t, store)

	csv := fmt.Sprintf("type,title,start,end,order,track_name,lyrics\nS,Verse A,0,10,1,,\nP,Line 1,1,3,1,%s,第一句歌詞", song.Tracks[0].Name)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Verify lyrics were saved
	structures, err := store.ListStructuresBySong(song.ID)
	if err != nil {
		t.Fatalf("list structures failed: %v", err)
	}
	var phraseID uuid.UUID
	for _, s := range structures {
		if s.Type == domain.StructureTypePHRASE {
			phraseID = s.ID
			break
		}
	}
	if phraseID == uuid.Nil {
		t.Fatal("expected a phrase, found none")
	}

	lyrics, err := store.GetTrackLyrics(song.Tracks[0].ID, phraseID)
	if err != nil {
		t.Fatalf("get lyrics failed: %v", err)
	}
	if lyrics.Lyrics != "第一句歌詞" {
		t.Fatalf("expected lyrics '第一句歌詞', got %q", lyrics.Lyrics)
	}
}

func TestImportStructures_MissingTrack(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongWithTracks(t, store)

	csv := "type,title,start,end,order,track_name,lyrics\nS,Verse A,0,10,1,NonExistentTrack,"
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["error"] != "track(s) not found in song" {
		t.Fatalf("expected track error, got %v", resp["error"])
	}
	missing := resp["missing_tracks"].([]interface{})
	if len(missing) != 1 || missing[0] != "NonExistentTrack" {
		t.Fatalf("expected missing [NonExistentTrack], got %v", missing)
	}
	available := resp["available_tracks"].([]interface{})
	if len(available) != 2 {
		t.Fatalf("expected 2 available tracks, got %d", len(available))
	}
}

func TestImportStructures_Conflict(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	// Pre-create a structure that overlaps
	existing := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Old Section", 0, 5, 0, 0, 1)
	if err := store.CreateStructure([]*domain.SongStructure{existing}); err != nil {
		t.Fatalf("create structure failed: %v", err)
	}

	// Import overlapping structure without force
	csv := "type,title,start,end,order,track_name,lyrics\nS,New Section,3,8,1,,"
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["conflict"] != true {
		t.Fatalf("expected conflict=true, got %v", resp["conflict"])
	}
	// Old structure should still exist
	_, err := store.GetStructureByID(existing.ID)
	if err != nil {
		t.Fatalf("old structure should still exist: %v", err)
	}
}

func TestImportStructures_ForceOverwrite(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	// Pre-create a structure
	existing := domain.NewSongStructure(song.ID, nil, nil, domain.StructureTypeSECTION, "Old Section", 0, 5, 0, 0, 1)
	if err := store.CreateStructure([]*domain.SongStructure{existing}); err != nil {
		t.Fatalf("create structure failed: %v", err)
	}

	// Import overlapping structure WITH force
	csv := "type,title,start,end,order,track_name,lyrics\nS,New Section,3,8,1,,"
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Old structure should be gone
	_, err := store.GetStructureByID(existing.ID)
	if err == nil {
		t.Fatal("old structure should have been deleted")
	}

	// New structure should exist
	structures, err := store.ListStructuresBySong(song.ID)
	if err != nil {
		t.Fatalf("list structures failed: %v", err)
	}
	if len(structures) != 1 || structures[0].Title != "New Section" {
		t.Fatalf("expected 1 structure titled 'New Section', got %d: %+v", len(structures), structures)
	}
}

func TestImportStructures_PhraseWithoutSection(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	csv := "type,title,start,end,order,track_name,lyrics\nP,Orphan Phrase,0,5,1,,"
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(csv))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !strings.Contains(resp.Error, "has no containing section") {
		t.Fatalf("expected 'no containing section' error, got %q", resp.Error)
	}
}

func TestImportStructures_InvalidCSV(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	// Missing header
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader("bad,csv,data"))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestImportStructures_EmptyBody(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)
	song := createSongForStructureTest(t, store)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/songs/%s/structures/import?force=true", song.ID),
		strings.NewReader(""))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestImportStructures_SongNotFound(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAdminStructuresHandler(store)
	r := setupStructuresTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/songs/"+uuid.New().String()+"/structures/import?force=true",
		strings.NewReader("type,title,start,end,order,track_name,lyrics\nS,Test,0,1,1,,"))
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- Unit test for parseStructureCSV ---

func TestParseStructureCSV_Unit(t *testing.T) {
	csv := "type,title,start,end,order,track_name,lyrics\nS,Verse A,0.0,10.0,1,,\nP,Line 1,1.0,3.0,1,Lead Vocal,第一句"
	entries, err := parseStructureCSV(csv)
	if err != nil {
		t.Fatalf("parseStructureCSV failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Type != "S" || entries[0].Title != "Verse A" {
		t.Fatalf("first entry: expected S+Verse A, got %s+%s", entries[0].Type, entries[0].Title)
	}
	if entries[0].Start != 0.0 || entries[0].End != 10.0 {
		t.Fatalf("first entry: expected 0-10, got %f-%f", entries[0].Start, entries[0].End)
	}
	if entries[1].Type != "P" || entries[1].Title != "Line 1" || entries[1].TrackName != "Lead Vocal" || entries[1].Lyrics != "第一句" {
		t.Fatalf("second entry mismatch: %+v", entries[1])
	}
}

func TestParseStructureCSV_UnitQuoted(t *testing.T) {
	// CSV with quoted title containing comma
	csv := "type,title,start,end,order,track_name,lyrics\nS,\"Verse A, Part 1\",0.0,10.0,1,,\nP,\"Line 1, yeah\",1.0,3.0,1,,"
	entries, err := parseStructureCSV(csv)
	if err != nil {
		t.Fatalf("parseStructureCSV failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "Verse A, Part 1" {
		t.Fatalf("expected 'Verse A, Part 1', got %q", entries[0].Title)
	}
	if entries[1].Title != "Line 1, yeah" {
		t.Fatalf("expected 'Line 1, yeah', got %q", entries[1].Title)
	}
}
