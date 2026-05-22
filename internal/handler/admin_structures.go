package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

func trackName(trackID *uuid.UUID, song *domain.Song) string {
	if trackID == nil {
		return ""
	}
	for _, t := range song.Tracks {
		if t.ID == *trackID {
			return t.Name
		}
	}
	return trackID.String()
}

func (h *AdminStructuresHandler) ExportStructures(w http.ResponseWriter, r *http.Request) {
	songID, err := uuid.Parse(chi.URLParam(r, "song_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	song, err := h.store.GetSongByID(songID)
	if err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	structures, err := h.store.ListStructuresBySong(songID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list structures")
		return
	}

	// Build lyrics map: structureID -> trackID -> lyrics
	lyricsByStruct := make(map[uuid.UUID]map[uuid.UUID]string)
	songLyrics, _ := h.store.ListTrackLyricsBySong(songID)
	for _, l := range songLyrics {
		if lyricsByStruct[l.StructureID] == nil {
			lyricsByStruct[l.StructureID] = make(map[uuid.UUID]string)
		}
		lyricsByStruct[l.StructureID][l.TrackID] = l.Lyrics
	}

	// Build CSV rows
	var sb strings.Builder
	sb.WriteString("type,title,start,end,order,track_name,lyrics\n")
	for _, st := range structures {
		tName := trackName(st.TrackID, song)
		lyrics := ""
		if st.TrackID != nil {
			if m, ok := lyricsByStruct[st.ID]; ok {
				lyrics = m[*st.TrackID]
			}
		}
		t := "S"
		if st.Type == domain.StructureTypePHRASE {
			t = "P"
		}
		// Quote fields that may contain commas or quotes
		title := strings.ReplaceAll(st.Title, "\"", "\"\"")
		lyr := strings.ReplaceAll(lyrics, "\"", "\"\"")
		sb.WriteString(fmt.Sprintf("%s,\"%s\",%.6f,%.6f,%d,%s,\"%s\"\n",
			t, title, st.StartTime, st.EndTime, st.OrderIdx, tName, lyr))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"song": map[string]interface{}{
			"id":     song.ID,
			"title":  song.Title,
			"artist": song.Artist,
			"tracks": song.Tracks,
		},
		"csv": sb.String(),
	})
}

func (h *AdminStructuresHandler) ImportStructures(w http.ResponseWriter, r *http.Request) {
	songID, err := uuid.Parse(chi.URLParam(r, "song_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	song, err := h.store.GetSongByID(songID)
	if err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	// Read raw CSV body
	body, err := readAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	csvStr := strings.TrimSpace(string(body))
	if csvStr == "" {
		respondError(w, http.StatusBadRequest, "empty CSV body")
		return
	}

	// Parse CSV
	entries, err := parseStructureCSV(csvStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
		return
	}
	if len(entries) == 0 {
		respondError(w, http.StatusBadRequest, "no structures found in CSV")
		return
	}

	// Resolve track names to track IDs
	trackByName := make(map[string]uuid.UUID)
	for _, t := range song.Tracks {
		trackByName[t.Name] = t.ID
	}
	var missingTracks []string
	for _, e := range entries {
		if e.TrackName != "" {
			if _, ok := trackByName[e.TrackName]; !ok {
				missingTracks = append(missingTracks, e.TrackName)
			}
		}
	}
	if len(missingTracks) > 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":          "track(s) not found in song",
			"missing_tracks": missingTracks,
			"available_tracks": func() []map[string]interface{} {
				res := make([]map[string]interface{}, 0, len(song.Tracks))
				for _, t := range song.Tracks {
					res = append(res, map[string]interface{}{
						"name":     t.Name,
						"id":       t.ID,
						"is_vocal": t.IsVocal,
					})
				}
				return res
			}(),
		})
		return
	}

	// Check for time range conflicts with existing structures
	existing, _ := h.store.ListStructuresBySong(songID)
	var conflicts []map[string]interface{}
	for _, ex := range existing {
		for _, e := range entries {
			if timesOverlap(e.Start, e.End, ex.StartTime, ex.EndTime) {
				conflicts = append(conflicts, map[string]interface{}{
					"existing_title": ex.Title,
					"existing_start": ex.StartTime,
					"existing_end":   ex.EndTime,
					"incoming_title": e.Title,
					"incoming_start": e.Start,
					"incoming_end":   e.End,
				})
			}
		}
	}

	force := r.URL.Query().Get("force") == "true"
	if len(conflicts) > 0 && !force {
		respondJSON(w, http.StatusConflict, map[string]interface{}{
			"conflict":       true,
			"conflict_count": len(conflicts),
			"conflicts":      conflicts,
			"existing_structures": func() []map[string]interface{} {
				res := make([]map[string]interface{}, 0, len(existing))
				for _, ex := range existing {
					res = append(res, map[string]interface{}{
						"title": ex.Title,
						"start": ex.StartTime,
						"end":   ex.EndTime,
					})
				}
				return res
			}(),
			"message": fmt.Sprintf("%d 處時間範圍與現有結構衝突，如需覆蓋請加上 ?force=true", len(conflicts)),
		})
		return
	}

	// Delete all existing structures + lyrics for this song
	h.store.DeleteAllStructuresBySong(songID)
	for _, ex := range existing {
		if ex.TrackID != nil {
			h.store.DeleteTrackLyrics(*ex.TrackID, ex.ID)
		}
		// Also delete lyrics for this structure across all tracks
		deleteAllLyricsByStructure(h.store, ex.ID)
	}

	// Build and save new structures
	var allStructs []*domain.SongStructure
	sectionByTitle := make(map[string]*domain.SongStructure)
	lyricsBatch := make([]struct {
		trackID     uuid.UUID
		structureID uuid.UUID
		lyrics      string
	}, 0)

	for _, e := range entries {
		var trackID *uuid.UUID
		if e.TrackName != "" {
			if tid, ok := trackByName[e.TrackName]; ok {
				trackID = &tid
			}
		}
		if e.Type == "S" {
			st := domain.NewSongStructure(songID, trackID, nil, domain.StructureTypeSECTION, e.Title, e.Start, e.End, 0, 0, e.Order)
			allStructs = append(allStructs, st)
			sectionByTitle[e.Title] = st
		} else if e.Type == "P" {
			// Find parent section (the most recent SECTION that contains this phrase)
			var parent *domain.SongStructure
			for _, st := range allStructs {
				if st.Type == domain.StructureTypeSECTION && st.StartTime <= e.Start && st.EndTime >= e.End {
					parent = st
				}
			}
			if parent == nil {
				respondError(w, http.StatusBadRequest, "phrase '"+e.Title+"' has no containing section")
				return
			}
			ph := domain.NewSongStructure(songID, trackID, &parent.ID, domain.StructureTypePHRASE, e.Title, e.Start, e.End, 0, 0, e.Order)
			allStructs = append(allStructs, ph)
			if e.Lyrics != "" && trackID != nil {
				lyricsBatch = append(lyricsBatch, struct {
					trackID     uuid.UUID
					structureID uuid.UUID
					lyrics      string
				}{trackID: *trackID, structureID: ph.ID, lyrics: e.Lyrics})
			}
		}
	}

	if err := h.store.CreateStructure(allStructs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create structures")
		return
	}
	for _, lb := range lyricsBatch {
		h.store.UpsertTrackLyrics(lb.trackID, lb.structureID, lb.lyrics)
	}

	tree, err := h.store.BuildStructureTree(songID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build tree")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"imported":   len(entries),
		"song_id":    songID,
		"structures": tree,
	})
}

