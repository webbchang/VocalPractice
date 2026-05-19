package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"vocal-practice-app/internal/service"
	"vocal-practice-app/internal/storage"

	"encoding/json"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminSongsHandler struct {
	store      *storage.Store
	midiParser *service.MIDIParser
	uploadDir  string
}

func NewAdminSongsHandler(store *storage.Store, midiParser *service.MIDIParser, uploadDir string) *AdminSongsHandler {
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
		"song_id": song.ID,
		"title":   song.Title,
		"artist":  song.Artist,
		"tracks":  tracks,
	})
}

func (h *AdminSongsHandler) ListSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := h.store.ListSongs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list songs")
		return
	}

	type songSummary struct {
		ID     uuid.UUID `json:"id"`
		Title  string    `json:"title"`
		Artist string    `json:"artist"`
		Tracks int       `json:"track_count"`
	}

	result := make([]songSummary, 0, len(songs))
	for _, s := range songs {
		result = append(result, songSummary{
			ID:     s.ID,
			Title:  s.Title,
			Artist: s.Artist,
			Tracks: len(s.Tracks),
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
	})
}
