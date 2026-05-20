package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func setupStructuresTestRouter(h *AdminStructuresHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/v1/admin/songs/{song_id}/structures/{section_id}/copy-phrases", h.CopySectionPhrases)
	r.Put("/api/v1/admin/songs/{song_id}/structures/{structure_id}", h.Update)
	return r
}

func createSongForStructureTest(t *testing.T, store *storage.Store) *domain.Song {
	t.Helper()
	song := domain.NewSong("t", "a", "", nil)
	if err := store.CreateSong(song); err != nil {
		t.Fatalf("create song failed: %v", err)
	}
	return song
}

func TestUpdatePhraseRejectWhenTickOutOfParentRange(t *testing.T) {
	store := storage.New()
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
	store := storage.New()
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
	store := storage.New()
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
	store := storage.New()
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
	store := storage.New()
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
