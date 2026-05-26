package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserSongsHandler struct {
	store     storage.Store
	uploadDir string
}

func NewUserSongsHandler(store storage.Store, uploadDir string) *UserSongsHandler {
	return &UserSongsHandler{store: store, uploadDir: uploadDir}
}

func (h *UserSongsHandler) ListSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := h.store.ListActiveSongs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list songs")
		return
	}

	type trackSummary struct {
		ID      uuid.UUID `json:"id"`
		Name    string    `json:"name"`
		IsVocal bool      `json:"is_vocal"`
	}

	type songEntry struct {
		ID     uuid.UUID      `json:"id"`
		Title  string         `json:"title"`
		Artist string         `json:"artist"`
		Tracks []trackSummary `json:"tracks"`
	}

	result := make([]songEntry, 0, len(songs))
	for _, s := range songs {
		tracks := make([]trackSummary, 0, len(s.Tracks))
		for _, t := range s.Tracks {
			tracks = append(tracks, trackSummary{
				ID:      t.ID,
				Name:    t.Name,
				IsVocal: t.IsVocal,
			})
		}
		result = append(result, songEntry{
			ID:     s.ID,
			Title:  s.Title,
			Artist: s.Artist,
			Tracks: tracks,
		})
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *UserSongsHandler) GetSong(w http.ResponseWriter, r *http.Request) {
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
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		IsVocal   bool      `json:"is_vocal"`
		Channel   int       `json:"channel"`
		Notes     int       `json:"note_count"`
		MIDIIndex int       `json:"midi_index"`
	}

	tracks := make([]trackInfo, 0, len(song.Tracks))
	for _, t := range song.Tracks {
		tracks = append(tracks, trackInfo{
			ID:        t.ID,
			Name:      t.Name,
			IsVocal:   t.IsVocal,
			Channel:   t.Channel,
			Notes:     len(t.Notes),
			MIDIIndex: t.MIDIIndex,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":                song.ID,
		"title":             song.Title,
		"artist":            song.Artist,
		"midi_file_url":     "/api/v1/songs/" + song.ID.String() + "/midi",
		"tracks":            tracks,
		"tempo_map":         song.TempoMap,
		"ticks_per_quarter": song.TicksPerQuarter,
	})
}

func (h *UserSongsHandler) DownloadMIDI(w http.ResponseWriter, r *http.Request) {
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

	filePath := song.MIDIFilePath
	if filePath == "" {
		// Try to find in upload dir
		filePath = filepath.Join(h.uploadDir, songID.String()+".mid")
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "MIDI file not found on disk")
		return
	}

	w.Header().Set("Content-Type", "audio/midi")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+song.Title+".mid\"")
	http.ServeFile(w, r, filePath)
}

func (h *UserSongsHandler) GetStructures(w http.ResponseWriter, r *http.Request) {
	songIDStr := chi.URLParam(r, "song_id")
	songID, err := uuid.Parse(songIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	// If track_id is provided, build tree filtered by track
	var trackID *uuid.UUID
	trackIDStr := r.URL.Query().Get("track_id")
	if trackIDStr != "" {
		if tid, err := uuid.Parse(trackIDStr); err == nil {
			trackID = &tid
		}
	}

	var tree []*domain.StructureNode
	if trackID != nil {
		tree, err = h.store.BuildStructureTreeForTrack(songID, trackID)
	} else {
		tree, err = h.store.BuildStructureTree(songID)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build structure tree")
		return
	}

	if tree == nil {
		tree = make([]*domain.StructureNode, 0)
	}

	// Load lyrics if track_id query param is provided
	if trackID != nil {
		lyricsMap := make(map[uuid.UUID]string)
		lyrics, err := h.store.ListTrackLyricsByTrack(*trackID)
		if err == nil {
			for _, l := range lyrics {
				lyricsMap[l.StructureID] = l.Lyrics
			}
			// Inject lyrics into tree
			injectLyrics(tree, lyricsMap)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"song_id":    songID,
		"structures": tree,
	})
}

// injectLyrics recursively injects lyrics into structure nodes
func injectLyrics(nodes []*domain.StructureNode, lyricsMap map[uuid.UUID]string) {
	for _, node := range nodes {
		if lyrics, ok := lyricsMap[node.ID]; ok {
			node.Lyrics = lyrics
		}
		injectLyricsFromSlice(node.Phrases, lyricsMap)
	}
}

// injectLyricsFromSlice handles []domain.StructureNode (non-pointer slice)
func injectLyricsFromSlice(nodes []domain.StructureNode, lyricsMap map[uuid.UUID]string) {
	for i := range nodes {
		if lyrics, ok := lyricsMap[nodes[i].ID]; ok {
			nodes[i].Lyrics = lyrics
		}
		if len(nodes[i].Phrases) > 0 {
			injectLyricsFromSlice(nodes[i].Phrases, lyricsMap)
		}
	}
}