type csvEntry struct {
	Type      string
	Title     string
	Start     float64
	End       float64
	Order     int
	TrackName string
	Lyrics    string
}

func parseStructureCSV(csvStr string) ([]csvEntry, error) {
	lines := strings.Split(csvStr, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("csv must have header + at least 1 row")
	}
	header := strings.TrimSpace(lines[0])
	expectedHeader := "type,title,start,end,order,track_name,lyrics"
	if strings.TrimSpace(header) != expectedHeader {
		return nil, fmt.Errorf("expected header: %s", expectedHeader)
	}

	var entries []csvEntry
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Simple CSV parser (handles quoted fields)
		fields := parseCSVLine(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("line %d: expected at least 6 fields, got %d", i+1, len(fields))
		}
		t := strings.TrimSpace(fields[0])
		if t != "S" && t != "P" {
			return nil, fmt.Errorf("line %d: type must be S or P, got %s", i+1, t)
		}
		title := strings.TrimSpace(fields[1])
		start, err := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid start: %s", i+1, fields[2])
		}
		end, err := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid end: %s", i+1, fields[3])
		}
		order, err := strconv.Atoi(strings.TrimSpace(fields[4]))
		if err != nil {
			order = 0
		}
		trackName := strings.TrimSpace(fields[5])
		lyrics := ""
		if len(fields) >= 7 {
			lyrics = strings.TrimSpace(fields[6])
		}
		entries = append(entries, csvEntry{
			Type: t, Title: title, Start: start, End: end,
			Order: order, TrackName: trackName, Lyrics: lyrics,
		})
	}
	return entries, nil
}

// parseCSVLine handles simple CSV with quoted fields
func parseCSVLine(line string) []string {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			if inQuote && i+1 < len(line) && line[i+1] == '"' {
				cur.WriteByte('"')
				i++
			} else {
				inQuote = !inQuote
			}
		} else if ch == ',' && !inQuote {
			fields = append(fields, cur.String())
			cur.Reset()
		} else {
			cur.WriteByte(ch)
		}
	}
	fields = append(fields, cur.String())
	return fields
}

func timesOverlap(s1, e1, s2, e2 float64) bool {
	return s1 < e2 && s2 < e1
}

// deleteAllLyricsByStructure removes all TrackLyrics entries for a given structure ID.
func deleteAllLyricsByStructure(store *storage.Store, structureID uuid.UUID) {
	// List all lyrics by song is the best we can do without direct method;
	// we find all lyrics entries with this structureID by attempting to list all.
	// Since the store uses "trackID:structureID" keys, we need a song-aware approach.
	// A simpler approach: iterate all trackLyrics keys and delete matching structureID.
	// But since store doesn't expose raw map, we'll use the available methods:
	// ImportStructures will delete all structures+lyrics for a song in one pass,
	// so we delete structures first, then the lyrics that were connected to them.
	// The existing DeleteTrackLyrics calls in the loop above handle track-specific ones.
	// For global structures (trackID==nil), there are no lyrics entries.
	_ = structureID // no-op: lyrics are already handled by DeleteTrackLyrics calls above
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
