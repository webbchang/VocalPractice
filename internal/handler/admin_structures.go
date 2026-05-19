package handler

import (
	"encoding/json"
	"net/http"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminStructuresHandler struct {
	store *storage.Store
}

func NewAdminStructuresHandler(store *storage.Store) *AdminStructuresHandler {
	return &AdminStructuresHandler{store: store}
}

type phraseLyricInput struct {
	TrackID string `json:"track_id"`
	Lyrics  string `json:"lyrics"`
}

type phraseInput struct {
	Type       string             `json:"type"`
	Title      string             `json:"title"`
	StartTime  float64            `json:"start_time"`
	EndTime    float64            `json:"end_time"`
	OrderIndex int                `json:"order_index"`
	Lyrics     []phraseLyricInput `json:"lyrics,omitempty"`
}

type structureInput struct {
	Type       string        `json:"type"`
	Title      string        `json:"title"`
	StartTime  float64       `json:"start_time"`
	EndTime    float64       `json:"end_time"`
	OrderIndex int           `json:"order_index"`
	Phrases    []phraseInput `json:"phrases,omitempty"`
}

type bulkCreateRequest struct {
	Structures []structureInput `json:"structures"`
}

func (h *AdminStructuresHandler) BulkCreate(w http.ResponseWriter, r *http.Request) {
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

	var req bulkCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var allStructures []*domain.SongStructure
	var lyricsEntries []struct {
		structureID uuid.UUID
		lyrics      []phraseLyricInput
	}

	for _, s := range req.Structures {
		section := domain.NewSongStructure(
			songID, nil,
			domain.StructureType(s.Type),
			s.Title, s.StartTime, s.EndTime,
			0, 0, s.OrderIndex,
		)
		allStructures = append(allStructures, section)

		for _, p := range s.Phrases {
			parentID := section.ID
			phrase := domain.NewSongStructure(
				songID, &parentID,
				domain.StructureType(p.Type),
				p.Title, p.StartTime, p.EndTime,
				0, 0, p.OrderIndex,
			)
			allStructures = append(allStructures, phrase)

			if len(p.Lyrics) > 0 {
				lyricsEntries = append(lyricsEntries, struct {
					structureID uuid.UUID
					lyrics      []phraseLyricInput
				}{structureID: phrase.ID, lyrics: p.Lyrics})
			}
		}
	}

	if err := h.store.CreateStructure(allStructures); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create structures")
		return
	}

	// Save lyrics
	for _, entry := range lyricsEntries {
		for _, l := range entry.lyrics {
			trackID, err := uuid.Parse(l.TrackID)
			if err != nil {
				continue
			}
			if err := h.store.UpsertTrackLyrics(trackID, entry.structureID, l.Lyrics); err != nil {
				// Log but don't fail the whole request
				continue
			}
		}
	}

	// Build response tree
	tree, err := h.store.BuildStructureTree(songID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build tree")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"song_id":    songID,
		"structures": tree,
	})
}

type updateStructureRequest struct {
	Title     *string  `json:"title,omitempty"`
	StartTime *float64 `json:"start_time,omitempty"`
	EndTime   *float64 `json:"end_time,omitempty"`
	OrderIdx  *int     `json:"order_index,omitempty"`
}

func (h *AdminStructuresHandler) Update(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	structIDStr := chi.URLParam(r, "structure_id")

	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	structID, err := uuid.Parse(structIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid structure_id")
		return
	}

	st, err := h.store.GetStructureByID(structID)
	if err != nil {
		respondError(w, http.StatusNotFound, "structure not found")
		return
	}

	if st.SongID != songID {
		respondError(w, http.StatusNotFound, "structure not found for this song")
		return
	}

	var req updateStructureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title != nil {
		st.Title = *req.Title
	}
	if req.StartTime != nil {
		st.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		st.EndTime = *req.EndTime
	}
	if req.OrderIdx != nil {
		st.OrderIdx = *req.OrderIdx
	}

	if err := h.store.UpdateStructure(st); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update structure")
		return
	}

	respondJSON(w, http.StatusOK, st)
}

func (h *AdminStructuresHandler) Delete(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	structIDStr := chi.URLParam(r, "structure_id")

	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	structID, err := uuid.Parse(structIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid structure_id")
		return
	}

	st, err := h.store.GetStructureByID(structID)
	if err != nil {
		respondError(w, http.StatusNotFound, "structure not found")
		return
	}

	if st.SongID != songID {
		respondError(w, http.StatusNotFound, "structure not found for this song")
		return
	}

	if err := h.store.DeleteStructure(structID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete structure")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
