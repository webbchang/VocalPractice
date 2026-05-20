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
	StartTick  int                `json:"start_tick"`
	EndTick    int                `json:"end_tick"`
	OrderIndex int                `json:"order_index"`
	TrackID    string             `json:"track_id,omitempty"`
	Lyrics     []phraseLyricInput `json:"lyrics,omitempty"`
}

type structureInput struct {
	Type       string        `json:"type"`
	Title      string        `json:"title"`
	StartTime  float64       `json:"start_time"`
	EndTime    float64       `json:"end_time"`
	StartTick  int           `json:"start_tick"`
	EndTick    int           `json:"end_tick"`
	OrderIndex int           `json:"order_index"`
	TrackID    string        `json:"track_id,omitempty"`
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
		if domain.StructureType(s.Type) != domain.StructureTypeSECTION {
			respondError(w, http.StatusBadRequest, "top-level structure must be SECTION")
			return
		}
		var sectionTrackID *uuid.UUID
		if s.TrackID != "" {
			if tid, err := uuid.Parse(s.TrackID); err == nil {
				sectionTrackID = &tid
			}
		}
		section := domain.NewSongStructure(
			songID, sectionTrackID, nil,
			domain.StructureType(s.Type),
			s.Title, s.StartTime, s.EndTime,
			s.StartTick, s.EndTick, s.OrderIndex,
		)
		allStructures = append(allStructures, section)

		for _, p := range s.Phrases {
			if domain.StructureType(p.Type) != domain.StructureTypePHRASE {
				respondError(w, http.StatusBadRequest, "nested structure must be PHRASE")
				return
			}
			parentID := section.ID
			var phraseTrackID *uuid.UUID
			if p.TrackID != "" {
				if tid, err := uuid.Parse(p.TrackID); err == nil {
					phraseTrackID = &tid
				}
			}
			phrase := domain.NewSongStructure(
				songID, phraseTrackID, &parentID,
				domain.StructureType(p.Type),
				p.Title, p.StartTime, p.EndTime,
				p.StartTick, p.EndTick, p.OrderIndex,
			)
			if !isStructureWithin(phrase.StartTick, phrase.EndTick, phrase.StartTime, phrase.EndTime, section) {
				respondError(w, http.StatusBadRequest, "phrase tick range must be within parent section tick range")
				return
			}
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
	Title     *string               `json:"title,omitempty"`
	StartTime *float64              `json:"start_time,omitempty"`
	EndTime   *float64              `json:"end_time,omitempty"`
	StartTick *int                  `json:"start_tick,omitempty"`
	EndTick   *int                  `json:"end_tick,omitempty"`
	OrderIdx  *int                  `json:"order_index,omitempty"`
	Type      *domain.StructureType `json:"type,omitempty"`
	ParentID  *string               `json:"parent_id,omitempty"`
}

type copyPhrasesRequest struct {
	TargetSectionID string `json:"target_section_id"`
}

func isRangeWithin(innerStart, innerEnd, outerStart, outerEnd int) bool {
	return innerStart >= outerStart && innerEnd <= outerEnd
}

func isStructureWithin(childStartTick, childEndTick int, childStartTime, childEndTime float64, parent *domain.SongStructure) bool {
	// Prefer real MIDI ticks when both sides have valid tick range (inclusive boundaries).
	if childEndTick >= childStartTick && parent.EndTick >= parent.StartTick {
		return isRangeWithin(childStartTick, childEndTick, parent.StartTick, parent.EndTick)
	}
	// Fallback for legacy records without valid tick ranges.
	return childStartTime >= parent.StartTime && childEndTime <= parent.EndTime
}

func findContainingSectionByTick(store *storage.Store, songID uuid.UUID, startTick, endTick int, excludeID *uuid.UUID) (*domain.SongStructure, error) {
	all, err := store.ListStructuresBySong(songID)
	if err != nil {
		return nil, err
	}
	for _, st := range all {
		if excludeID != nil && st.ID == *excludeID {
			continue
		}
		if st.Type != domain.StructureTypeSECTION {
			continue
		}
		if isStructureWithin(startTick, endTick, 0, 0, st) {
			return st, nil
		}
	}
	return nil, nil
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
	if req.StartTick != nil {
		st.StartTick = *req.StartTick
	}
	if req.EndTick != nil {
		st.EndTick = *req.EndTick
	}
	if req.OrderIdx != nil {
		st.OrderIdx = *req.OrderIdx
	}
	if req.Type != nil {
		st.Type = *req.Type
	}

	if req.ParentID != nil {
		if *req.ParentID == "" {
			st.ParentID = nil
		} else {
			pid, err := uuid.Parse(*req.ParentID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid parent_id")
				return
			}
			if pid == st.ID {
				respondError(w, http.StatusBadRequest, "parent_id cannot be self")
				return
			}
			parent, err := h.store.GetStructureByID(pid)
			if err != nil || parent.SongID != songID || parent.Type != domain.StructureTypeSECTION {
				respondError(w, http.StatusBadRequest, "parent_id must be a section in the same song")
				return
			}
			st.ParentID = &pid
		}
	}

	if st.Type == domain.StructureTypePHRASE {
		if st.ParentID == nil {
			parent, err := findContainingSectionByTick(h.store, songID, st.StartTick, st.EndTick, &st.ID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to validate parent section")
				return
			}
			if parent == nil {
				respondError(w, http.StatusBadRequest, "phrase must be within a section tick range")
				return
			}
			pid := parent.ID
			st.ParentID = &pid
		} else {
			parent, err := h.store.GetStructureByID(*st.ParentID)
			if err != nil || parent.SongID != songID || parent.Type != domain.StructureTypeSECTION {
				respondError(w, http.StatusBadRequest, "parent_id must be a section in the same song")
				return
			}
			if !isStructureWithin(st.StartTick, st.EndTick, st.StartTime, st.EndTime, parent) {
				respondError(w, http.StatusBadRequest, "phrase tick range must be within parent section tick range")
				return
			}
		}
	} else {
		st.ParentID = nil
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

func (h *AdminStructuresHandler) CopySectionPhrases(w http.ResponseWriter, r *http.Request) {
	songID, err := uuid.Parse(chi.URLParam(r, "song_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}
	sourceSectionID, err := uuid.Parse(chi.URLParam(r, "section_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid section_id")
		return
	}

	var req copyPhrasesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	targetSectionID, err := uuid.Parse(req.TargetSectionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid target_section_id")
		return
	}

	sourceSection, err := h.store.GetStructureByID(sourceSectionID)
	if err != nil || sourceSection.SongID != songID || sourceSection.Type != domain.StructureTypeSECTION {
		respondError(w, http.StatusBadRequest, "source section not found in song")
		return
	}
	targetSection, err := h.store.GetStructureByID(targetSectionID)
	if err != nil || targetSection.SongID != songID || targetSection.Type != domain.StructureTypeSECTION {
		respondError(w, http.StatusBadRequest, "target section not found in song")
		return
	}

	all, err := h.store.ListStructuresBySong(songID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list structures")
		return
	}

	sourcePhrases := make([]*domain.SongStructure, 0)
	for _, st := range all {
		if st.Type == domain.StructureTypePHRASE && st.ParentID != nil && *st.ParentID == sourceSectionID {
			sourcePhrases = append(sourcePhrases, st)
		}
	}

	newPhrases := make([]*domain.SongStructure, 0, len(sourcePhrases))
	oldToNew := make(map[uuid.UUID]uuid.UUID, len(sourcePhrases))
	for _, phrase := range sourcePhrases {
		pid := targetSection.ID
		copied := domain.NewSongStructure(
			songID,
			phrase.TrackID,
			&pid,
			domain.StructureTypePHRASE,
			phrase.Title,
			phrase.StartTime,
			phrase.EndTime,
			phrase.StartTick,
			phrase.EndTick,
			phrase.OrderIdx,
		)
		newPhrases = append(newPhrases, copied)
		oldToNew[phrase.ID] = copied.ID
	}

	if len(newPhrases) > 0 {
		if err := h.store.CreateStructure(newPhrases); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to copy phrases")
			return
		}
	}

	songLyrics, err := h.store.ListTrackLyricsBySong(songID)
	if err == nil {
		for _, l := range songLyrics {
			if newStructureID, ok := oldToNew[l.StructureID]; ok {
				_ = h.store.UpsertTrackLyrics(l.TrackID, newStructureID, l.Lyrics)
			}
		}
	}

	tree, err := h.store.BuildStructureTree(songID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build tree")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"song_id":        songID,
		"copied_count":   len(newPhrases),
		"source_section": sourceSectionID,
		"target_section": targetSectionID,
		"structures":     tree,
	})
}
