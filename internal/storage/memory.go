package storage

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sort"
	"sync"
	"time"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/seed"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("record not found")
	ErrEmailExists    = errors.New("email already exists")
	ErrUsernameExists = errors.New("username already exists")
)

// MemoryStore is an in-memory implementation of Store interface.
type MemoryStore struct {
	mu           sync.RWMutex
	users        map[uuid.UUID]*domain.User
	usersByEmail map[string]*domain.User
	songs        map[uuid.UUID]*domain.Song
	structures   map[uuid.UUID]*domain.SongStructure
	assessments  map[uuid.UUID]*domain.UserAssessment
	trackLyrics  map[string]*domain.TrackLyrics
	// writeCallback is invoked after every successful mutating write so that
	// external components (e.g. the backup manager) can react to changes.
	writeCallback func()
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		users:        make(map[uuid.UUID]*domain.User),
		usersByEmail: make(map[string]*domain.User),
		songs:        make(map[uuid.UUID]*domain.Song),
		structures:   make(map[uuid.UUID]*domain.SongStructure),
		assessments:  make(map[uuid.UUID]*domain.UserAssessment),
		trackLyrics:  make(map[string]*domain.TrackLyrics),
	}
	s.seedUsers()
	return s
}

// SetWriteCallback registers a function invoked (without holding the store
// lock) after every successful mutating write to the store. It is intended
// for components that need to react to changes — typically to persist the
// in-memory state to local disk. Passing nil clears any previously registered
// callback.
func (s *MemoryStore) SetWriteCallback(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeCallback = fn
}

// notifyWrite fires the registered write callback, if any. It must be called
// while the caller already holds the store write lock (s.mu), which provides
// the synchronization necessary to read writeCallback safely under the race
// detector. The callback itself must not acquire the store lock.
func (s *MemoryStore) notifyWrite() {
	fn := s.writeCallback
	if fn != nil {
		fn()
	}
}

// seedUsers populates the store with default demo users. The authoritative
// seed data lives in the local storage file data/seed/users.json; if that file
// cannot be loaded, a minimal set of built-in dev users is used as a fallback.
func (s *MemoryStore) seedUsers() {
	recs, err := seed.LoadUsers(seed.UsersFile())
	if err == nil && len(recs) > 0 {
		for _, r := range recs {
			id, perr := uuid.Parse(r.ID)
			if perr != nil {
				continue
			}
			u := &domain.User{
				ID:           id,
				Username:     r.Username,
				Email:        r.Email,
				PasswordHash: r.PasswordHash,
				Role:         r.Role,
				IsActive:     r.IsActive,
				CreatedAt:    time.Now(),
			}
			s.users[u.ID] = u
			s.usersByEmail[u.Email] = u
		}
		return
	}
	seedDefaultUsers(s)
}

// seedDefaultUsers is the built-in fallback used only when the seed data file
// cannot be loaded (e.g. running the binary outside the repository).
func seedDefaultUsers(s *MemoryStore) {
	admin := &domain.User{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Username:     "admin",
		Email:        "admin@localhost",
		PasswordHash: hashPwd("admin1234"),
		Role:         "admin",
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
	s.users[admin.ID] = admin
	s.usersByEmail[admin.Email] = admin

	demo := &domain.User{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Username:     "testuser",
		Email:        "test@localhost",
		PasswordHash: hashPwd("test1234"),
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
	s.users[demo.ID] = demo
	s.usersByEmail[demo.Email] = demo
}

func hashPwd(password string) string {
	h := sha256.Sum256([]byte(password))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// --- User ---

func (s *MemoryStore) CreateUser(user *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.usersByEmail[user.Email]; exists {
		return ErrEmailExists
	}
	s.users[user.ID] = user
	s.usersByEmail[user.Email] = user
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) GetUserByID(id uuid.UUID) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) GetUserByEmail(email string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) UpdateUser(user *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[user.ID]; !ok {
		return ErrNotFound
	}
	s.users[user.ID] = user
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) ListUsers() ([]*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.User, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, u)
	}
	return result, nil
}

// --- Song ---

func (s *MemoryStore) CreateSong(song *domain.Song) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.songs[song.ID] = song
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) GetSongByID(id uuid.UUID) (*domain.Song, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	song, ok := s.songs[id]
	if !ok {
		return nil, ErrNotFound
	}
	// Deduplicate track names for songs stored before the dedup fix
	domain.DeduplicateTrackNames(song.Tracks)
	return song, nil
}

func (s *MemoryStore) ListSongs() ([]*domain.Song, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Song, 0, len(s.songs))
	for _, song := range s.songs {
		result = append(result, song)
	}
	return result, nil
}

func (s *MemoryStore) ListActiveSongs() ([]*domain.Song, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Song, 0)
	for _, song := range s.songs {
		if song.IsActive {
			result = append(result, song)
		}
	}
	return result, nil
}

func (s *MemoryStore) ListVersionsByGroup(groupID uuid.UUID) ([]*domain.Song, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Song, 0)
	for _, song := range s.songs {
		if song.VersionGroup == groupID {
			result = append(result, song)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *MemoryStore) SetActiveVersion(groupID, songID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, song := range s.songs {
		if song.VersionGroup == groupID {
			song.IsActive = false
		}
	}
	target, ok := s.songs[songID]
	if !ok {
		return ErrNotFound
	}
	if target.VersionGroup != groupID {
		return ErrNotFound
	}
	target.IsActive = true
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) DeleteSong(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.songs[id]; !ok {
		return ErrNotFound
	}
	delete(s.songs, id)
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) UpdateTrack(songID, trackID uuid.UUID, updates map[string]interface{}) error {
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
			s.notifyWrite()
			return nil
		}
	}
	return ErrNotFound
}

