package storage

import (
	"errors"
	"sync"
	"time"

	"vocal-practice-app/internal/domain"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("record not found")
	ErrEmailExists    = errors.New("email already exists")
	ErrUsernameExists = errors.New("username already exists")
)

type Store struct {
	mu           sync.RWMutex
	users        map[uuid.UUID]*domain.User
	usersByEmail map[string]*domain.User
	songs        map[uuid.UUID]*domain.Song
	structures   map[uuid.UUID]*domain.SongStructure
	assessments  map[uuid.UUID]*domain.UserAssessment
	trackLyrics  map[string]*domain.TrackLyrics // key: "trackID:structureID"
}

func New() *Store {
	return &Store{
		users:        make(map[uuid.UUID]*domain.User),
		usersByEmail: make(map[string]*domain.User),
		songs:        make(map[uuid.UUID]*domain.Song),
		structures:   make(map[uuid.UUID]*domain.SongStructure),
		assessments:  make(map[uuid.UUID]*domain.UserAssessment),
		trackLyrics:  make(map[string]*domain.TrackLyrics),
	}
}

// --- User ---

func (s *Store) CreateUser(user *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usersByEmail[user.Email]; exists {
		return ErrEmailExists
	}
	s.users[user.ID] = user
	s.usersByEmail[user.Email] = user
	return nil
}

func (s *Store) GetUserByID(id uuid.UUID) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *Store) GetUserByEmail(email string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *Store) UpdateUser(user *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[user.ID]; !ok {
		return ErrNotFound
	}
	s.users[user.ID] = user
	return nil
}

func (s *Store) ListUsers() ([]*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.User, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, u)
	}
	return result, nil
}

// --- Song ---

func (s *Store) CreateSong(song *domain.Song) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.songs[song.ID] = song
	return nil
}

func (s *Store) GetSongByID(id uuid.UUID) (*domain.Song, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	song, ok := s.songs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return song, nil
}

func (s *Store) ListSongs() ([]*domain.Song, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.Song, 0, len(s.songs))
	for _, song := range s.songs {
		result = append(result, song)
	}
	return result, nil
}

func (s *Store) DeleteSong(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.songs[id]; !ok {
		return ErrNotFound
	}
	delete(s.songs, id)
	return nil
}

// --- Track ---

func (s *Store) UpdateTrack(songID, trackID uuid.UUID, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	song, ok := s.songs[songID]
	if !ok {
		return ErrNotFound
	}

	for i, t := range song.Tracks {
		if t.ID == trackID {
			if v, ok := updates["is_vocal"]; ok {
				song.Tracks[i].IsVocal = v.(bool)
			}
			return nil
		}
	}
	return ErrNotFound
}

// --- Structure ---

func (s *Store) CreateStructure(structures []*domain.SongStructure) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, st := range structures {
		s.structures[st.ID] = st
	}
	return nil
}

func (s *Store) GetStructureByID(id uuid.UUID) (*domain.SongStructure, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, ok := s.structures[id]
	if !ok {
		return nil, ErrNotFound
	}
	return st, nil
}

func (s *Store) UpdateStructure(st *domain.SongStructure) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.structures[st.ID]; !ok {
		return ErrNotFound
	}
	s.structures[st.ID] = st
	return nil
}

func (s *Store) DeleteStructure(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// cascade delete: find all phrases with this parent ID
	for sid, st := range s.structures {
		if st.ParentID != nil && *st.ParentID == id {
			delete(s.structures, sid)
		}
	}

	if _, ok := s.structures[id]; !ok {
		return ErrNotFound
	}
	delete(s.structures, id)
	return nil
}

func (s *Store) ListStructuresBySong(songID uuid.UUID) ([]*domain.SongStructure, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.SongStructure, 0)
	for _, st := range s.structures {
		if st.SongID == songID {
			result = append(result, st)
		}
	}
	return result, nil
}

