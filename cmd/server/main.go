package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"vocal-practice-app/internal/handler"
	"vocal-practice-app/internal/service"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := ":8080"
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "vocal-practice-app-secret-key-change-in-production"
	}

	// Ensure upload directory exists
	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		log.Fatalf("failed to resolve upload directory: %v", err)
	}
	if err := os.MkdirAll(absUploadDir, 0755); err != nil {
		log.Fatalf("failed to create upload directory: %v", err)
	}

	// Initialize dependencies
	store := storage.New()
	midiParser := service.NewMIDIParser()

	// Initialize handlers
	authHandler := handler.NewAuthHandler(store, jwtSecret)
	adminUsersHandler := handler.NewAdminUsersHandler(store)
	adminSongsHandler := handler.NewAdminSongsHandler(store, midiParser, absUploadDir)
	adminStructuresHandler := handler.NewAdminStructuresHandler(store)
	adminLyricsHandler := handler.NewAdminLyricsHandler(store)
	userSongsHandler := handler.NewUserSongsHandler(store, absUploadDir)
	userAssessmentsHandler := handler.NewUserAssessmentsHandler(store)
	trackAudioHandler := handler.NewTrackAudioHandler(store)

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// === Admin API (no auth required for simplicity in demo) ===
	r.Route("/api/v1/admin", func(r chi.Router) {
		// Users
		r.Post("/users", adminUsersHandler.CreateUser)
		r.Get("/users", adminUsersHandler.ListUsers)
		r.Put("/users/{user_id}/toggle-active", adminUsersHandler.ToggleUserActive)

		// Songs (MIDI upload)
		r.Post("/songs", adminSongsHandler.UploadSong)
		r.Get("/songs", adminSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", adminSongsHandler.GetSong)
		r.Patch("/songs/{song_id}/tracks/{track_id}", adminSongsHandler.UpdateTrack)
		r.Delete("/songs/{song_id}", adminSongsHandler.DeleteSong)

		// Structures
		r.Post("/songs/{song_id}/structures", adminStructuresHandler.BulkCreate)
		r.Put("/songs/{song_id}/structures/{structure_id}", adminStructuresHandler.Update)
		r.Delete("/songs/{song_id}/structures/{structure_id}", adminStructuresHandler.Delete)

		// Lyrics
		r.Post("/songs/{song_id}/lyrics", adminLyricsHandler.BatchUpsert)
		r.Get("/tracks/{track_id}/lyrics", adminLyricsHandler.GetByTrack)
		r.Delete("/songs/{song_id}/lyrics", adminLyricsHandler.Delete)
	})

	// === Auth API ===
	r.Post("/api/v1/auth/login", authHandler.Login)

	// === User API (protected) ===
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints
		r.Get("/songs", userSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", userSongsHandler.GetSong)
		r.Get("/songs/{song_id}/midi", userSongsHandler.DownloadMIDI)
		r.Get("/songs/{song_id}/structures", userSongsHandler.GetStructures)
		r.Get("/songs/{song_id}/tracks/{track_id}/audio", trackAudioHandler.ServeTrackAudio)

		// Protected endpoints
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

	// Static file server for frontend
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, ".")
	r.Handle("/*", http.FileServer(http.Dir(filesDir)))

	fmt.Printf("Vocal Practice App server starting on %s\n", port)
	fmt.Printf("Upload directory: %s\n", absUploadDir)
	log.Fatal(http.ListenAndServe(port, r))
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
