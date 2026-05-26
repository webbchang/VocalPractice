package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// setupAuthTestRouter creates a test router with the auth middleware chain.
// If requireRole is set, it also applies RequireRole middleware.
func setupAuthTestRouter(h *AuthHandler, requireRole string) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/test", func(r chi.Router) {
		r.Use(h.Middleware)
		if requireRole != "" {
			r.Use(h.RequireRole(requireRole))
		}
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(UserRoleKey).(string)
			userID, _ := r.Context().Value(UserIDKey).(uuid.UUID)
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"user_id": userID.String(),
				"role":    role,
			})
		})
	})
	return r
}

// adminID is the seeded admin user UUID.
var adminID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// testUserID is the seeded test user UUID.
var testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

func TestMiddlewareRejectsMissingAuthHeader(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error message, got empty")
	}
}

func TestMiddlewareRejectsInvalidAuthFormat(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddlewareRejectsBadTokenSignature(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "")

	// Craft a token with wrong signature
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIwMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDEiLCJlbWFpbCI6ImFkbWluQGxvY2FsaG9zdCIsImV4cCI6OTk5OTk5OTk5OX0.badsignature")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddlewareRejectsNonexistentUser(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "")

	// Generate a token for a user that doesn't exist
	nonexistentID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	token, err := h.generateToken(nonexistentID, "ghost@localhost")
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddlewareAllowsValidUser(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "")

	token, err := h.generateToken(adminID, "admin@localhost")
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["role"] != "admin" {
		t.Fatalf("expected role=admin, got %v", resp["role"])
	}
	if resp["user_id"] != adminID.String() {
		t.Fatalf("expected user_id=%s, got %v", adminID.String(), resp["user_id"])
	}
}

func TestRequireRoleAcceptsAdmin(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "admin")

	token, err := h.generateToken(adminID, "admin@localhost")
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireRoleRejectsUser(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")
	r := setupAuthTestRouter(h, "admin")

	token, err := h.generateToken(testUserID, "test@localhost")
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error message, got empty")
	}
}

func TestRequireRoleAcceptsMultipleRoles(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")

	// Custom router with multiple allowed roles
	r := chi.NewRouter()
	r.Route("/api/v1/test", func(r chi.Router) {
		r.Use(h.Middleware)
		r.Use(h.RequireRole("admin", "manager"))
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	})

	token, err := h.generateToken(adminID, "admin@localhost")
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireRoleRejectsWhenNoRoleInContext(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")

	// Router that applies RequireRole without Middleware (no role set)
	r := chi.NewRouter()
	r.Route("/api/v1/test", func(r chi.Router) {
		r.Use(h.RequireRole("admin"))
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiddlewareAndRequireRoleFullChain(t *testing.T) {
	store := storage.NewMemoryStore()
	h := NewAuthHandler(store, "test-secret")

	testCases := []struct {
		name       string
		userID     uuid.UUID
		email      string
		expectCode int
	}{
		{"admin user gets 200", adminID, "admin@localhost", http.StatusOK},
		{"regular user gets 403", testUserID, "test@localhost", http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := setupAuthTestRouter(h, "admin")

			token, err := h.generateToken(tc.userID, tc.email)
			if err != nil {
				t.Fatalf("generateToken failed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/test/ping", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectCode {
				t.Fatalf("expected %d, got %d, body=%s", tc.expectCode, rec.Code, rec.Body.String())
			}
		})
	}
}