// --- Structure ---

func (s *MemoryStore) CreateStructure(structures []*domain.SongStructure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range structures {
		s.structures[st.ID] = st
	}
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) GetStructureByID(id uuid.UUID) (*domain.SongStructure, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.structures[id]
	if !ok {
		return nil, ErrNotFound
	}
	return st, nil
}

func (s *MemoryStore) UpdateStructure(st *domain.SongStructure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.structures[st.ID]; !ok {
		return ErrNotFound
	}
	s.structures[st.ID] = st
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) DeleteStructure(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sid, st := range s.structures {
		if st.ParentID != nil && *st.ParentID == id {
			delete(s.structures, sid)
		}
	}
	if _, ok := s.structures[id]; !ok {
		return ErrNotFound
	}
	delete(s.structures, id)
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) DeleteAllStructuresBySong(songID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sid, st := range s.structures {
		if st.SongID == songID {
			delete(s.structures, sid)
		}
	}
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) ListStructuresBySong(songID uuid.UUID) ([]*domain.SongStructure, error) {
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

func (s *MemoryStore) ListStructuresBySongAndTrack(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.SongStructure, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.SongStructure, 0)
	for _, st := range s.structures {
		if st.SongID != songID {
			continue
		}
		if trackID == nil {
			if st.TrackID == nil {
				result = append(result, st)
			}
		} else {
			if st.TrackID == nil || *st.TrackID == *trackID {
				result = append(result, st)
			}
		}
	}
	return result, nil
}

func (s *MemoryStore) BuildStructureTree(songID uuid.UUID) ([]*domain.StructureNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	sortStructures := func(items []*domain.SongStructure) {
		sort.Slice(items, func(i, j int) bool {
			a := items[i]
			b := items[j]
			if a.StartTick != b.StartTick {
				return a.StartTick < b.StartTick
			}
			if a.StartTime != b.StartTime {
				return a.StartTime < b.StartTime
			}
			if a.OrderIdx != b.OrderIdx {
				return a.OrderIdx < b.OrderIdx
			}
			return a.ID.String() < b.ID.String()
		})
	}
	sortStructures(roots)
	for parentID := range children {
		sortStructures(children[parentID])
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

func (s *MemoryStore) BuildStructureTreeForTrack(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.StructureNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	children := make(map[uuid.UUID][]*domain.SongStructure)
	var roots []*domain.SongStructure
	for _, st := range s.structures {
		if st.SongID != songID {
			continue
		}
		if trackID != nil {
			if st.TrackID != nil && *st.TrackID != *trackID {
				continue
			}
		} else {
			if st.TrackID != nil {
				continue
			}
		}
		if st.ParentID == nil {
			roots = append(roots, st)
		} else {
			children[*st.ParentID] = append(children[*st.ParentID], st)
		}
	}
	sortStructures := func(items []*domain.SongStructure) {
		sort.Slice(items, func(i, j int) bool {
			a := items[i]
			b := items[j]
			if a.StartTick != b.StartTick {
				return a.StartTick < b.StartTick
			}
			if a.StartTime != b.StartTime {
				return a.StartTime < b.StartTime
			}
			if a.OrderIdx != b.OrderIdx {
				return a.OrderIdx < b.OrderIdx
			}
			return a.ID.String() < b.ID.String()
		})
	}
	sortStructures(roots)
	for parentID := range children {
		sortStructures(children[parentID])
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

func (s *MemoryStore) CreateAssessment(a *domain.UserAssessment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assessments[a.ID] = a
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) GetAssessmentByID(id uuid.UUID) (*domain.UserAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assessments[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (s *MemoryStore) ListAssessmentsByUser(userID uuid.UUID, songID *uuid.UUID, limit, offset int) ([]*domain.UserAssessment, error) {
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
	if offset >= len(result) {
		return []*domain.UserAssessment{}, nil
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (s *MemoryStore) DeleteAssessment(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.assessments[id]; !ok {
		return ErrNotFound
	}
	delete(s.assessments, id)
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) GetAssessmentStatsByUser(userID uuid.UUID) (map[string]interface{}, error) {
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

func (s *MemoryStore) ListAllAssessments(userID, songID *uuid.UUID) ([]*domain.UserAssessment, error) {
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

func (s *MemoryStore) UpsertTrackLyrics(trackID, structureID uuid.UUID, lyrics string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := trackID.String() + ":" + structureID.String()
	if existing, ok := s.trackLyrics[key]; ok {
		existing.Lyrics = lyrics
		existing.UpdatedAt = time.Now()
	} else {
		s.trackLyrics[key] = domain.NewTrackLyrics(trackID, structureID, lyrics)
	}
	s.notifyWrite()
	return nil
}

func (s *MemoryStore) GetTrackLyrics(trackID, structureID uuid.UUID) (*domain.TrackLyrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := trackID.String() + ":" + structureID.String()
	lyrics, ok := s.trackLyrics[key]
	if !ok {
		return nil, ErrNotFound
	}
	return lyrics, nil
}

func (s *MemoryStore) ListTrackLyricsByTrack(trackID uuid.UUID) ([]*domain.TrackLyrics, error) {
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

func (s *MemoryStore) ListTrackLyricsBySong(songID uuid.UUID) ([]*domain.TrackLyrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

func (s *MemoryStore) DeleteTrackLyrics(trackID, structureID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := trackID.String() + ":" + structureID.String()
	delete(s.trackLyrics, key)
	s.notifyWrite()
	return nil
}
