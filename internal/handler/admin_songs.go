package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/service"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminSongsHandler struct {
	store      storage.Store
	midiParser *service.MIDIParser
	uploadDir  string
}

func NewAdminSongsHandler(store storage.Store, midiParser *service.MIDIParser, uploadDir string) *AdminSongsHandler {
	os.MkdirAll(uploadDir, 0755)
	return &AdminSongsHandler{
		store:      store,
		midiParser: midiParser,
		uploadDir:  uploadDir,
	}
}

func (h *AdminSongsHandler) UploadSong(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	title := r.FormValue("title")
	artist := r.FormValue("artist")
	if title == "" || artist == "" {
		respondError(w, http.StatusBadRequest, "title and artist required")
		return
	}

	file, header, err := r.FormFile("midi_file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "midi_file required")
		return
	}
	defer file.Close()

	data := make([]byte, header.Size)
	if _, err := file.Read(data); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Parse MIDI
	song, err := h.midiParser.Parse(data, title, artist)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid MIDI file: "+err.Error())
		return
	}

	// Check if this is a new version of an existing song
	sourceSongIDStr := r.FormValue("source_song_id")
	if sourceSongIDStr != "" {
		sourceSongID, err := uuid.Parse(sourceSongIDStr)
		if err == nil {
			sourceSong, err := h.store.GetSongByID(sourceSongID)
			if err == nil {
				// Inherit version group from source
				song.VersionGroup = sourceSong.VersionGroup
				song.VersionLabel = nextVersionLabel(sourceSong.VersionLabel)

				// Deactivate source song
				h.store.SetActiveVersion(sourceSong.VersionGroup, song.ID)
			}
		}
	}

	// Save file to disk
	filename := song.ID.String() + ".mid"
	filePath := filepath.Join(h.uploadDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	song.MIDIFilePath = filePath

	// Store in memory
	if err := h.store.CreateSong(song); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store song")
		return
	}

	// Auto-copy structures from source song if this is a new version
	sourceSongIDStr = r.FormValue("source_song_id")
	if sourceSongIDStr != "" {
		sourceSongID, _ := uuid.Parse(sourceSongIDStr)
		copyStructuresInternal(h.store, sourceSongID, song.ID)
	}

	type trackSummary struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		NoteCount   int       `json:"note_count"`
		DurationSec float64   `json:"duration_sec"`
		IsVocal     bool      `json:"is_vocal"`
	}

	tracks := make([]trackSummary, 0, len(song.Tracks))
	for _, t := range song.Tracks {
		var maxEnd float64
		for _, n := range t.Notes {
			if n.EndTime > maxEnd {
				maxEnd = n.EndTime
			}
		}
		tracks = append(tracks, trackSummary{
			ID:          t.ID,
			Name:        t.Name,
			NoteCount:   len(t.Notes),
			DurationSec: maxEnd,
			IsVocal:     t.IsVocal,
		})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"song_id":       song.ID,
		"title":         song.Title,
		"artist":        song.Artist,
		"tracks":        tracks,
		"version_group": song.VersionGroup,
		"version_label": song.VersionLabel,
		"is_active":     song.IsActive,
	})
}

