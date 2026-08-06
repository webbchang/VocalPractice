package main

import (
	"flag"
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
	"golang.org/x/crypto/bcrypt"
)

func main() {
	port := ":8080"
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// HTTPS configuration via environment variables
	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")
	httpsPort := os.Getenv("HTTPS_PORT")
	if httpsPort == "" {
		httpsPort = ":8443"
	}
	// If TLS is enabled, optionally redirect HTTP traffic to HTTPS
	redirectHTTP := os.Getenv("REDIRECT_HTTP_TO_HTTPS") == "true"

	// Ensure upload directory exists
	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		log.Fatalf("failed to resolve upload directory: %v", err)
	}
	if err := os.MkdirAll(absUploadDir, 0755); err != nil {
		log.Fatalf("failed to create upload directory: %v", err)
	}

	// Parse command-line flags for default admin credentials
	adminEmail := flag.String("admin-email", "", "Default admin email (overrides seeded value)")
	adminPassword := flag.String("admin-password", "", "Default admin password (overrides seeded value)")
	flag.Parse()

	// Initialize store: PostgreSQL or in-memory
	databaseURL := os.Getenv("DATABASE_URL")
	var store storage.Store
	if databaseURL != "" {
		pgStore, err := storage.NewPostgresStore(databaseURL)
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		defer pgStore.Close()
		store = pgStore
		log.Println("Using PostgreSQL store")
	} else {
		store = storage.NewMemoryStore()
		log.Println("DATABASE_URL not set, using in-memory store")
	}

	// Override default admin credentials if flags provided
	if *adminEmail != "" && *adminPassword != "" {
		adminUser, err := store.GetUserByEmail(*adminEmail)
		if err != nil {
			// Try the default seeded admin email
			adminUser, err = store.GetUserByEmail("admin@localhost")
			if err != nil {
				log.Fatalf("admin user not found: cannot set credentials for %s", *adminEmail)
			}
			// Update email as well
			adminUser.Email = *adminEmail
		}
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte(*adminPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("failed to hash admin password: %v", err)
		}
		adminUser.PasswordHash = string(hashedPwd)
		if err := store.UpdateUser(adminUser); err != nil {
			log.Fatalf("failed to update admin credentials: %v", err)
		}
		log.Printf("Default admin credentials set: %s / %s", *adminEmail, *adminPassword)
	}

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

	// === Admin API (requires auth + admin role) ===
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(authHandler.Middleware)
		r.Use(authHandler.RequireRole("admin"))

		r.Post("/users", adminUsersHandler.CreateUser)
		r.Get("/users", adminUsersHandler.ListUsers)
		r.Put("/users/{user_id}/toggle-active", adminUsersHandler.ToggleUserActive)

		r.Post("/songs", adminSongsHandler.UploadSong)
		r.Get("/songs", adminSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", adminSongsHandler.GetSong)
		r.Post("/songs/{song_id}/new-version", adminSongsHandler.CreateNewVersion)
		r.Put("/songs/{song_id}/set-active", adminSongsHandler.SetActiveVersion)
		r.Get("/songs/{song_id}/versions", adminSongsHandler.ListVersions)
		r.Patch("/songs/{song_id}/tracks/{track_id}", adminSongsHandler.UpdateTrack)
		r.Delete("/songs/{song_id}", adminSongsHandler.DeleteSong)

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

	// === Auth API ===
	r.Post("/api/v1/auth/login", authHandler.Login)

	// === User API ===
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authHandler.Middleware)

		r.Get("/songs", userSongsHandler.ListSongs)
		r.Get("/songs/{song_id}", userSongsHandler.GetSong)
		r.Get("/songs/{song_id}/midi", userSongsHandler.DownloadMIDI)
		r.Get("/songs/{song_id}/structures", userSongsHandler.GetStructures)
		r.Get("/songs/{song_id}/tracks/{track_id}/audio", trackAudioHandler.ServeTrackAudio)

		r.Post("/assessments/submit", userAssessmentsHandler.Submit)
		r.Post("/assessments/analyze", userAssessmentsHandler.AnalyzeRecording)
		r.Post("/assessments/analyze/basic", userAssessmentsHandler.AnalyzeRecordingWithoutFiltering)
		r.Get("/assessments", userAssessmentsHandler.ListMyAssessments)
		r.Get("/assessments/stats", userAssessmentsHandler.GetStats)
		r.Get("/assessments/{assessment_id}", userAssessmentsHandler.GetAssessment)
		r.Get("/assessments/{assessment_id}/download", userAssessmentsHandler.Download)
		r.Delete("/assessments/{assessment_id}", userAssessmentsHandler.Delete)
	})

	// Static file server for frontend
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, ".")
	r.Handle("/*", http.FileServer(http.Dir(filesDir)))

	fmt.Printf("Upload directory: %s\n", absUploadDir)

	// Start server with HTTPS if TLS cert/key are provided, otherwise HTTP
	if tlsCert != "" && tlsKey != "" {
		fmt.Printf("Vocal Practice App server starting on %s (HTTPS)\n", httpsPort)
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
		fmt.Printf("Vocal Practice App server starting on %s (HTTP)\n", port)
		log.Fatal(http.ListenAndServe(port, r))
	}
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
