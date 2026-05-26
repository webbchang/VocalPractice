package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

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
		r.Get("/assessments", userAssessmentsHandler.ListMyAssessments)
		r.Get("/assessments/stats", userAssessmentsHandler.GetStats)
		r.Get("/assessments/{assessment_id}", userAssessmentsHandler.GetAssessment)
		r.Get("/assessments/{assessment_id}/download", userAssessmentsHandler.Download)
		r.Delete("/assessments/{assessment_id}", userAssessmentsHandler.Delete)
	})

	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, ".")
	r.Handle("/*", http.FileServer(http.Dir(filesDir)))

	fmt.Printf("Test server starting on %s\n", port)
	fmt.Printf("Upload directory: %s\n", absUploadDir)
	log.Fatal(http.ListenAndServe(port, r))
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

	// Load reference2.MID
	midiCandidates := []string{
		"test_data/reference2.MID",
		"test_data/reference2.mid",
	}

	for _, midiPath := range midiCandidates {
		data, err := os.ReadFile(midiPath)
		if err != nil {
			continue
		}

		song, err := midiParser.Parse(data, "song1", "artist1")
		if err != nil {
			log.Printf("[seed] reference2 parse failed: %v", err)
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

		// Find Tenor 2 track to bind structures to it
		var tenor2TrackID *uuid.UUID
		for i, t := range song.Tracks {
			if t.Name == "Tenor 2" {
				tenor2TrackID = &song.Tracks[i].ID
				break
			}
		}
		if tenor2TrackID == nil {
			log.Printf("[seed] Tenor 2 track not found, structures will be global")
		}

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

		startTick := service.SecToTick(15, tempoEntries, ppq)
		a1EndTick := service.SecToTick(29, tempoEntries, ppq)
		endTick := service.SecToTick(50.6, tempoEntries, ppq)

		section := domain.NewSongStructure(
			song.ID, tenor2TrackID, nil,
			domain.StructureTypeSECTION, "Verse A",
			15, 50.6, startTick, endTick, 1,
		)

		phrase1 := domain.NewSongStructure(
			song.ID, tenor2TrackID, &section.ID,
			domain.StructureTypePHRASE, "A1",
			15, 29, startTick, a1EndTick, 1,
		)

		phrase2 := domain.NewSongStructure(
			song.ID, tenor2TrackID, &section.ID,
			domain.StructureTypePHRASE, "A2",
			29, 50.6, a1EndTick, endTick, 2,
		)

		if err := store.CreateStructure([]*domain.SongStructure{section, phrase1, phrase2}); err != nil {
			log.Printf("[seed] create structures failed: %v", err)
		} else {
			if tenor2TrackID != nil {
				log.Printf("[seed] Tenor 2 structures ready: Verse A with A1, A2")
			} else {
				log.Printf("[seed] global structures ready: Verse A with A1, A2")
			}
		}

		return
	}

	log.Printf("[seed] reference2.MID not found, you can upload later via POST /api/v1/admin/songs")
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