func (h *AdminSongsHandler) CreateNewVersion(w http.ResponseWriter, r *http.Request) {
	sourceSongIDStr := chi.URLParam(r, "song_id")
	sourceSongID, err := uuid.Parse(sourceSongIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	sourceSong, err := h.store.GetSongByID(sourceSongID)
	if err != nil {
		respondError(w, http.StatusNotFound, "source song not found")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, header, err := r.FormFile("midi_file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "midi_file required")
		return
	}
	defer file.Close()

	data := make([]byte, header.Size)
	if _, err := file.Read(data); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Parse MIDI with same title/artist as source
	newSong, err := h.midiParser.Parse(data, sourceSong.Title, sourceSong.Artist)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid MIDI file: "+err.Error())
		return
	}

	// Set version info
	newSong.VersionGroup = sourceSong.VersionGroup
	newSong.VersionLabel = nextVersionLabel(sourceSong.VersionLabel)
	newSong.IsActive = true

	// Save file
	filename := newSong.ID.String() + ".mid"
	filePath := filepath.Join(h.uploadDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	newSong.MIDIFilePath = filePath

	// Store new song
	if err := h.store.CreateSong(newSong); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store new song")
		return
	}

	// Deactivate old version, activate new one
	h.store.SetActiveVersion(sourceSong.VersionGroup, newSong.ID)

	// Copy structures from source to new song
	copyStructuresInternal(h.store, sourceSongID, newSong.ID)

	type trackSummary struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		NoteCount   int       `json:"note_count"`
		DurationSec float64   `json:"duration_sec"`
		IsVocal     bool      `json:"is_vocal"`
	}

	tracks := make([]trackSummary, 0, len(newSong.Tracks))
	for _, t := range newSong.Tracks {
		var maxEnd float64
		for _, n := range t.Notes {
			if n.EndTime > maxEnd {
				maxEnd = n.EndTime
			}
		}
		tracks = append(tracks, trackSummary{
			ID:          t.ID,
			Name:        t.Name,
			NoteCount:   len(t.Notes),
			DurationSec: maxEnd,
			IsVocal:     t.IsVocal,
		})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"song_id":       newSong.ID,
		"title":         newSong.Title,
		"artist":        newSong.Artist,
		"version_group": newSong.VersionGroup,
		"version_label": newSong.VersionLabel,
		"is_active":     newSong.IsActive,
		"tracks":        tracks,
	})
}

func (h *AdminSongsHandler) SetActiveVersion(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	song, err := h.store.GetSongByID(songID)
	if err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	if err := h.store.SetActiveVersion(song.VersionGroup, songID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to set active version")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"song_id":       songID,
		"version_group": song.VersionGroup,
		"version_label": song.VersionLabel,
		"is_active":     true,
	})
}

func (h *AdminSongsHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	song, err := h.store.GetSongByID(songID)
	if err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	versions, err := h.store.ListVersionsByGroup(song.VersionGroup)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list versions")
		return
	}

	type versionEntry struct {
		ID           uuid.UUID `json:"id"`
		VersionLabel string    `json:"version_label"`
		IsActive     bool      `json:"is_active"`
		CreatedAt    string    `json:"created_at"`
		TrackCount   int       `json:"track_count"`
	}

	result := make([]versionEntry, 0, len(versions))
	for _, v := range versions {
		result = append(result, versionEntry{
			ID:           v.ID,
			VersionLabel: v.VersionLabel,
			IsActive:     v.IsActive,
			CreatedAt:    v.CreatedAt.Format("2006-01-02 15:04:05"),
			TrackCount:   len(v.Tracks),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"song_id":       songID,
		"title":         song.Title,
		"artist":        song.Artist,
		"version_group": song.VersionGroup,
		"versions":      result,
	})
}

