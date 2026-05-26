package storage

import (
	"vocal-practice-app/internal/domain"

	"github.com/google/uuid"
)

// Store defines the interface for all data storage operations.
type Store interface {
	// User operations
	CreateUser(user *domain.User) error
	GetUserByID(id uuid.UUID) (*domain.User, error)
	GetUserByEmail(email string) (*domain.User, error)
	UpdateUser(user *domain.User) error
	ListUsers() ([]*domain.User, error)

	// Song operations
	CreateSong(song *domain.Song) error
	GetSongByID(id uuid.UUID) (*domain.Song, error)
	ListSongs() ([]*domain.Song, error)
	ListActiveSongs() ([]*domain.Song, error)
	ListVersionsByGroup(groupID uuid.UUID) ([]*domain.Song, error)
	SetActiveVersion(groupID, songID uuid.UUID) error
	DeleteSong(id uuid.UUID) error
	UpdateTrack(songID, trackID uuid.UUID, updates map[string]interface{}) error

	// Structure operations
	GetStructureByID(id uuid.UUID) (*domain.SongStructure, error)
	CreateStructure(structures []*domain.SongStructure) error
	UpdateStructure(st *domain.SongStructure) error
	DeleteStructure(id uuid.UUID) error
	DeleteAllStructuresBySong(songID uuid.UUID) error
	ListStructuresBySong(songID uuid.UUID) ([]*domain.SongStructure, error)
	ListStructuresBySongAndTrack(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.SongStructure, error)
	BuildStructureTree(songID uuid.UUID) ([]*domain.StructureNode, error)
	BuildStructureTreeForTrack(songID uuid.UUID, trackID *uuid.UUID) ([]*domain.StructureNode, error)

	// Assessment operations
	CreateAssessment(a *domain.UserAssessment) error
	GetAssessmentByID(id uuid.UUID) (*domain.UserAssessment, error)
	ListAssessmentsByUser(userID uuid.UUID, songID *uuid.UUID, limit, offset int) ([]*domain.UserAssessment, error)
	GetAssessmentStatsByUser(userID uuid.UUID) (map[string]interface{}, error)
	ListAllAssessments(userID, songID *uuid.UUID) ([]*domain.UserAssessment, error)
	DeleteAssessment(id uuid.UUID) error

	// TrackLyrics operations
	UpsertTrackLyrics(trackID, structureID uuid.UUID, lyrics string) error
	GetTrackLyrics(trackID, structureID uuid.UUID) (*domain.TrackLyrics, error)
	ListTrackLyricsByTrack(trackID uuid.UUID) ([]*domain.TrackLyrics, error)
	ListTrackLyricsBySong(songID uuid.UUID) ([]*domain.TrackLyrics, error)
	DeleteTrackLyrics(trackID, structureID uuid.UUID) error
}
