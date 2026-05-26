package handler

import (
	"encoding/json"
	"net/http"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminLyricsHandler struct {
	store storage.Store
}

func NewAdminLyricsHandler(store storage.Store) *AdminLyricsHandler {
	return &AdminLyricsHandler{store: store}
}

type lyricsBatchEntry struct {
	TrackID  string `json:"track_id"`
	StructID string `json:"structure_id"`
	Lyrics   string `json:"lyrics"`
}

type lyricsBatchRequest struct {
	Lyrics []lyricsBatchEntry `json:"lyrics"`
}

func (h *AdminLyricsHandler) BatchUpsert(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	// Verify song exists
	if _, err := h.store.GetSongByID(songID); err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	var req lyricsBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated := 0
	for _, entry := range req.Lyrics {
		trackID, err := uuid.Parse(entry.TrackID)
		if err != nil {
			continue
		}
		structID, err := uuid.Parse(entry.StructID)
		if err != nil {
			continue
		}
		if err := h.store.UpsertTrackLyrics(trackID, structID, entry.Lyrics); err == nil {
			updated++
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"updated": updated,
		"total":   len(req.Lyrics),
	})
}

func (h *AdminLyricsHandler) GetByTrack(w http.ResponseWriter, r *http.Request) {
	trackIDStr := chi.URLParam(r, "track_id")
	trackID, err := uuid.Parse(trackIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid track_id")
		return
	}

	result, err := h.store.ListTrackLyricsByTrack(trackID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list lyrics")
		return
	}

	if result == nil {
		result = make([]*domain.TrackLyrics, 0)
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *AdminLyricsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	trackIDStr := r.URL.Query().Get("track_id")
	structIDStr := r.URL.Query().Get("structure_id")

	if _, err := uuid.Parse(songIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	if trackIDStr == "" || structIDStr == "" {
		respondError(w, http.StatusBadRequest, "track_id and structure_id query params required")
		return
	}

	trackID, err := uuid.Parse(trackIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid track_id")
		return
	}

	structID, err := uuid.Parse(structIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid structure_id")
		return
	}

	if err := h.store.DeleteTrackLyrics(trackID, structID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete lyrics")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