func (h *AdminSongsHandler) ListSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := h.store.ListSongs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list songs")
		return
	}

	type songSummary struct {
		ID           uuid.UUID `json:"id"`
		Title        string    `json:"title"`
		Artist       string    `json:"artist"`
		Tracks       int       `json:"track_count"`
		VersionGroup uuid.UUID `json:"version_group"`
		VersionLabel string    `json:"version_label"`
		IsActive     bool      `json:"is_active"`
	}

	result := make([]songSummary, 0, len(songs))
	for _, s := range songs {
		result = append(result, songSummary{
			ID:           s.ID,
			Title:        s.Title,
			Artist:       s.Artist,
			Tracks:       len(s.Tracks),
			VersionGroup: s.VersionGroup,
			VersionLabel: s.VersionLabel,
			IsActive:     s.IsActive,
		})
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *AdminSongsHandler) DeleteSong(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	if err := h.store.DeleteSong(songID); err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AdminSongsHandler) UpdateTrack(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	trackIDStr := chi.URLParam(r, "track_id")
	trackID, err := uuid.Parse(trackIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid track_id")
		return
	}

	var req struct {
		IsVocal *bool `json:"is_vocal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if req.IsVocal != nil {
		updates["is_vocal"] = *req.IsVocal
	}

	if len(updates) == 0 {
		respondError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	if err := h.store.UpdateTrack(songID, trackID, updates); err != nil {
		respondError(w, http.StatusNotFound, "song or track not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminSongsHandler) GetSong(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	song, err := h.store.GetSongByID(songID)
	if err != nil {
		respondError(w, http.StatusNotFound, "song not found")
		return
	}

	type trackInfo struct {
		ID      uuid.UUID `json:"id"`
		Name    string    `json:"name"`
		IsVocal bool      `json:"is_vocal"`
		Channel int       `json:"channel"`
		Notes   int       `json:"note_count"`
	}

	tracks := make([]trackInfo, 0, len(song.Tracks))
	for _, t := range song.Tracks {
		tracks = append(tracks, trackInfo{
			ID:      t.ID,
			Name:    t.Name,
			IsVocal: t.IsVocal,
			Channel: t.Channel,
			Notes:   len(t.Notes),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":            song.ID,
		"title":         song.Title,
		"artist":        song.Artist,
		"midi_file_url": "/api/v1/songs/" + song.ID.String() + "/midi",
		"tracks":        tracks,
		"version_group": song.VersionGroup,
		"version_label": song.VersionLabel,
		"is_active":     song.IsActive,
	})
}

// nextVersionLabel increments version label (v1 -> v2, or appends .1)
func nextVersionLabel(current string) string {
	if len(current) > 1 && current[0] == 'v' {
		if n, err := strconv.Atoi(current[1:]); err == nil {
			return "v" + strconv.Itoa(n+1)
		}
	}
	return current + ".1"
}

// copyStructuresInternal copies all structures + lyrics from source to target song
func copyStructuresInternal(store storage.Store, sourceSongID, targetSongID uuid.UUID) {
	sourceSong, err := store.GetSongByID(sourceSongID)
	if err != nil {
		return
	}
	targetSong, err := store.GetSongByID(targetSongID)
	if err != nil {
		return
	}

	sourceStructures, err := store.ListStructuresBySong(sourceSongID)
	if err != nil || len(sourceStructures) == 0 {
		return
	}

	svcTempoMap := convertTempoMap(targetSong.TempoMap)
	tpq := targetSong.TicksPerQuarter

	idMap := make(map[uuid.UUID]uuid.UUID, len(sourceStructures))
	newStructures := make([]*domain.SongStructure, 0, len(sourceStructures))

	for _, st := range sourceStructures {
		newID := uuid.New()
		idMap[st.ID] = newID

		newStartTick := service.SecToTick(st.StartTime, svcTempoMap, tpq)
		newEndTick := service.SecToTick(st.EndTime, svcTempoMap, tpq)

		var newParentID *uuid.UUID
		if st.ParentID != nil {
			if mapped, ok := idMap[*st.ParentID]; ok {
				newParentID = &mapped
			}
		}

		var newTrackID *uuid.UUID
		if st.TrackID != nil {
			for _, srcTrack := range sourceSong.Tracks {
				if srcTrack.ID == *st.TrackID {
					for _, tgtTrack := range targetSong.Tracks {
						if tgtTrack.Name == srcTrack.Name {
							newTrackID = &tgtTrack.ID
							break
						}
					}
					break
				}
			}
		}

		copied := &domain.SongStructure{
			ID:        newID,
			SongID:    targetSongID,
			TrackID:   newTrackID,
			ParentID:  newParentID,
			Type:      st.Type,
			Title:     st.Title,
			StartTime: st.StartTime,
			EndTime:   st.EndTime,
			StartTick: newStartTick,
			EndTick:   newEndTick,
			OrderIdx:  st.OrderIdx,
		}
		newStructures = append(newStructures, copied)
	}

	if err := store.CreateStructure(newStructures); err != nil {
		return
	}

	// Copy lyrics
	sourceLyrics, _ := store.ListTrackLyricsBySong(sourceSongID)
	for _, l := range sourceLyrics {
		newStructID, ok := idMap[l.StructureID]
		if !ok {
			continue
		}
		var newTrackID *uuid.UUID
		for _, srcTrack := range sourceSong.Tracks {
			if srcTrack.ID == l.TrackID {
				for _, tgtTrack := range targetSong.Tracks {
					if tgtTrack.Name == srcTrack.Name {
						newTrackID = &tgtTrack.ID
						break
					}
				}
				break
			}
		}
		if newTrackID != nil {
			store.UpsertTrackLyrics(*newTrackID, newStructID, l.Lyrics)
		}
	}
}
