package main

import (
	"crypto/sha256"
	"encoding/base64"
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

	store := storage.New()
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
		r.Post("/users", adminUsersHandler.CreateUser)
		r.Get("/users", adminUsersHandler.ListUsers)
		r.Put("/users/{user_id}/toggle-active", adminUsersHandler.ToggleUserActive)

		r.Post("/songs", adminSongsHandler.UploadSong)
		r.Get("/songs", adminSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", adminSongsHandler.GetSong)
		r.Patch("/songs/{song_id}/tracks/{track_id}", adminSongsHandler.UpdateTrack)
		r.Delete("/songs/{song_id}", adminSongsHandler.DeleteSong)

		r.Post("/songs/{song_id}/structures", adminStructuresHandler.BulkCreate)
		r.Put("/songs/{song_id}/structures/{structure_id}", adminStructuresHandler.Update)
		r.Delete("/songs/{song_id}/structures/{structure_id}", adminStructuresHandler.Delete)

		r.Post("/songs/{song_id}/lyrics", adminLyricsHandler.BatchUpsert)
		r.Get("/tracks/{track_id}/lyrics", adminLyricsHandler.GetByTrack)
		r.Delete("/songs/{song_id}/lyrics", adminLyricsHandler.Delete)
	})

	r.Post("/api/v1/auth/login", authHandler.Login)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/songs", userSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", userSongsHandler.GetSong)
		r.Get("/songs/{song_id}/midi", userSongsHandler.DownloadMIDI)
		r.Get("/songs/{song_id}/structures", userSongsHandler.GetStructures)
		r.Get("/songs/{song_id}/tracks/{track_id}/audio", trackAudioHandler.ServeTrackAudio)

		r.Group(func(r chi.Router) {
			r.Use(authHandler.Middleware)
			r.Post("/assessments/submit", userAssessmentsHandler.Submit)
			r.Get("/assessments", userAssessmentsHandler.ListMyAssessments)
			r.Get("/assessments/stats", userAssessmentsHandler.GetStats)
			r.Get("/assessments/{assessment_id}", userAssessmentsHandler.GetAssessment)
			r.Get("/assessments/{assessment_id}/download", userAssessmentsHandler.Download)
			r.Delete("/assessments/{assessment_id}", userAssessmentsHandler.Delete)
		})
	})

	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, ".")
	r.Handle("/*", http.FileServer(http.Dir(filesDir)))

	fmt.Printf("Test server starting on %s\n", port)
	fmt.Printf("Upload directory: %s\n", absUploadDir)
	log.Fatal(http.ListenAndServe(port, r))
}

func seedTestData(store *storage.Store, midiParser *service.MIDIParser, uploadDir string) {
	user := domain.NewUser("webbchang", "webbchang@gmail.com", hashPassword("test1234"))
	if err := store.CreateUser(user); err != nil {
		log.Printf("[seed] create user skipped: %v", err)
	} else {
		log.Printf("[seed] user ready: %s / test1234", user.Email)
	}

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

		// Seed global structures visible to all tracks
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
			song.ID, nil, nil,
			domain.StructureTypeSECTION, "Verse A",
			15, 50.6, startTick, endTick, 1,
		)

		phrase1 := domain.NewSongStructure(
			song.ID, nil, &section.ID,
			domain.StructureTypePHRASE, "A1",
			15, 29, startTick, a1EndTick, 1,
		)

		phrase2 := domain.NewSongStructure(
			song.ID, nil, &section.ID,
			domain.StructureTypePHRASE, "A2",
			29, 50.6, a1EndTick, endTick, 2,
		)

		if err := store.CreateStructure([]*domain.SongStructure{section, phrase1, phrase2}); err != nil {
			log.Printf("[seed] create structures failed: %v", err)
		} else {
			log.Printf("[seed] global structures ready: Verse A with A1, A2")
		}

		return
	}

	log.Printf("[seed] reference2.MID not found, you can upload later via POST /api/v1/admin/songs")
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return base64.RawURLEncoding.EncodeToString(h[:])
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