func (s *Store) BuildStructureTree(songID uuid.UUID) ([]*domain.StructureNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// group by parent
	children := make(map[uuid.UUID][]*domain.SongStructure)
	var roots []*domain.SongStructure

	for _, st := range s.structures {
		if st.SongID != songID {
			continue
		}
		if st.ParentID == nil {
			roots = append(roots, st)
		} else {
			children[*st.ParentID] = append(children[*st.ParentID], st)
		}
	}

	var build func(parent uuid.UUID) []domain.StructureNode
	build = func(parent uuid.UUID) []domain.StructureNode {
		var nodes []domain.StructureNode
		for _, st := range children[parent] {
			node := domain.StructureNode{
				SongStructure: *st,
				Phrases:       build(st.ID),
			}
			nodes = append(nodes, node)
		}
		return nodes
	}

	result := make([]*domain.StructureNode, 0, len(roots))
	for _, root := range roots {
		node := &domain.StructureNode{
			SongStructure: *root,
			Phrases:       build(root.ID),
		}
		result = append(result, node)
	}
	return result, nil
}

// --- Assessment ---

func (s *Store) CreateAssessment(a *domain.UserAssessment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.assessments[a.ID] = a
	return nil
}

func (s *Store) GetAssessmentByID(id uuid.UUID) (*domain.UserAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.assessments[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (s *Store) ListAssessmentsByUser(userID uuid.UUID, songID *uuid.UUID, limit, offset int) ([]*domain.UserAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.UserAssessment, 0)
	for _, a := range s.assessments {
		if a.UserID != userID {
			continue
		}
		if songID != nil && a.SongID != *songID {
			continue
		}
		result = append(result, a)
	}

	// apply offset/limit (sorted by created_at desc)
	if offset >= len(result) {
		return []*domain.UserAssessment{}, nil
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) DeleteAssessment(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.assessments[id]; !ok {
		return ErrNotFound
	}
	delete(s.assessments, id)
	return nil
}

func (s *Store) GetAssessmentStatsByUser(userID uuid.UUID) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalPractices int
	var totalScore float64
	songSet := make(map[uuid.UUID]bool)

	for _, a := range s.assessments {
		if a.UserID != userID {
			continue
		}
		totalPractices++
		totalScore += a.Score
		songSet[a.SongID] = true
	}

	avgScore := float64(0)
	if totalPractices > 0 {
		avgScore = totalScore / float64(totalPractices)
	}

	return map[string]interface{}{
		"total_practices": totalPractices,
		"average_score":   avgScore,
		"total_songs":     len(songSet),
	}, nil
}

func (s *Store) ListAllAssessments(userID, songID *uuid.UUID) ([]*domain.UserAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.UserAssessment, 0)
	for _, a := range s.assessments {
		if userID != nil && a.UserID != *userID {
			continue
		}
		if songID != nil && a.SongID != *songID {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

// --- TrackLyrics ---

func (s *Store) UpsertTrackLyrics(trackID, structureID uuid.UUID, lyrics string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := trackID.String() + ":" + structureID.String()
	if existing, ok := s.trackLyrics[key]; ok {
		existing.Lyrics = lyrics
		existing.UpdatedAt = time.Now()
	} else {
		s.trackLyrics[key] = domain.NewTrackLyrics(trackID, structureID, lyrics)
	}
	return nil
}

func (s *Store) GetTrackLyrics(trackID, structureID uuid.UUID) (*domain.TrackLyrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := trackID.String() + ":" + structureID.String()
	lyrics, ok := s.trackLyrics[key]
	if !ok {
		return nil, ErrNotFound
	}
	return lyrics, nil
}

func (s *Store) ListTrackLyricsByTrack(trackID uuid.UUID) ([]*domain.TrackLyrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := trackID.String() + ":"
	result := make([]*domain.TrackLyrics, 0)
	for key, l := range s.trackLyrics {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, l)
		}
	}
	return result, nil
}

func (s *Store) ListTrackLyricsBySong(songID uuid.UUID) ([]*domain.TrackLyrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all structure IDs for this song
	songStructIDs := make(map[uuid.UUID]bool)
	for _, st := range s.structures {
		if st.SongID == songID {
			songStructIDs[st.ID] = true
		}
	}

	result := make([]*domain.TrackLyrics, 0)
	for _, l := range s.trackLyrics {
		if songStructIDs[l.StructureID] {
			result = append(result, l)
		}
	}
	return result, nil
}

func (s *Store) DeleteTrackLyrics(trackID, structureID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := trackID.String() + ":" + structureID.String()
	delete(s.trackLyrics, key)
	return nil
}
