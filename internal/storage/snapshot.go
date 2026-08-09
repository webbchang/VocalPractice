package storage

import (
	"encoding/json"
	"time"

	"vocal-practice-app/internal/domain"

	"github.com/google/uuid"
)

// userSnapshot is the serializable representation of a user. It mirrors
// domain.User but, unlike it, includes the password hash so that in-memory
// state can be fully restored from a backup file (the domain type marks the
// hash json:"-" to avoid leaking it in API responses).
type userSnapshot struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

// storeSnapshot is the serializable representation of every collection held
// by MemoryStore. It is used to persist in-memory state to local disk and to
// restore it back later.
type storeSnapshot struct {
	Users       []userSnapshot           `json:"users"`
	Songs       []*domain.Song           `json:"songs"`
	Structures  []*domain.SongStructure  `json:"structures"`
	Assessments []*domain.UserAssessment `json:"assessments"`
	TrackLyrics []*domain.TrackLyrics    `json:"track_lyrics"`
}

func toUserSnapshot(u *domain.User) userSnapshot {
	return userSnapshot{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt,
	}
}

func (u userSnapshot) toDomain() *domain.User {
	return &domain.User{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt,
	}
}

// Snapshot returns a JSON-encoded copy of the entire in-memory store state.
// It is safe for concurrent use.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]userSnapshot, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, toUserSnapshot(u))
	}
	songs := make([]*domain.Song, 0, len(s.songs))
	for _, song := range s.songs {
		songs = append(songs, song)
	}
	structures := make([]*domain.SongStructure, 0, len(s.structures))
	for _, st := range s.structures {
		structures = append(structures, st)
	}
	assessments := make([]*domain.UserAssessment, 0, len(s.assessments))
	for _, a := range s.assessments {
		assessments = append(assessments, a)
	}
	trackLyrics := make([]*domain.TrackLyrics, 0, len(s.trackLyrics))
	for _, l := range s.trackLyrics {
		trackLyrics = append(trackLyrics, l)
	}

	snap := storeSnapshot{
		Users:       users,
		Songs:       songs,
		Structures:  structures,
		Assessments: assessments,
		TrackLyrics: trackLyrics,
	}
	return json.Marshal(snap)
}

// RestoreFromJSON replaces the entire in-memory store state with the data
// encoded in the provided JSON document (as produced by Snapshot). The
// secondary users-by-email index is rebuilt during restoration. It is safe
// for concurrent use.
func (s *MemoryStore) RestoreFromJSON(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.users = make(map[uuid.UUID]*domain.User, len(snap.Users))
	s.usersByEmail = make(map[string]*domain.User, len(snap.Users))
	for _, u := range snap.Users {
		d := u.toDomain()
		s.users[d.ID] = d
		s.usersByEmail[d.Email] = d
	}

	s.songs = make(map[uuid.UUID]*domain.Song, len(snap.Songs))
	for _, song := range snap.Songs {
		s.songs[song.ID] = song
	}

	s.structures = make(map[uuid.UUID]*domain.SongStructure, len(snap.Structures))
	for _, st := range snap.Structures {
		s.structures[st.ID] = st
	}

	s.assessments = make(map[uuid.UUID]*domain.UserAssessment, len(snap.Assessments))
	for _, a := range snap.Assessments {
		s.assessments[a.ID] = a
	}

	s.trackLyrics = make(map[string]*domain.TrackLyrics, len(snap.TrackLyrics))
	for _, l := range snap.TrackLyrics {
		key := l.TrackID.String() + ":" + l.StructureID.String()
		s.trackLyrics[key] = l
	}

	return nil
}
