package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vocal-practice-app/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	store     storage.Store
	jwtSecret string
}

func NewAuthHandler(store storage.Store, jwtSecret string) *AuthHandler {
	return &AuthHandler{store: store, jwtSecret: jwtSecret}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string        `json:"token"`
	User  *userResponse `json:"user"`
}

type userResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	IsActive bool      `json:"is_active"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := h.store.GetUserByEmail(req.Email)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Verify password: try bcrypt first, fall back to SHA256
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// Fallback: SHA256 (for seeded dev users)
		expectedHash := hashPasswordSHA256(req.Password)
		if user.PasswordHash != expectedHash {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
	}

	token, err := h.generateToken(user.ID, user.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{
		Token: token,
		User: &userResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
			IsActive: user.IsActive,
		},
	})
}

type contextKey string

const UserIDKey contextKey = "user_id"
const UserRoleKey contextKey = "user_role"

func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		userID, err := h.validateToken(parts[1])
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Fetch full user to get role
		user, err := h.store.GetUserByID(userID)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "user not found")
			return
		}

		// Set user ID and role in request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, user.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that checks if the authenticated user has one of the required roles.
func (h *AuthHandler) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(UserRoleKey).(string)
			if !ok || role == "" {
				respondError(w, http.StatusForbidden, "access denied")
				return
			}
			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			respondError(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

func (h *AuthHandler) generateToken(userID uuid.UUID, email string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	exp := strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10)
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"sub":"` + userID.String() + `","email":"` + email + `","exp":` + exp + `}`),
	)

	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return header + "." + payload + "." + sig, nil
}

func (h *AuthHandler) validateToken(token string) (uuid.UUID, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return uuid.Nil, ErrInvalidToken
	}

	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if parts[2] != expectedSig {
		return uuid.Nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return uuid.Parse(claims.Sub)
}

// hashPassword creates a bcrypt hash. Used for new user registration.
func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		// Fallback to SHA256 if bcrypt fails
		return hashPasswordSHA256(password)
	}
	return string(hash)
}

// hashPasswordSHA256 creates a SHA256 hash (for backwards compatibility).
func hashPasswordSHA256(password string) string {
	h := sha256.Sum256([]byte(password))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
