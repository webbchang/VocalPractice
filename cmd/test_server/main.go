package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/handler"
	"vocal-practice-app/internal/service"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	port := ":18080"
	uploadDir := os.Getenv("TEST_UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads_test"
	}
	jwtSecret := os.Getenv("TEST_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-server-secret"
	}

	// HTTPS configuration via environment variables
	tlsCert := os.Getenv("TEST_TLS_CERT")
	tlsKey := os.Getenv("TEST_TLS_KEY")
	httpsPort := os.Getenv("TEST_HTTPS_PORT")
	if httpsPort == "" {
		httpsPort = ":18443"
	}
	// If TLS is enabled, optionally redirect HTTP traffic to HTTPS
	redirectHTTP := os.Getenv("TEST_REDIRECT_HTTP_TO_HTTPS") == "true"

	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		log.Fatalf("failed to resolve upload directory: %v", err)
	}
	if err := os.MkdirAll(absUploadDir, 0755); err != nil {
		log.Fatalf("failed to create upload directory: %v", err)
	}

	// Initialize store: PostgreSQL or in-memory
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	var store storage.Store
	if databaseURL != "" {
		pgStore, err := storage.NewPostgresStore(databaseURL)
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		defer pgStore.Close()
		store = pgStore
		log.Printf("Using PostgreSQL store: %s", databaseURL)
	} else {
		store = storage.NewMemoryStore()
		log.Println("DATABASE_URL not set, using in-memory store")
	}

	midiParser := service.NewMIDIParser()

	seedTestData(store, midiParser, absUploadDir)

	authHandler := handler.NewAuthHandler(store, jwtSecret)
	adminUsersHandler := handler.NewAdminUsersHandler(store)
	adminSongsHandler := handler.NewAdminSongsHandler(store, midiParser, absUploadDir)
	adminStructuresHandler := handler.NewAdminStructuresHandler(store)
	adminLyricsHandler := handler.NewAdminLyricsHandler(store)
	userSongsHandler := handler.NewUserSongsHandler(store, absUploadDir)
	userAssessmentsHandler := handler.NewUserAssessmentsHandler(store)
	trackAudioHandler := handler.NewTrackAudioHandler(store)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","server":"test"}`))
	})

	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(authHandler.Middleware)
		r.Use(authHandler.RequireRole("admin"))

		r.Post("/users", adminUsersHandler.CreateUser)
		r.Get("/users", adminUsersHandler.ListUsers)
		r.Put("/users/{user_id}/toggle-active", adminUsersHandler.ToggleUserActive)

		r.Post("/songs", adminSongsHandler.UploadSong)
		r.Get("/songs", adminSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", adminSongsHandler.GetSong)
		r.Patch("/songs/{song_id}/tracks/{track_id}", adminSongsHandler.UpdateTrack)
		r.Delete("/songs/{song_id}", adminSongsHandler.DeleteSong)
		r.Post("/songs/{song_id}/new-version", adminSongsHandler.CreateNewVersion)
		r.Put("/songs/{song_id}/set-active", adminSongsHandler.SetActiveVersion)
		r.Get("/songs/{song_id}/versions", adminSongsHandler.ListVersions)

		r.Post("/songs/{song_id}/structures", adminStructuresHandler.BulkCreate)
		r.Get("/songs/{song_id}/structures/export", adminStructuresHandler.ExportStructures)
		r.Post("/songs/{song_id}/structures/import", adminStructuresHandler.ImportStructures)
		r.Post("/songs/{song_id}/structures/copy-from/{source_song_id}", adminStructuresHandler.CopyStructuresFromSong)
		r.Post("/songs/{song_id}/structures/{section_id}/copy-phrases", adminStructuresHandler.CopySectionPhrases)
		r.Put("/songs/{song_id}/structures/{structure_id}", adminStructuresHandler.Update)
		r.Delete("/songs/{song_id}/structures/{structure_id}", adminStructuresHandler.Delete)

		r.Post("/songs/{song_id}/lyrics", adminLyricsHandler.BatchUpsert)
		r.Get("/tracks/{track_id}/lyrics", adminLyricsHandler.GetByTrack)
		r.Delete("/songs/{song_id}/lyrics", adminLyricsHandler.Delete)
	})

	r.Post("/api/v1/auth/login", authHandler.Login)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authHandler.Middleware)

		r.Get("/songs", userSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", userSongsHandler.GetSong)
		r.Get("/songs/{song_id}/midi", userSongsHandler.DownloadMIDI)
		r.Get("/songs/{song_id}/structures", userSongsHandler.GetStructures)
		r.Get("/songs/{song_id}/tracks/{track_id}/audio", trackAudioHandler.ServeTrackAudio)

		r.Post("/assessments/submit", userAssessmentsHandler.Submit)
		r.Post("/assessments/analyze", userAssessmentsHandler.AnalyzeRecording)
		r.Get("/assessments", userAssessmentsHandler.ListMyAssessments)
		r.Get("/assessments/stats", userAssessmentsHandler.GetStats)
		r.Get("/assessments/{assessment_id}", userAssessmentsHandler.GetAssessment)
		r.Get("/assessments/{assessment_id}/download", userAssessmentsHandler.Download)
		r.Delete("/assessments/{assessment_id}", userAssessmentsHandler.Delete)
	})

	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, ".")
	r.Handle("/*", http.FileServer(http.Dir(filesDir)))

	fmt.Printf("Upload directory: %s\n", absUploadDir)

	// Start server with HTTPS if TLS cert/key are provided, otherwise HTTP
	if tlsCert != "" && tlsKey != "" {
		fmt.Printf("Test server starting on %s (HTTPS)\n", httpsPort)
		if redirectHTTP {
			// Start HTTP server that redirects to HTTPS
			go func() {
				redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					host := req.Host
					// Replace the HTTP port with the HTTPS port
					redirectURL := "https://" + host + req.URL.RequestURI()
					http.Redirect(w, req, redirectURL, http.StatusMovedPermanently)
				})
				fmt.Printf("HTTP redirect server starting on %s -> %s\n", port, httpsPort)
				log.Fatal(http.ListenAndServe(port, redirectHandler))
			}()
		}
		log.Fatal(http.ListenAndServeTLS(httpsPort, tlsCert, tlsKey, r))
	} else {
		fmt.Printf("Test server starting on %s (HTTP)\n", port)
		log.Fatal(http.ListenAndServe(port, r))
	}
}

