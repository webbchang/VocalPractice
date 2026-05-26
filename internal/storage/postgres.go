package storage

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"vocal-practice-app/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgresStore and runs migrations.
func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}
	s := &PostgresStore{pool: pool}
	if err := s.migrate(); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func pgErrUniqueViolation(err error) bool {
	return err != nil && (isPgCode(err, "23505") || isPgCode(err, "23514"))
}

func isPgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

// Close releases the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) migrate() error {
	_, err := s.pool.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		username TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS songs (
		id UUID PRIMARY KEY,
		title TEXT NOT NULL,
		artist TEXT NOT NULL,
		midi_file_path TEXT NOT NULL DEFAULT '',
		tracks JSONB NOT NULL DEFAULT '[]',
		tempo_map JSONB NOT NULL DEFAULT '[]',
		ticks_per_quarter INT NOT NULL DEFAULT 0,
		version_group UUID NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true,
		version_label TEXT NOT NULL DEFAULT 'v1',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS song_structures (
		id UUID PRIMARY KEY,
		song_id UUID NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
		track_id UUID,
		parent_id UUID REFERENCES song_structures(id) ON DELETE CASCADE,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		start_time DOUBLE PRECISION NOT NULL DEFAULT 0,
		end_time DOUBLE PRECISION NOT NULL DEFAULT 0,
		start_tick INT NOT NULL DEFAULT 0,
		end_tick INT NOT NULL DEFAULT 0,
		order_idx INT NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_song_structures_song_id ON song_structures(song_id);
	CREATE INDEX IF NOT EXISTS idx_song_structures_parent_id ON song_structures(parent_id);
	CREATE TABLE IF NOT EXISTS user_assessments (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		song_id UUID NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
		track_id UUID NOT NULL,
		structure_id UUID,
		score DOUBLE PRECISION NOT NULL DEFAULT 0,
		total_notes INT NOT NULL DEFAULT 0,
		matched_notes INT NOT NULL DEFAULT 0,
		average_pitch_deviation DOUBLE PRECISION NOT NULL DEFAULT 0,
		average_duration_deviation DOUBLE PRECISION NOT NULL DEFAULT 0,
		pitch_deviation JSONB NOT NULL DEFAULT '[]',
		duration_deviation JSONB NOT NULL DEFAULT '[]',
		note_comparison JSONB NOT NULL DEFAULT '[]',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_assessments_user_id ON user_assessments(user_id);
	CREATE INDEX IF NOT EXISTS idx_assessments_song_id ON user_assessments(song_id);
	CREATE TABLE IF NOT EXISTS track_lyrics (
		track_id UUID NOT NULL,
		structure_id UUID NOT NULL,
		lyrics TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (track_id, structure_id)
	);
	`)
	return err
}

// ——— User ———

func (s *PostgresStore) CreateUser(user *domain.User) error {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, is_active, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.IsActive, user.CreatedAt,
	)
	if err != nil && pgErrUniqueViolation(err) {
		return ErrEmailExists
	}
	return err
}

func (s *PostgresStore) GetUserByID(id uuid.UUID) (*domain.User, error) {
	return s.scanUser(s.pool.QueryRow(context.Background(),
		`SELECT id, username, email, password_hash, role, is_active, created_at FROM users WHERE id=$1`, id))
}

func (s *PostgresStore) GetUserByEmail(email string) (*domain.User, error) {
	return s.scanUser(s.pool.QueryRow(context.Background(),
		`SELECT id, username, email, password_hash, role, is_active, created_at FROM users WHERE email=$1`, email))
}

func (s *PostgresStore) UpdateUser(user *domain.User) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE users SET username=$1, email=$2, password_hash=$3, role=$4, is_active=$5 WHERE id=$6`,
		user.Username, user.Email, user.PasswordHash, user.Role, user.IsActive, user.ID)
	return err
}

func (s *PostgresStore) ListUsers() ([]*domain.User, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, username, email, password_hash, role, is_active, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func scanUser(row pgx.Row) (*domain.User, error) {
	u := &domain.User{}
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *PostgresStore) scanUser(row pgx.Row) (*domain.User, error) { return scanUser(row) }

// ——— Song ———

func (s *PostgresStore) CreateSong(song *domain.Song) error {
	tracksJSON, _ := json.Marshal(song.Tracks)
	tempoJSON, _ := json.Marshal(song.TempoMap)
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO songs (id, title, artist, midi_file_path, tracks, tempo_map, ticks_per_quarter, version_group, is_active, version_label, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		song.ID, song.Title, song.Artist, song.MIDIFilePath, tracksJSON, tempoJSON,
		song.TicksPerQuarter, song.VersionGroup, song.IsActive, song.VersionLabel, song.CreatedAt)
	return err
}

func scanSong(row pgx.Row) (*domain.Song, error) {
	s := &domain.Song{}
	var tracksJSON, tempoJSON []byte
	err := row.Scan(&s.ID, &s.Title, &s.Artist, &s.MIDIFilePath, &tracksJSON, &tempoJSON,
		&s.TicksPerQuarter, &s.VersionGroup, &s.IsActive, &s.VersionLabel, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(tracksJSON, &s.Tracks)
	json.Unmarshal(tempoJSON, &s.TempoMap)
	return s, nil
}

func (s *PostgresStore) GetSongByID(id uuid.UUID) (*domain.Song, error) {
	return scanSong(s.pool.QueryRow(context.Background(),
		`SELECT id, title, artist, midi_file_path, tracks, tempo_map, ticks_per_quarter, version_group, is_active, version_label, created_at FROM songs WHERE id=$1`, id))
}

func (s *PostgresStore) ListSongs() ([]*domain.Song, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, title, artist, midi_file_path, tracks, tempo_map, ticks_per_quarter, version_group, is_active, version_label, created_at FROM songs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *PostgresStore) ListActiveSongs() ([]*domain.Song, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, title, artist, midi_file_path, tracks, tempo_map, ticks_per_quarter, version_group, is_active, version_label, created_at FROM songs WHERE is_active=true ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *PostgresStore) ListVersionsByGroup(groupID uuid.UUID) ([]*domain.Song, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, title, artist, midi_file_path, tracks, tempo_map, ticks_per_quarter, version_group, is_active, version_label, created_at FROM songs WHERE version_group=$1 ORDER BY created_at DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func collectSongs(rows pgx.Rows) ([]*domain.Song, error) {
	var songs []*domain.Song
	for rows.Next() {
		s, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		songs = append(songs, s)
	}
	return songs, nil
}

func (s *PostgresStore) SetActiveVersion(groupID, songID uuid.UUID) error {
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	// Deactivate all in group
	if _, err := tx.Exec(context.Background(), `UPDATE songs SET is_active=false WHERE version_group=$1`, groupID); err != nil {
		return err
	}
	// Activate target
	tag, err := tx.Exec(context.Background(), `UPDATE songs SET is_active=true WHERE id=$1 AND version_group=$2`, songID, groupID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(context.Background())
}

func (s *PostgresStore) DeleteSong(id uuid.UUID) error {
	tag, err := s.pool.Exec(context.Background(), `DELETE FROM songs WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) UpdateTrack(songID, trackID uuid.UUID, updates map[string]interface{}) error {
	// Read current tracks JSON
	var tracksJSON []byte
	err := s.pool.QueryRow(context.Background(), `SELECT tracks FROM songs WHERE id=$1`, songID).Scan(&tracksJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var tracks []domain.MIDITrack
	json.Unmarshal(tracksJSON, &tracks)
	found := false
	for i := range tracks {
		if tracks[i].ID == trackID {
			if v, ok := updates["is_vocal"]; ok {
				tracks[i].IsVocal = v.(bool)
			}
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	newJSON, _ := json.Marshal(tracks)
	_, err = s.pool.Exec(context.Background(), `UPDATE songs SET tracks=$1 WHERE id=$2`, newJSON, songID)
	return err
}

// ——— Structure ———

func (s *PostgresStore) CreateStructure(structures []*domain.SongStructure) error {
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	for _, st := range structures {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO song_structures (id, song_id, track_id, parent_id, type, title, start_time, end_time, start_tick, end_tick, order_idx)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			st.ID, st.SongID, st.TrackID, st.ParentID, st.Type, st.Title,
			st.StartTime, st.EndTime, st.StartTick, st.EndTick, st.OrderIdx)
		if err != nil {
			return err
		}
	}
	return tx.Commit(context.Background())
}

func scanStructure(row pgx.Row) (*domain.SongStructure, error) {
	st := &domain.SongStructure{}
	err := row.Scan(&st.ID, &st.SongID, &st.TrackID, &st.ParentID, &st.Type, &st.Title,
		&st.StartTime, &st.EndTime, &st.StartTick, &st.EndTick, &st.OrderIdx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return st, err
}

func (s *PostgresStore) GetStructureByID(id uuid.UUID) (*domain.SongStructure, error) {
	return scanStructure(s.pool.QueryRow(context.Background(),
		`SELECT id, song_id, track_id, parent_id, type, title, start_time, end_time, start_tick, end_tick, order_idx FROM song_structures WHERE id=$1`, id))
}

func (s *PostgresStore) UpdateStructure(st *domain.SongStructure) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE song_structures SET track_id=$1, parent_id=$2, type=$3, title=$4, start_time=$5, end_time=$6, start_tick=$7, end_tick=$8, order_idx=$9 WHERE id=$10`,
		st.TrackID, st.ParentID, st.Type, st.Title, st.StartTime, st.EndTime, st.StartTick, st.EndTick, st.OrderIdx, st.ID)
	return err
}

func (s *PostgresStore) DeleteStructure(id uuid.UUID) error {
	// CASCADE is handled by FK constraint but we need the not-found check
	tag, err := s.pool.Exec(context.Background(), `DELETE FROM song_structures WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteAllStructuresBySong(songID uuid.UUID) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM song_structures WHERE song_id=$1`, songID)
	return err
}

func (s *PostgresStore) ListStructuresBySong(songID uuid.UUID) ([]*domain.SongStructure, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, song_id, track_id, parent_id, type, title, start_time, end_time, start_tick, end_tick, order_idx FROM song_structures WHERE song_id=$1 ORDER BY order_idx, start_tick, start_time`, songID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectStructures(rows)
}

func (s *PostgresStore) ListStructuresBySongAndTrack(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.SongStructure, error) {
	if trackID == nil {
		return s.ListStructuresBySong(songID)
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, song_id, track_id, parent_id, type, title, start_time, end_time, start_tick, end_tick, order_idx FROM song_structures WHERE song_id=$1 AND (track_id IS NULL OR track_id=$2) ORDER BY order_idx, start_tick, start_time`,
		songID, *trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectStructures(rows)
}

func collectStructures(rows pgx.Rows) ([]*domain.SongStructure, error) {
	var sts []*domain.SongStructure
	for rows.Next() {
		st, err := scanStructure(rows)
		if err != nil {
			return nil, err
		}
		sts = append(sts, st)
	}
	return sts, nil
}

func (s *PostgresStore) BuildStructureTree(songID uuid.UUID) ([]*domain.StructureNode, error) {
	return s.buildTree(songID, nil)
}

func (s *PostgresStore) BuildStructureTreeForTrack(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.StructureNode, error) {
	return s.buildTree(songID, trackID)
}

func (s *PostgresStore) buildTree(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.StructureNode, error) {
	all, err := s.ListStructuresBySong(songID)
	if err != nil {
		return nil, err
	}
	// Filter by track
	if trackID != nil {
		filtered := make([]*domain.SongStructure, 0, len(all))
		for _, st := range all {
			if st.TrackID == nil || *st.TrackID == *trackID {
				filtered = append(filtered, st)
			}
		}
		all = filtered
	}
	// Build tree in memory (same logic as MemoryStore)
	children := make(map[uuid.UUID][]*domain.SongStructure)
	var roots []*domain.SongStructure
	for _, st := range all {
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
	for pid := range children {
		sortStructures(children[pid])
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

// ——— Assessment ———

func (s *PostgresStore) CreateAssessment(a *domain.UserAssessment) error {
	pitchJSON, _ := json.Marshal(a.PitchDeviation)
	durJSON, _ := json.Marshal(a.DurationDeviation)
	noteCompJSON, _ := json.Marshal(a.NoteComparison)
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO user_assessments (id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		a.ID, a.UserID, a.SongID, a.TrackID, a.StructureID, a.Score, a.TotalNotes, a.MatchedNotes,
		a.AveragePitchDeviation, a.AverageDurationDeviation, pitchJSON, durJSON, noteCompJSON, a.CreatedAt)
	return err
}

func scanAssessment(row pgx.Row) (*domain.UserAssessment, error) {
	a := &domain.UserAssessment{}
	var pitchJSON, durJSON, noteCompJSON []byte
	err := row.Scan(&a.ID, &a.UserID, &a.SongID, &a.TrackID, &a.StructureID, &a.Score, &a.TotalNotes, &a.MatchedNotes,
		&a.AveragePitchDeviation, &a.AverageDurationDeviation, &pitchJSON, &durJSON, &noteCompJSON, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(pitchJSON, &a.PitchDeviation)
	json.Unmarshal(durJSON, &a.DurationDeviation)
	json.Unmarshal(noteCompJSON, &a.NoteComparison)
	return a, nil
}

func (s *PostgresStore) GetAssessmentByID(id uuid.UUID) (*domain.UserAssessment, error) {
	return scanAssessment(s.pool.QueryRow(context.Background(),
		`SELECT id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at FROM user_assessments WHERE id=$1`, id))
}

func (s *PostgresStore) ListAssessmentsByUser(userID uuid.UUID, songID *uuid.UUID, limit, offset int) ([]*domain.UserAssessment, error) {
	if songID != nil {
		rows, err := s.pool.Query(context.Background(),
			`SELECT id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at FROM user_assessments WHERE user_id=$1 AND song_id=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
			userID, *songID, limit, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return collectAssessments(rows)
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at FROM user_assessments WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAssessments(rows)
}

func (s *PostgresStore) GetAssessmentStatsByUser(userID uuid.UUID) (map[string]interface{}, error) {
	var totalPractices int
	var totalScore float64
	var totalSongs int
	row := s.pool.QueryRow(context.Background(),
		`SELECT COUNT(*), COALESCE(SUM(score),0), COUNT(DISTINCT song_id) FROM user_assessments WHERE user_id=$1`, userID)
	row.Scan(&totalPractices, &totalScore, &totalSongs)
	avgScore := 0.0
	if totalPractices > 0 {
		avgScore = totalScore / float64(totalPractices)
	}
	return map[string]interface{}{
		"total_practices": totalPractices,
		"average_score":   avgScore,
		"total_songs":     totalSongs,
	}, nil
}

func (s *PostgresStore) ListAllAssessments(userID, songID *uuid.UUID) ([]*domain.UserAssessment, error) {
	if userID != nil && songID != nil {
		rows, err := s.pool.Query(context.Background(),
			`SELECT id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at FROM user_assessments WHERE user_id=$1 AND song_id=$2 ORDER BY created_at DESC`,
			*userID, *songID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return collectAssessments(rows)
	}
	if userID != nil {
		rows, err := s.pool.Query(context.Background(),
			`SELECT id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at FROM user_assessments WHERE user_id=$1 ORDER BY created_at DESC`,
			*userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return collectAssessments(rows)
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, user_id, song_id, track_id, structure_id, score, total_notes, matched_notes, average_pitch_deviation, average_duration_deviation, pitch_deviation, duration_deviation, note_comparison, created_at FROM user_assessments ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAssessments(rows)
}

func (s *PostgresStore) DeleteAssessment(id uuid.UUID) error {
	tag, err := s.pool.Exec(context.Background(), `DELETE FROM user_assessments WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func collectAssessments(rows pgx.Rows) ([]*domain.UserAssessment, error) {
	var as []*domain.UserAssessment
	for rows.Next() {
		a, err := scanAssessment(rows)
		if err != nil {
			return nil, err
		}
		as = append(as, a)
	}
	return as, nil
}

// ——— TrackLyrics ———

func (s *PostgresStore) UpsertTrackLyrics(trackID, structureID uuid.UUID, lyrics string) error {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO track_lyrics (track_id, structure_id, lyrics, updated_at) VALUES ($1,$2,$3,NOW())
		 ON CONFLICT (track_id, structure_id) DO UPDATE SET lyrics=$3, updated_at=NOW()`,
		trackID, structureID, lyrics)
	return err
}

func (s *PostgresStore) GetTrackLyrics(trackID, structureID uuid.UUID) (*domain.TrackLyrics, error) {
	l := &domain.TrackLyrics{}
	err := s.pool.QueryRow(context.Background(),
		`SELECT track_id, structure_id, lyrics, updated_at FROM track_lyrics WHERE track_id=$1 AND structure_id=$2`,
		trackID, structureID).Scan(&l.TrackID, &l.StructureID, &l.Lyrics, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return l, err
}

func (s *PostgresStore) ListTrackLyricsByTrack(trackID uuid.UUID) ([]*domain.TrackLyrics, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT track_id, structure_id, lyrics, updated_at FROM track_lyrics WHERE track_id=$1`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLyrics(rows)
}

func (s *PostgresStore) ListTrackLyricsBySong(songID uuid.UUID) ([]*domain.TrackLyrics, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT l.track_id, l.structure_id, l.lyrics, l.updated_at
		 FROM track_lyrics l
		 JOIN song_structures s ON s.id = l.structure_id
		 WHERE s.song_id = $1`, songID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLyrics(rows)
}

func (s *PostgresStore) DeleteTrackLyrics(trackID, structureID uuid.UUID) error {
	_, err := s.pool.Exec(context.Background(),
		`DELETE FROM track_lyrics WHERE track_id=$1 AND structure_id=$2`, trackID, structureID)
	return err
}

func collectLyrics(rows pgx.Rows) ([]*domain.TrackLyrics, error) {
	var lyrics []*domain.TrackLyrics
	for rows.Next() {
		l := &domain.TrackLyrics{}
		if err := rows.Scan(&l.TrackID, &l.StructureID, &l.Lyrics, &l.UpdatedAt); err != nil {
			return nil, err
		}
		lyrics = append(lyrics, l)
	}
	return lyrics, nil
}
