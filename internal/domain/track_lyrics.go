package domain

import (
	"time"

	"github.com/google/uuid"
)

type TrackLyrics struct {
	ID          uuid.UUID `json:"id"`
	TrackID     uuid.UUID `json:"track_id"`
	StructureID uuid.UUID `json:"structure_id"`
	Lyrics      string    `json:"lyrics"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewTrackLyrics(trackID, structureID uuid.UUID, lyrics string) *TrackLyrics {
	now := time.Now()
	return &TrackLyrics{
		ID:          uuid.New(),
		TrackID:     trackID,
		StructureID: structureID,
		Lyrics:      lyrics,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