func seedTestData(store storage.Store, midiParser *service.MIDIParser, uploadDir string) {
	// Admin: admin / admin@localhost / a1d2m3i4n
	adminPwd, _ := bcrypt.GenerateFromPassword([]byte("a1d2m3i4n"), bcrypt.DefaultCost)
	admin := domain.NewUser("admin", "admin@localhost", string(adminPwd))
	if err := store.CreateUser(admin); err != nil {
		// May already exist from MemoryStore seed; update password to bcrypt
		existing, _ := store.GetUserByEmail("admin@localhost")
		if existing != nil {
			existing.PasswordHash = string(adminPwd)
			store.UpdateUser(existing)
			log.Printf("[seed] admin password updated to bcrypt")
		} else {
			log.Printf("[seed] create admin skipped: %v", err)
		}
	} else {
		log.Printf("[seed] admin ready: admin@localhost / a1d2m3i4n")
	}

	// User: webb / webbchang@gmail.com / test1234
	userPwd, _ := bcrypt.GenerateFromPassword([]byte("test1234"), bcrypt.DefaultCost)
	user := domain.NewUser("webb", "webbchang@gmail.com", string(userPwd))
	if err := store.CreateUser(user); err != nil {
		existing, _ := store.GetUserByEmail("webbchang@gmail.com")
		if existing != nil {
			existing.PasswordHash = string(userPwd)
			store.UpdateUser(existing)
			log.Printf("[seed] user password updated to bcrypt")
		} else {
			log.Printf("[seed] create user skipped: %v", err)
		}
	} else {
		log.Printf("[seed] user ready: webbchang@gmail.com / test1234")
	}

	// Load 油桐花 (Josu Elberdin)
	midiPath := "test_data/all_油桐花_hidden (2).mid"
	data, err := os.ReadFile(midiPath)
	if err != nil {
		log.Printf("[seed] 油桐花 MIDI not found: %v, you can upload later via POST /api/v1/admin/songs", err)
		return
	}

	song, err := midiParser.Parse(data, "油桐花", "Josu Elberdin")
	if err != nil {
		log.Printf("[seed] 油桐花 parse failed: %v", err)
		return
	}

	filename := song.ID.String() + ".mid"
	filePath := filepath.Join(uploadDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("[seed] write midi failed: %v", err)
		return
	}

	song.MIDIFilePath = filePath
	if err := store.CreateSong(song); err != nil {
		log.Printf("[seed] create song failed: %v", err)
		return
	}

	log.Printf("[seed] song ready: %s - %s (%s)", song.Title, song.Artist, midiPath)

	// Print track names for debugging
	var trackNames []string
	for _, t := range song.Tracks {
		trackNames = append(trackNames, t.Name)
	}
	trackNamesJSON, _ := json.Marshal(trackNames)
	log.Printf("[seed] tracks: %s", string(trackNamesJSON))

	// Load structures from CSV
	csvPath := "test_data/油桐花-structures.csv"
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		log.Printf("[seed] 油桐花 structures CSV not found: %v", err)
		return
	}

	entries, err := parseSeedStructureCSV(string(csvData))
	if err != nil {
		log.Printf("[seed] parse structures CSV failed: %v", err)
		return
	}

	// Resolve track names to track IDs
	trackByName := make(map[string]uuid.UUID)
	for _, t := range song.Tracks {
		trackByName[t.Name] = t.ID
	}

	// Build tempo map for tick calculation
	tempoEntries := make([]service.TempoEntry, len(song.TempoMap))
	for i, te := range song.TempoMap {
		tempoEntries[i] = service.TempoEntry{
			Tick:        te.Tick,
			TimeSec:     te.TimeSec,
			TempoUSecQN: te.TempoUSecQN,
		}
	}

	ppq := song.TicksPerQuarter
	if ppq == 0 {
		ppq = 480
	}

	// Build structures grouped by track
	var allStructs []*domain.SongStructure
	var lyricsBatch []struct {
		trackID     uuid.UUID
		structureID uuid.UUID
		lyrics      string
	}

	// Group entries by track name, preserving CSV order
	type trackGroup struct {
		trackName string
		entries   []seedCSVEntry
	}
	var groups []trackGroup
	groupIndex := make(map[string]int)
	for _, e := range entries {
		idx, ok := groupIndex[e.TrackName]
		if !ok {
			idx = len(groups)
			groupIndex[e.TrackName] = idx
			groups = append(groups, trackGroup{trackName: e.TrackName})
		}
		groups[idx].entries = append(groups[idx].entries, e)
	}

	for _, g := range groups {
		var trackID *uuid.UUID
		if g.trackName != "" {
			if tid, ok := trackByName[g.trackName]; ok {
				trackID = &tid
			} else {
				log.Printf("[seed] track '%s' not found in song, skipping its structures", g.trackName)
				continue
			}
		}

		// Build sections first, then phrases
		var sections []seedCSVEntry
		var phrases []seedCSVEntry
		for _, e := range g.entries {
			if e.Type == "S" {
				sections = append(sections, e)
			} else {
				phrases = append(phrases, e)
			}
		}

		// Create sections
		for _, e := range sections {
			st := domain.NewSongStructure(
				song.ID, trackID, nil,
				domain.StructureTypeSECTION, e.Title,
				e.Start, e.End,
				service.SecToTick(e.Start, tempoEntries, ppq),
				service.SecToTick(e.End, tempoEntries, ppq),
				e.Order,
			)
			allStructs = append(allStructs, st)
		}

		// Create phrases, linking to containing section
		for _, e := range phrases {
			var parent *domain.SongStructure
			for _, st := range allStructs {
				if st.Type == domain.StructureTypeSECTION && st.StartTime <= e.Start && st.EndTime >= e.End {
					parent = st
				}
			}
			if parent == nil {
				log.Printf("[seed] phrase '%s' has no containing section, skipping", e.Title)
				continue
			}
			ph := domain.NewSongStructure(
				song.ID, trackID, &parent.ID,
				domain.StructureTypePHRASE, e.Title,
				e.Start, e.End,
				service.SecToTick(e.Start, tempoEntries, ppq),
				service.SecToTick(e.End, tempoEntries, ppq),
				e.Order,
			)
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

	if len(allStructs) > 0 {
		if err := store.CreateStructure(allStructs); err != nil {
			log.Printf("[seed] create structures failed: %v", err)
		} else {
			log.Printf("[seed] structures ready: %d structures imported from %s", len(allStructs), csvPath)
		}
	}

	// Save lyrics
	for _, lb := range lyricsBatch {
		if err := store.UpsertTrackLyrics(lb.trackID, lb.structureID, lb.lyrics); err != nil {
			log.Printf("[seed] upsert lyrics failed: %v", err)
		}
	}
	if len(lyricsBatch) > 0 {
		log.Printf("[seed] lyrics ready: %d lyrics entries", len(lyricsBatch))
	}
}

type seedCSVEntry struct {
	Type      string
	Title     string
	Start     float64
	End       float64
	Order     int
	TrackName string
	Lyrics    string
}

func parseSeedStructureCSV(csvStr string) ([]seedCSVEntry, error) {
	lines := strings.Split(csvStr, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("csv must have header + at least 1 row")
	}
	header := strings.TrimSpace(lines[0])
	expectedHeader := "type,title,start,end,order,track_name,lyrics"
	if strings.TrimSpace(header) != expectedHeader {
		return nil, fmt.Errorf("expected header: %s", expectedHeader)
	}

	var entries []seedCSVEntry
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Skip comment lines (e.g. "# Track: Vocal")
		if line[0] == '#' {
			continue
		}
		// Simple CSV parser (handles quoted fields)
		fields := parseSeedCSVLine(line)
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
		entries = append(entries, seedCSVEntry{
			Type: t, Title: title, Start: start, End: end,
			Order: order, TrackName: trackName, Lyrics: lyrics,
		})
	}
	return entries, nil
}

// parseSeedCSVLine handles simple CSV with quoted fields
func parseSeedCSVLine(line string) []string {
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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
